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
		interval = 250 * time.Millisecond
	}
	tk := time.NewTicker(interval)
	defer tk.Stop()

	// Sliding window for accurate speed calculation (approx 3 seconds)
	const windowSamples = 12
	type sample struct {
		n int64
		t time.Time
	}
	var window [windowSamples]sample
	var windowIdx int
	var numSamples int

	initialN := written.Load()
	now := time.Now()
	
	window[0] = sample{n: initialN, t: now}
	windowIdx = 1
	numSamples = 1

	smoothedETASeconds := 0.0

	send := func(force bool) {
		currT := time.Now()
		currN := written.Load()
		
		// Update sliding window
		window[windowIdx] = sample{n: currN, t: currT}
		windowIdx = (windowIdx + 1) % windowSamples
		if numSamples < windowSamples {
			numSamples++
		}

		// Calculate speed over the sliding window
		oldestIdx := windowIdx
		if numSamples < windowSamples {
			oldestIdx = 0
		}
		
		oldest := window[oldestIdx]
		dn := currN - oldest.n
		dt := currT.Sub(oldest.t).Seconds()
		
		speed := 0.0
		if dt > 0.1 && dn > 0 { // Need at least 100ms delta to avoid div zero
			speed = float64(dn) / dt
		}

		// ETA Calculation
		eta := time.Duration(0)
		if total > 0 && speed > 0 && currN <= total {
			rawETASeconds := float64(total-currN) / speed
			if smoothedETASeconds <= 0 {
				smoothedETASeconds = rawETASeconds
			} else {
				smoothedETASeconds = 0.7*smoothedETASeconds + 0.3*rawETASeconds // Slight smoothing for ETA only
			}
			eta = time.Duration(smoothedETASeconds * float64(time.Second))
		} else {
			smoothedETASeconds = 0
		}

		msg := ProgressMsg{
			Downloaded: currN,
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
	}

	for {
		select {
		case <-tk.C:
			send(false)
		case <-done:
			send(true)
			return
		case <-ctx.Done():
			return
		}
	}
}
