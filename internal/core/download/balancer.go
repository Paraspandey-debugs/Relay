package download

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

type Balancer struct {
	activeTasks map[int]*ActiveTask
	activeMu    sync.Mutex
}

func NewBalancer() *Balancer {
	return &Balancer{
		activeTasks: make(map[int]*ActiveTask),
	}
}

func (b *Balancer) RegisterWorker(id int, t *ActiveTask) {
	b.activeMu.Lock()
	b.activeTasks[id] = t
	b.activeMu.Unlock()
}

func (b *Balancer) UnregisterWorker(id int) {
	b.activeMu.Lock()
	delete(b.activeTasks, id)
	b.activeMu.Unlock()
}

func (b *Balancer) SnapshotTasks(queue *TaskQueue) []segmentState {
	b.activeMu.Lock()
	defer b.activeMu.Unlock()
	
	var segs []segmentState
	queue.mu.Lock()
	for _, t := range queue.tasks {
		segs = append(segs, segmentState{
			Start: t.Offset,
			End:   t.Offset + t.Length - 1,
			Next:  t.Offset,
			Done:  false,
		})
	}
	queue.mu.Unlock()
	
	for _, a := range b.activeTasks {
		current := a.CurrentOffset.Load()
		stop := a.StopAt.Load()
		if current < stop {
			segs = append(segs, segmentState{
				Start: a.Offset, 
				End:   stop - 1,
				Next:  current,
				Done:  false,
			})
		}
	}
	return segs
}

func (b *Balancer) Run(ctx context.Context, queue *TaskQueue) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for queue.IdleWorkers() > 0 {
				didWork := false
				if queue.Len() == 0 {
					if b.StealWork(queue) {
						didWork = true
					}
				}
				if !didWork && queue.Len() == 0 {
					if b.HedgeWork(queue) {
						didWork = true
					}
				}
				if !didWork {
					break
				}
			}
		}
	}
}

func (b *Balancer) StealWork(queue *TaskQueue) bool {
	b.activeMu.Lock()
	defer b.activeMu.Unlock()

	bestID := -1
	var maxRemaining int64 = 0
	var bestActive *ActiveTask

	for id, active := range b.activeTasks {
		remaining := active.RemainingBytes()
		if remaining > 1024*1024 && remaining > maxRemaining { // min steal 1MB
			maxRemaining = remaining
			bestID = id
			bestActive = active
		}
	}

	if bestID == -1 {
		return false
	}

	splitSize := (maxRemaining / 2) & ^int64(4096-1) // align to 4KB
	if splitSize == 0 {
		return false
	}

	current := bestActive.CurrentOffset.Load()
	newStopAt := current + splitSize
	bestActive.StopAt.Store(newStopAt)

	finalCurrent := bestActive.CurrentOffset.Load()
	stolenStart := newStopAt
	if finalCurrent > newStopAt {
		stolenStart = finalCurrent
	}

	originalEnd := current + maxRemaining
	if stolenStart >= originalEnd {
		return false
	}

	stolenTask := Task{
		Offset: stolenStart,
		Length: originalEnd - stolenStart,
	}

	queue.Push(stolenTask)
	log.Printf("[Balancer] Stole %d bytes from worker %d (new range: %d-%d)", stolenTask.Length, bestID, stolenTask.Offset, stolenTask.Offset+stolenTask.Length)
	return true
}

func (b *Balancer) HedgeWork(queue *TaskQueue) bool {
	b.activeMu.Lock()
	defer b.activeMu.Unlock()

	var bestActive *ActiveTask
	var maxRemaining int64

	for _, active := range b.activeTasks {
		if active.Hedged.Load() != 0 {
			continue
		}
		remaining := active.RemainingBytes()
		if remaining > 0 && remaining > maxRemaining {
			maxRemaining = remaining
			bestActive = active
		}
	}

	if bestActive == nil || maxRemaining == 0 {
		return false
	}

	if !bestActive.Hedged.CompareAndSwap(0, 1) {
		return false
	}

	current := bestActive.CurrentOffset.Load()
	stopAt := bestActive.StopAt.Load()
	if current >= stopAt {
		return false
	}

	bestActive.SharedMaxOffsetMu.Lock()
	if bestActive.SharedMaxOffset == nil {
		maxOff := &atomic.Int64{}
		maxOff.Store(current)
		bestActive.SharedMaxOffset = maxOff
	}
	hedgedTask := Task{
		Offset:          current,
		Length:          stopAt - current,
		SharedMaxOffset: bestActive.SharedMaxOffset,
	}
	bestActive.SharedMaxOffsetMu.Unlock()

	queue.Push(hedgedTask)
	log.Printf("[Balancer] Hedged task (range: %d-%d)", hedgedTask.Offset, hedgedTask.Offset+hedgedTask.Length)
	return true
}
