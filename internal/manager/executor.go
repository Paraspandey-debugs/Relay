package manager

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/Paraspandey-debugs/Relay/internal/core/download"
)

func (m *Manager) runDownload(ctx context.Context, id string) {
	defer m.wg.Done()

	m.mu.RLock()
	job, ok := m.jobs[id]
	if !ok {
		m.mu.RUnlock()
		return
	}
	url := job.rec.URL
	dst := job.rec.Destination

	// Merge persisted options with current defaults.
	// This ensures that options which were added when defaults were different
	// (e.g. Workers: 16 stored before we changed the default to 4) are
	// bounded to reasonable values.
	defaults := download.DefaultOptions()
	opts := job.rec.Options
	// Fill in any zero/unset fields from defaults.
	if opts.Workers <= 0 {
		if m.defaultWorkers > 0 {
			opts.Workers = m.defaultWorkers
		} else {
			opts.Workers = defaults.Workers
		}
	}
	// Hard cap: never open more than 8 connections to a single host to avoid 429 storms.
	const maxWorkersPerHost = 8
	if opts.Workers > maxWorkersPerHost {
		opts.Workers = maxWorkersPerHost
	}
	if opts.MaxRetries <= 0 {
		opts.MaxRetries = defaults.MaxRetries
	}
	if opts.BaseBackoff <= 0 {
		opts.BaseBackoff = defaults.BaseBackoff
	}
	if opts.MaxBackoff <= 0 {
		opts.MaxBackoff = defaults.MaxBackoff
	}
	if opts.StallTimeout <= 0 {
		opts.StallTimeout = defaults.StallTimeout
	}
	if opts.SlowWorkerGracePeriod <= 0 {
		opts.SlowWorkerGracePeriod = defaults.SlowWorkerGracePeriod
	}
	if opts.SlowWorkerThreshold <= 0 {
		opts.SlowWorkerThreshold = defaults.SlowWorkerThreshold
	}
	m.mu.RUnlock()

	progressCh := make(chan download.ProgressMsg, 32)
	progressDone := make(chan struct{})

	go func() {
		defer close(progressDone)
		for p := range progressCh {
			m.mu.Lock()
			current, exists := m.jobs[id]
			if !exists {
				m.mu.Unlock()
				continue
			}

			current.rec.Progress = ProgressInfo{
				Downloaded: p.Downloaded,
				Total:      p.Total,
				SpeedBps:   p.SpeedBps,
				ETA:        p.ETA,
				Workers:    p.Workers,
				Retries:    p.Retries,
			}
			current.rec.UpdatedAt = time.Now()
			progressSnapshot := current.rec.Progress
			m.publishLocked(Event{
				Type:     EventProgress,
				ID:       id,
				Status:   current.rec.Status,
				Progress: &progressSnapshot,
				At:       time.Now(),
			})
			_ = m.saveStateIfDueLocked(false)
			m.mu.Unlock()
		}
	}()

	err := download.DownloadFileV2(ctx, url, dst, &opts, progressCh)
	close(progressCh)
	<-progressDone

	// Auto-fallback to single-worker on HTTP 429 rate limit.
	if err != nil && strings.Contains(err.Error(), "rate limited") && opts.Workers > 1 {
		log.Printf("[relay] auto-fallback to single-worker for %s (was %d workers)", url, opts.Workers)
		opts.Workers = 1
		opts.ForceSingle = true
		progressCh = make(chan download.ProgressMsg, 32)
		progressDone = make(chan struct{})
		go func() {
			defer close(progressDone)
			for p := range progressCh {
				m.mu.Lock()
				current, exists := m.jobs[id]
				if !exists {
					m.mu.Unlock()
					continue
				}
				current.rec.Progress = ProgressInfo{
					Downloaded: p.Downloaded,
					Total:      p.Total,
					SpeedBps:   p.SpeedBps,
					ETA:        p.ETA,
					Workers:    1,
					Retries:    p.Retries,
				}
				current.rec.UpdatedAt = time.Now()
				progressSnapshot := current.rec.Progress
				m.publishLocked(Event{
					Type:     EventProgress,
					ID:       id,
					Status:   current.rec.Status,
					Progress: &progressSnapshot,
					At:       time.Now(),
				})
				_ = m.saveStateIfDueLocked(false)
				m.mu.Unlock()
			}
		}()
		err = download.DownloadFileV2(ctx, url, dst, &opts, progressCh)
		close(progressCh)
		<-progressDone
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok = m.jobs[id]
	if !ok {
		delete(m.active, id)
		m.scheduleLocked()
		return
	}

	delete(m.active, id)
	job.cancel = nil

	now := time.Now()
	if !job.rec.StartedAt.IsZero() {
		if ranFor := now.Sub(job.rec.StartedAt); ranFor > 0 {
			job.rec.ActiveFor += ranFor
		}
		job.rec.StartedAt = time.Time{}
	}

	switch {
	case err == nil:
		job.rec.Status = StatusCompleted
		job.rec.Error = ""
		job.rec.CompletedAt = now
		job.rec.UpdatedAt = now
		m.publishLocked(Event{Type: EventCompleted, ID: id, Status: StatusCompleted, At: now})
	case errors.Is(err, context.Canceled):
		if job.rec.Status != StatusPaused {
			job.rec.Status = StatusPaused
			job.rec.UpdatedAt = now
		}
		job.rec.Error = ""
		m.publishLocked(Event{Type: EventPaused, ID: id, Status: StatusPaused, At: now})
	default:
		job.rec.Status = StatusErrored
		job.rec.Error = err.Error()
		job.rec.UpdatedAt = now
		m.publishLocked(Event{Type: EventErrored, ID: id, Status: StatusErrored, Error: err.Error(), At: now})
	}

	_ = m.saveStateIfDueLocked(true)
	m.scheduleLocked()
}
