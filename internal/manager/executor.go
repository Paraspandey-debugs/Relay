package manager

import (
	"context"
	"errors"
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
	opts := job.rec.Options
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
	switch {
	case err == nil:
		job.rec.Status = StatusCompleted
		job.rec.Error = ""
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
