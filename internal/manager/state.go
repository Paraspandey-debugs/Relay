package manager

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

func (m *Manager) loadState() error {
	b, err := os.ReadFile(m.statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	var st persistedState
	if err := json.Unmarshal(b, &st); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.jobs = make(map[string]*managedDownload, len(st.Downloads))
	for _, rec := range st.Downloads {
		if rec.ID == "" {
			continue
		}
		if rec.Status == StatusDownloading {
			rec.Status = StatusPaused
			rec.Error = ""
			rec.UpdatedAt = time.Now()
		}
		copied := rec
		m.jobs[rec.ID] = &managedDownload{rec: copied}
	}

	m.queue = make([]string, 0, len(st.Queue))
	for _, id := range st.Queue {
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
	st := persistedState{
		Version: 1,
		Queue:   append([]string(nil), m.queue...),
		SavedAt: time.Now(),
	}

	st.Downloads = make([]DownloadRecord, 0, len(m.jobs))
	for _, job := range m.jobs {
		st.Downloads = append(st.Downloads, job.rec)
	}

	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(m.statePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp := m.statePath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, m.statePath); err != nil {
		return err
	}
	m.lastPersistAt = time.Now()
	return nil
}

func (m *Manager) saveStateIfDueLocked(force bool) error {
	if !force && m.persistEvery > 0 && time.Since(m.lastPersistAt) < m.persistEvery {
		return nil
	}
	return m.saveStateLocked()
}
