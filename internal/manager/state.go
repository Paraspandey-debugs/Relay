package manager

import (
	"encoding/json"
	"os"
	"time"
)

func (m *Manager) migrateFromJSONIfNecessary() error {
	b, err := os.ReadFile(m.statePath)
	if err != nil {
		return nil
	}

	var st persistedState
	if err := json.Unmarshal(b, &st); err != nil {
		return err
	}

	records, _, err := m.store.Load()
	if err == nil && len(records) > 0 {
		_ = os.Rename(m.statePath, m.statePath+".bak")
		return nil
	}

	jobs := make(map[string]*managedDownload, len(st.Downloads))
	for _, rec := range st.Downloads {
		if rec.ID == "" {
			continue
		}
		jobs[rec.ID] = &managedDownload{rec: rec}
	}

	if err := m.store.SaveAll(jobs, st.Queue); err != nil {
		return err
	}

	_ = os.Rename(m.statePath, m.statePath+".bak")
	return nil
}

func (m *Manager) loadState() error {
	if err := m.migrateFromJSONIfNecessary(); err != nil {
		return err
	}

	records, queue, err := m.store.Load()
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.jobs = make(map[string]*managedDownload, len(records))
	for _, rec := range records {
		if rec.Status == StatusCompleted {
			if rec.ActiveFor <= 0 && !rec.CompletedAt.IsZero() && !rec.CreatedAt.IsZero() {
				rec.ActiveFor = rec.CompletedAt.Sub(rec.CreatedAt)
			}
			if rec.ActiveFor < 0 {
				rec.ActiveFor = 0
			}
		}
		if rec.Status == StatusDownloading {
			if !rec.StartedAt.IsZero() {
				if ranFor := time.Since(rec.StartedAt); ranFor > 0 {
					rec.ActiveFor += ranFor
				}
			}
			rec.StartedAt = time.Time{}
			rec.Status = StatusPaused
			rec.Error = ""
			rec.UpdatedAt = time.Now()
		}
		m.jobs[rec.ID] = &managedDownload{rec: rec}
	}

	m.queue = make([]string, 0, len(queue))
	for _, id := range queue {
		job, ok := m.jobs[id]
		if !ok {
			continue
		}
		if job.rec.Status == StatusQueued {
			m.queue = append(m.queue, id)
		}
	}

	return nil
}

func (m *Manager) saveStateLocked() error {
	err := m.store.SaveAll(m.jobs, m.queue)
	if err == nil {
		m.lastPersistAt = time.Now()
	}
	return err
}

func (m *Manager) saveStateIfDueLocked(force bool) error {
	if !force && m.persistEvery > 0 && time.Since(m.lastPersistAt) < m.persistEvery {
		return nil
	}
	return m.saveStateLocked()
}
