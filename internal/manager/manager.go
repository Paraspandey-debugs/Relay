package manager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type Manager struct {
	mu            sync.RWMutex
	jobs          map[string]*managedDownload
	queue         []string
	active        map[string]struct{}
	maxConcurrent int
	statePath     string
	persistEvery  time.Duration
	lastPersistAt time.Time
	events        chan Event
	autoStart     bool
	closed        bool

	wg sync.WaitGroup
}

type managedDownload struct {
	rec    DownloadRecord
	cancel context.CancelFunc
}

func New(cfg Config) (*Manager, error) {
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 3
	}
	if cfg.StatePath == "" {
		cfg.StatePath = "relay-downloads.state.json"
	}
	if cfg.EventBuffer <= 0 {
		cfg.EventBuffer = 256
	}

	m := &Manager{
		jobs:          make(map[string]*managedDownload),
		queue:         make([]string, 0),
		active:        make(map[string]struct{}),
		maxConcurrent: cfg.MaxConcurrent,
		statePath:     cfg.StatePath,
		persistEvery:  time.Second,
		events:        make(chan Event, cfg.EventBuffer),
		autoStart:     cfg.AutoStart,
	}

	if err := m.loadState(); err != nil {
		return nil, err
	}

	if m.autoStart {
		m.mu.Lock()
		m.scheduleLocked()
		m.mu.Unlock()
	}

	return m, nil
}

func (m *Manager) Events() <-chan Event {
	return m.events
}

func (m *Manager) Add(req AddRequest) (string, error) {
	req.URL = strings.TrimSpace(req.URL)
	req.Destination = strings.TrimSpace(req.Destination)

	if req.URL == "" {
		return "", errors.New("url is required")
	}
	if req.Destination == "" {
		return "", errors.New("destination is required")
	}

	id, err := newID()
	if err != nil {
		return "", err
	}

	now := time.Now()
	entry := &managedDownload{
		rec: DownloadRecord{
			ID:          id,
			URL:         req.URL,
			Destination: req.Destination,
			Status:      StatusQueued,
			Options:     req.Options,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return "", errors.New("manager is closed")
	}

	if existing, ok := m.findDuplicateLocked(req.URL, req.Destination); ok {
		return "", fmt.Errorf("duplicate download already exists (%s)", existing.ID)
	}

	m.jobs[id] = entry
	m.queue = append(m.queue, id)
	m.publishLocked(Event{Type: EventQueued, ID: id, Status: StatusQueued, At: time.Now()})

	if err := m.saveStateIfDueLocked(true); err != nil {
		return "", err
	}
	m.scheduleLocked()

	return id, nil
}

func (m *Manager) FindDuplicate(url, destination string) (DownloadRecord, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.findDuplicateLocked(strings.TrimSpace(url), strings.TrimSpace(destination))
}

func (m *Manager) findDuplicateLocked(url, destination string) (DownloadRecord, bool) {
	for _, job := range m.jobs {
		if job.rec.URL == url && job.rec.Destination == destination {
			return job.rec, true
		}
	}
	return DownloadRecord{}, false
}

func (m *Manager) Pause(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("manager is closed")
	}

	job, ok := m.jobs[id]
	if !ok {
		return fmt.Errorf("download %s not found", id)
	}
	if job.rec.Status != StatusDownloading {
		return fmt.Errorf("download %s is not downloading", id)
	}

	if job.cancel != nil {
		job.cancel()
	}
	job.rec.Status = StatusPaused
	job.rec.Error = ""
	job.rec.UpdatedAt = time.Now()
	m.publishLocked(Event{Type: EventPaused, ID: id, Status: StatusPaused, At: time.Now()})

	return m.saveStateIfDueLocked(true)
}

func (m *Manager) Resume(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("manager is closed")
	}

	job, ok := m.jobs[id]
	if !ok {
		return fmt.Errorf("download %s not found", id)
	}
	if job.rec.Status != StatusPaused && job.rec.Status != StatusErrored {
		return fmt.Errorf("download %s is not paused or errored", id)
	}
	if _, running := m.active[id]; running {
		return fmt.Errorf("download %s is already running", id)
	}

	if !containsID(m.queue, id) {
		m.queue = append(m.queue, id)
	}
	job.rec.Status = StatusQueued
	job.rec.Error = ""
	job.rec.UpdatedAt = time.Now()
	m.publishLocked(Event{Type: EventQueued, ID: id, Status: StatusQueued, At: time.Now()})

	if err := m.saveStateIfDueLocked(true); err != nil {
		return err
	}
	m.scheduleLocked()
	return nil
}

func (m *Manager) Remove(id string, cleanupPartials bool) error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return errors.New("manager is closed")
	}

	job, ok := m.jobs[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("download %s not found", id)
	}

	if job.cancel != nil {
		job.cancel()
	}
	m.queue = removeID(m.queue, id)
	delete(m.active, id)
	delete(m.jobs, id)
	m.publishLocked(Event{Type: EventRemoved, ID: id, Status: StatusPaused, At: time.Now()})

	err := m.saveStateIfDueLocked(true)
	m.scheduleLocked()
	m.mu.Unlock()
	if err != nil {
		return err
	}

	if cleanupPartials {
		_ = os.Remove(job.rec.Destination + ".part")
		_ = os.Remove(job.rec.Destination + ".part.state.json")
	}
	return nil
}

func (m *Manager) ReorderQueue(ids []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("manager is closed")
	}

	if len(ids) != len(m.queue) {
		return errors.New("queue reorder must include all queued IDs")
	}

	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			return fmt.Errorf("duplicate id in queue reorder: %s", id)
		}
		seen[id] = struct{}{}

		job, ok := m.jobs[id]
		if !ok {
			return fmt.Errorf("download %s not found", id)
		}
		if job.rec.Status != StatusQueued {
			return fmt.Errorf("download %s is not queued", id)
		}
	}

	m.queue = append([]string(nil), ids...)
	return m.saveStateIfDueLocked(true)
}

func (m *Manager) Get(id string) (DownloadRecord, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.jobs[id]
	if !ok {
		return DownloadRecord{}, false
	}
	return job.rec, true
}

func (m *Manager) List() []DownloadRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]DownloadRecord, 0, len(m.jobs))
	for _, job := range m.jobs {
		out = append(out, job.rec)
	}
	return out
}

func (m *Manager) Queue() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return append([]string(nil), m.queue...)
}

func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true

	for id, job := range m.jobs {
		if job.cancel != nil {
			job.cancel()
		}
		if job.rec.Status == StatusDownloading {
			job.rec.Status = StatusPaused
			job.rec.UpdatedAt = time.Now()
		}
		delete(m.active, id)
	}

	err := m.saveStateIfDueLocked(true)
	m.mu.Unlock()

	waitCh := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(waitCh)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-waitCh:
	}

	close(m.events)
	return err
}
