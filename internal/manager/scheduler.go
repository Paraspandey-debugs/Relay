package manager

import (
	"context"
	"time"
)

func (m *Manager) scheduleLocked() {
	for len(m.active) < m.maxConcurrent && len(m.queue) > 0 {
		id := m.queue[0]
		m.queue = m.queue[1:]

		job, ok := m.jobs[id]
		if !ok {
			continue
		}
		if job.rec.Status != StatusQueued {
			continue
		}

		ctx, cancel := context.WithCancel(context.Background())
		job.cancel = cancel
		job.rec.Status = StatusDownloading
		job.rec.Error = ""
		now := time.Now()
		job.rec.StartedAt = now
		job.rec.UpdatedAt = now
		m.active[id] = struct{}{}
		m.publishLocked(Event{Type: EventStarted, ID: id, Status: StatusDownloading, At: now})
		_ = m.saveStateIfDueLocked(true)

		m.wg.Add(1)
		go m.runDownload(ctx, id)
	}
}

func (m *Manager) publishLocked(e Event) {
	m.subMutex.RLock()
	defer m.subMutex.RUnlock()
	for _, ch := range m.subscribers {
		select {
		case ch <- e:
		default:
		}
	}
}
