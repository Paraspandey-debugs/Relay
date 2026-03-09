package download

import (
	"context"
	"math"
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
	smoothedSpeed := 0.0
	varianceSpeed := 0.0
	smoothedETASeconds := 0.0
	const (
		smoothingAlpha      = 0.25
		etaSmoothingAlpha   = 0.30
		idleDecayFactor     = 0.85
		minSampleSeconds    = 0.20
		jitterPenaltyFactor = 1.10
		minNormalizedRatio  = 0.35
	)

	send := func(force bool) {
		now := time.Now()
		n := written.Load()
		dn := n - lastN
		dt := now.Sub(lastT).Seconds()

		// Ignore very short sampling windows when forced (e.g. completion signal),
		// because they produce unrealistic instantaneous speed spikes.
		if dt >= minSampleSeconds {
			if dn < 0 {
				dn = 0
			}
			instant := 0.0
			if dt > 0 {
				instant = float64(dn) / dt
			}

			if dn == 0 {
				smoothedSpeed *= idleDecayFactor
			} else if smoothedSpeed <= 0 {
				smoothedSpeed = instant
			} else {
				smoothedSpeed = (1-smoothingAlpha)*smoothedSpeed + smoothingAlpha*instant
			}
		}

		speed := smoothedSpeed
		if speed < 0 {
			speed = 0
		}

		normalizedSpeed := speed
		if normalizedSpeed > 0 {
			stddev := math.Sqrt(math.Max(varianceSpeed, 0))
			normalizedSpeed = speed - (jitterPenaltyFactor * stddev)
			floor := speed * minNormalizedRatio
			if normalizedSpeed < floor {
				normalizedSpeed = floor
			}
		}

		eta := time.Duration(0)
		if total > 0 && normalizedSpeed > 0 && n <= total {
			rawETASeconds := float64(total-n) / normalizedSpeed
			if rawETASeconds < 0 {
				rawETASeconds = 0
			}
			if smoothedETASeconds <= 0 {
				smoothedETASeconds = rawETASeconds
			} else {
				smoothedETASeconds = (1-etaSmoothingAlpha)*smoothedETASeconds + etaSmoothingAlpha*rawETASeconds
			}
			eta = time.Duration(smoothedETASeconds * float64(time.Second))
		} else {
			smoothedETASeconds = 0
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

		if dn > 0 && dt >= minSampleSeconds {
			instant := float64(dn) / dt
			if speed <= 0 {
				varianceSpeed = 0
			} else {
				delta := instant - speed
				varianceSpeed = (1-smoothingAlpha)*varianceSpeed + smoothingAlpha*delta*delta
			}
		} else if dn == 0 {
			varianceSpeed *= idleDecayFactor
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
