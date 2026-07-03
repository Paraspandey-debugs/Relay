package download

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Task represents a byte-range unit of work.
type Task struct {
	Offset          int64
	Length          int64
	SharedMaxOffset *atomic.Int64 // non-nil when this task is part of a hedged pair
}

// ActiveTask tracks a task currently being processed by a worker.
type ActiveTask struct {
	Task
	CurrentOffset atomic.Int64
	StopAt        atomic.Int64

	// Health monitoring
	LastActivity atomic.Int64       // Unix nano of last bytes received
	StartTime    time.Time          // When this task started (for grace period)
	Cancel       context.CancelFunc // Abort this task; worker requeues remaining

	// EMA speed tracking (2-second sliding window)
	Speed       float64       // smoothed bytes/sec (protected by SpeedMu)
	SpeedMu     sync.Mutex
	WindowStart time.Time    // start of current measurement window
	WindowBytes atomic.Int64 // bytes downloaded in the current window

	// Set true while blocked on rate limiter so health monitor ignores us
	WaitingOnLimiter atomic.Bool

	// Hedging
	Hedged            atomic.Int32  // 1 if an idle worker is already racing this task
	SharedMaxOffsetMu sync.RWMutex // protects SharedMaxOffset initialisation
}

// RemainingBytes returns bytes left to download for this task.
func (a *ActiveTask) RemainingBytes() int64 {
	cur := a.CurrentOffset.Load()
	stop := a.StopAt.Load()
	if cur >= stop {
		return 0
	}
	return stop - cur
}

// RemainingTask returns a Task representing the unfinished portion, or nil.
func (a *ActiveTask) RemainingTask() *Task {
	cur := a.CurrentOffset.Load()
	stop := a.StopAt.Load()
	if cur >= stop {
		return nil
	}
	return &Task{Offset: cur, Length: stop - cur}
}

// GetSpeed returns the EMA-smoothed speed, decayed if the worker has been
// inactive for more than 2 seconds (mirrors Surge's decay logic).
func (a *ActiveTask) GetSpeed() float64 {
	a.SpeedMu.Lock()
	speed := a.Speed
	a.SpeedMu.Unlock()

	lastActivity := a.LastActivity.Load()
	if lastActivity == 0 {
		return speed
	}
	since := time.Since(time.Unix(0, lastActivity))
	const decayThreshold = 2 * time.Second
	if since > decayThreshold {
		decayFactor := float64(decayThreshold) / float64(since)
		speed *= decayFactor
	}
	return speed
}
