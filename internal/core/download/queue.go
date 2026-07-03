package download

import (
	"sync"
	"sync/atomic"
)

type TaskQueue struct {
	mu     sync.Mutex
	tasks  []Task
	cond   *sync.Cond
	closed bool
	idle   int32
}

func NewTaskQueue() *TaskQueue {
	q := &TaskQueue{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *TaskQueue) Push(t Task) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tasks = append(q.tasks, t)
	q.cond.Signal()
}

func (q *TaskQueue) Pop() (Task, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.tasks) == 0 && !q.closed {
		q.cond.Wait()
	}
	if len(q.tasks) == 0 {
		return Task{}, false
	}
	t := q.tasks[0]
	q.tasks = q.tasks[1:]
	return t, true
}

func (q *TaskQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.cond.Broadcast()
}

func (q *TaskQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.tasks)
}

func (q *TaskQueue) IncIdle() {
	atomic.AddInt32(&q.idle, 1)
}

func (q *TaskQueue) DecIdle() {
	atomic.AddInt32(&q.idle, -1)
}

func (q *TaskQueue) IdleWorkers() int32 {
	return atomic.LoadInt32(&q.idle)
}
