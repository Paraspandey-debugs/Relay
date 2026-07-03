package download

import (
	"context"
	"log"
	"time"
)

// RunHealthMonitor periodically checks for stalled or slow workers and cancels
// them so their remaining work gets requeued by the worker loop.
//
// Two-pass algorithm (mirrors Surge's health.go):
//  1. Compute mean EMA speed across all active workers.
//  2. For each worker:
//     - Skip workers in their grace period (recently started task).
//     - Skip workers waiting on the rate limiter.
//     - Cancel if no data received for > StallTimeout.
//     - Cancel if speed < SlowWorkerThreshold * meanSpeed (when > 1 worker).
func (b *Balancer) RunHealthMonitor(ctx context.Context, queue *TaskQueue, cfg Options) {
	ticker := time.NewTicker(cfg.StallTimeout / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.checkWorkerHealth(cfg)
		}
	}
}

func (b *Balancer) checkWorkerHealth(cfg Options) {
	b.activeMu.Lock()
	defer b.activeMu.Unlock()

	if len(b.activeTasks) == 0 {
		return
	}

	now := time.Now()

	// ── Pass 1: compute mean EMA speed across workers that have a measurement ──
	var totalSpeed float64
	var speedCount int
	for _, active := range b.activeTasks {
		if speed := active.GetSpeed(); speed > 0 {
			totalSpeed += speed
			speedCount++
		}
	}
	var meanSpeed float64
	if speedCount > 0 {
		meanSpeed = totalSpeed / float64(speedCount)
	}

	// ── Pass 2: evaluate each worker ──
	for id, active := range b.activeTasks {
		// Never cancel a worker that is intentionally throttled by the limiter.
		if active.WaitingOnLimiter.Load() {
			continue
		}

		taskDuration := now.Sub(active.StartTime)

		// Grace period: new tasks are immune from slow-worker cancellation.
		if taskDuration < cfg.SlowWorkerGracePeriod {
			continue
		}

		// Absolute stall check: no bytes received for > StallTimeout.
		if lastAct := active.LastActivity.Load(); lastAct > 0 {
			timeSinceData := now.Sub(time.Unix(0, lastAct))
			if timeSinceData >= cfg.StallTimeout {
				log.Printf("[Health] Worker %d stalled (%v no data), cancelling and requeuing", id, timeSinceData.Truncate(time.Millisecond))
				if active.Cancel != nil {
					active.Cancel()
				}
				continue // already cancelled, skip relative check
			}
		}

		// Relative slow-worker check (only meaningful with ≥ 2 workers).
		if meanSpeed > 0 && cfg.SlowWorkerThreshold > 0 && speedCount >= 2 {
			workerSpeed := active.GetSpeed()
			if workerSpeed > 0 && workerSpeed < cfg.SlowWorkerThreshold*meanSpeed {
				log.Printf("[Health] Worker %d slow (%.1f KB/s vs mean %.1f KB/s), cancelling",
					id, workerSpeed/1024, meanSpeed/1024)
				if active.Cancel != nil {
					active.Cancel()
				}
			}
		}
	}
}

