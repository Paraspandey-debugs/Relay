package binding

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Paraspandey-debugs/Relay/internal/manager"
)

const defaultEventHistory = 512

// Bridge exposes Relay backend operations in a JSON-oriented API suitable for
// mobile bridge layers.
type Bridge struct {
	mu sync.RWMutex

	mgr *manager.Manager

	eventsMu        sync.Mutex
	eventHistory    []eventOut
	maxEventHistory int
	eventPumpWG     sync.WaitGroup
}

func NewBridge() *Bridge {
	return &Bridge{
		maxEventHistory: defaultEventHistory,
		eventHistory:    make([]eventOut, 0, defaultEventHistory),
	}
}

func (b *Bridge) Start(configJSON string) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.mgr != nil {
		return errResponse(errors.New("bridge is already started"))
	}

	var req startRequest
	if configJSON != "" {
		if err := json.Unmarshal([]byte(configJSON), &req); err != nil {
			return errResponse(fmt.Errorf("invalid start config: %w", err))
		}
	}

	autoStart := true
	if req.AutoStart != nil {
		autoStart = *req.AutoStart
	}

	mgr, err := manager.New(manager.Config{
		MaxConcurrent: req.MaxConcurrent,
		StatePath:     req.StatePath,
		EventBuffer:   req.EventBuffer,
		AutoStart:     autoStart,
	})
	if err != nil {
		return errResponse(err)
	}

	b.mgr = mgr
	b.maxEventHistory = req.MaxEventHistory
	if b.maxEventHistory <= 0 {
		b.maxEventHistory = defaultEventHistory
	}

	b.eventsMu.Lock()
	b.eventHistory = b.eventHistory[:0]
	b.eventsMu.Unlock()

	b.eventPumpWG.Add(1)
	go b.consumeEvents(mgr)

	return okResponse(map[string]any{
		"state_path":        req.StatePath,
		"max_concurrent":    req.MaxConcurrent,
		"event_buffer":      req.EventBuffer,
		"auto_start":        autoStart,
		"max_event_history": b.maxEventHistory,
	})
}

func (b *Bridge) Stop(timeoutMS int64) string {
	b.mu.Lock()
	mgr := b.mgr
	if mgr == nil {
		b.mu.Unlock()
		return okResponse(map[string]any{"stopped": true})
	}
	b.mgr = nil
	b.mu.Unlock()

	if timeoutMS <= 0 {
		timeoutMS = 10000
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()

	err := mgr.Shutdown(ctx)
	b.eventPumpWG.Wait()
	if err != nil {
		return errResponse(err)
	}

	return okResponse(map[string]any{"stopped": true})
}

func (b *Bridge) IsRunning() string {
	b.mu.RLock()
	running := b.mgr != nil
	b.mu.RUnlock()
	return okResponse(map[string]bool{"running": running})
}

func (b *Bridge) AddDownload(requestJSON string) string {
	var req addDownloadRequest
	if err := json.Unmarshal([]byte(requestJSON), &req); err != nil {
		return errResponse(fmt.Errorf("invalid add request: %w", err))
	}

	mgr, err := b.getManager()
	if err != nil {
		return errResponse(err)
	}

	id, err := mgr.Add(manager.AddRequest{
		URL:         req.URL,
		Destination: req.Destination,
		Options:     toManagerOptions(req.Options),
	})
	if err != nil {
		return errResponse(err)
	}

	return okResponse(map[string]string{"id": id})
}

func (b *Bridge) PauseDownload(id string) string {
	mgr, err := b.getManager()
	if err != nil {
		return errResponse(err)
	}
	if err := mgr.Pause(id); err != nil {
		return errResponse(err)
	}
	return okResponse(map[string]any{"id": id, "paused": true})
}

func (b *Bridge) ResumeDownload(id string) string {
	mgr, err := b.getManager()
	if err != nil {
		return errResponse(err)
	}
	if err := mgr.Resume(id); err != nil {
		return errResponse(err)
	}
	return okResponse(map[string]any{"id": id, "resumed": true})
}

func (b *Bridge) RemoveDownload(requestJSON string) string {
	var req removeDownloadRequest
	if err := json.Unmarshal([]byte(requestJSON), &req); err != nil {
		return errResponse(fmt.Errorf("invalid remove request: %w", err))
	}

	mgr, err := b.getManager()
	if err != nil {
		return errResponse(err)
	}
	if err := mgr.Remove(req.ID, req.CleanupPartials); err != nil {
		return errResponse(err)
	}
	return okResponse(map[string]any{"id": req.ID, "removed": true})
}

func (b *Bridge) GetDownload(id string) string {
	mgr, err := b.getManager()
	if err != nil {
		return errResponse(err)
	}
	rec, ok := mgr.Get(id)
	if !ok {
		return errResponse(fmt.Errorf("download %s not found", id))
	}
	return okResponse(toRecordOut(rec))
}

func (b *Bridge) ListDownloads() string {
	mgr, err := b.getManager()
	if err != nil {
		return errResponse(err)
	}
	return okResponse(toSortedRecordOutList(mgr.List()))
}

func (b *Bridge) Queue() string {
	mgr, err := b.getManager()
	if err != nil {
		return errResponse(err)
	}
	return okResponse(mgr.Queue())
}

func (b *Bridge) ReorderQueue(idsJSON string) string {
	var ids []string
	if err := json.Unmarshal([]byte(idsJSON), &ids); err != nil {
		return errResponse(fmt.Errorf("invalid queue ids payload: %w", err))
	}

	mgr, err := b.getManager()
	if err != nil {
		return errResponse(err)
	}
	if err := mgr.ReorderQueue(ids); err != nil {
		return errResponse(err)
	}
	return okResponse(map[string]any{"reordered": true})
}

func (b *Bridge) PollEvents(limit int) string {
	b.eventsMu.Lock()
	defer b.eventsMu.Unlock()

	if limit <= 0 || limit > len(b.eventHistory) {
		limit = len(b.eventHistory)
	}

	batch := append([]eventOut(nil), b.eventHistory[:limit]...)
	b.eventHistory = append([]eventOut(nil), b.eventHistory[limit:]...)
	return okResponse(batch)
}

func (b *Bridge) Snapshot() string {
	mgr, err := b.getManager()
	if err != nil {
		return errResponse(err)
	}
	return okResponse(map[string]any{
		"downloads": toSortedRecordOutList(mgr.List()),
		"queue":     mgr.Queue(),
	})
}

func (b *Bridge) getManager() (*manager.Manager, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.mgr == nil {
		return nil, errors.New("bridge is not started")
	}
	return b.mgr, nil
}

func (b *Bridge) consumeEvents(mgr *manager.Manager) {
	defer b.eventPumpWG.Done()

	for e := range mgr.Events() {
		b.eventsMu.Lock()
		b.eventHistory = append(b.eventHistory, toEventOut(e))
		if len(b.eventHistory) > b.maxEventHistory {
			overflow := len(b.eventHistory) - b.maxEventHistory
			b.eventHistory = append([]eventOut(nil), b.eventHistory[overflow:]...)
		}
		b.eventsMu.Unlock()
	}
}
