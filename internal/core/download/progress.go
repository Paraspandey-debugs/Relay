package download

import (
	"context"
	"sync/atomic"
	"time"
)

type ProgressMsg struct {
	Downloaded int64
	Total      int64
	SpeedBps   float64
	ETA        time.Duration
	Workers    int
	Retries    int64
}

func emitProgress(
	ctx context.Context,
	written *atomic.Int64,
	total int64,
	workers int32,
	retries *atomic.Int64,
	interval time.Duration,
	ch chan<- ProgressMsg,
	done <-chan struct{},
) {
	if ch == nil {
		return
	}
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	tk := time.NewTicker(interval)
	defer tk.Stop()

	lastN := written.Load()
	lastT := time.Now()

	send := func() {
		now := time.Now()
		n := written.Load()
		dn := n - lastN
		dt := now.Sub(lastT).Seconds()
		speed := 0.0
		if dt > 0 {
			speed = float64(dn) / dt
		}
		eta := time.Duration(0)
		if total > 0 && speed > 0 && n <= total {
			eta = time.Duration(float64(total-n)/speed) * time.Second
		}
		msg := ProgressMsg{
			Downloaded: n,
			Total:      total,
			SpeedBps:   speed,
			ETA:        eta,
			Workers:    int(workers),
			Retries:    retries.Load(),
		}
		select {
		case ch <- msg:
		default:
		}
		lastN = n
		lastT = now
	}

	for {
		select {
		case <-tk.C:
			send()
		case <-done:
			send()
			return
		case <-ctx.Done():
			return
		}
	}
}
