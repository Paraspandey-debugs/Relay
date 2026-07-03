package manager

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type RemoteManager struct {
	baseURL string
	client  *http.Client
	
	subMutex    sync.RWMutex
	subscribers map[<-chan Event]chan Event
	
	ctx    context.Context
	cancel context.CancelFunc
}

func NewRemoteManager(baseURL string) (*RemoteManager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	rm := &RemoteManager{
		baseURL:     baseURL,
		client:      &http.Client{Timeout: 5 * time.Second},
		subscribers: make(map[<-chan Event]chan Event),
		ctx:         ctx,
		cancel:      cancel,
	}
	
	go rm.streamSSE()
	
	return rm, nil
}

func (r *RemoteManager) Ping() error {
	resp, err := r.client.Get(r.baseURL + "/api/downloads")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %d", resp.StatusCode)
	}
	return nil
}

func (r *RemoteManager) Close() {
	r.cancel()
}

func (r *RemoteManager) doRequest(method, endpoint string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewBuffer(b)
	}
	
	req, err := http.NewRequestWithContext(r.ctx, method, r.baseURL+endpoint, bodyReader)
	if err != nil {
		return nil, err
	}
	
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	
	return r.client.Do(req)
}

func (r *RemoteManager) Add(req AddRequest) (string, error) {
	resp, err := r.doRequest("POST", "/api/downloads", req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("API error: %d", resp.StatusCode)
	}
	
	var res map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	return res["id"], nil
}

func (r *RemoteManager) Pause(id string) error {
	resp, err := r.doRequest("POST", "/api/downloads/"+id+"/pause", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (r *RemoteManager) Resume(id string) error {
	resp, err := r.doRequest("POST", "/api/downloads/"+id+"/resume", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (r *RemoteManager) Remove(id string, cleanupPartials bool) error {
	resp, err := r.doRequest("DELETE", "/api/downloads/"+id, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (r *RemoteManager) ReorderQueue(ids []string) error {
	return nil
}

func (r *RemoteManager) List() []DownloadRecord {
	resp, err := r.doRequest("GET", "/api/downloads", nil)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	
	var list []DownloadRecord
	json.NewDecoder(resp.Body).Decode(&list)
	return list
}

func (r *RemoteManager) FindDuplicate(url, destination string) (DownloadRecord, bool) {
	for _, rec := range r.List() {
		if rec.URL == url && rec.Destination == destination {
			return rec, true
		}
	}
	return DownloadRecord{}, false
}

func (r *RemoteManager) Get(id string) (DownloadRecord, bool) {
	for _, rec := range r.List() {
		if rec.ID == id {
			return rec, true
		}
	}
	return DownloadRecord{}, false
}

func (r *RemoteManager) Queue() []string {
	var queue []string
	for _, rec := range r.List() {
		if rec.Status == StatusQueued {
			queue = append(queue, rec.ID)
		}
	}
	return queue
}

func (r *RemoteManager) Subscribe() <-chan Event {
	ch := make(chan Event, 256)
	r.subMutex.Lock()
	r.subscribers[ch] = ch
	r.subMutex.Unlock()
	return ch
}

func (r *RemoteManager) Unsubscribe(ch <-chan Event) {
	if ch == nil {
		return
	}
	r.subMutex.Lock()
	if c, ok := r.subscribers[ch]; ok {
		delete(r.subscribers, ch)
		close(c)
	}
	r.subMutex.Unlock()
}

func (r *RemoteManager) broadcast(e Event) {
	r.subMutex.RLock()
	defer r.subMutex.RUnlock()
	for _, ch := range r.subscribers {
		select {
		case ch <- e:
		default:
		}
	}
}

func (r *RemoteManager) streamSSE() {
	sseClient := &http.Client{Timeout: 0}
	backoff := time.Second
	
	for {
		select {
		case <-r.ctx.Done():
			return
		default:
		}
		
		req, err := http.NewRequestWithContext(r.ctx, "GET", r.baseURL+"/api/events", nil)
		if err == nil {
			req.Header.Set("Accept", "text/event-stream")
			resp, err := sseClient.Do(req)
			if err == nil && resp.StatusCode == 200 {
				backoff = time.Second
				reader := bufio.NewReader(resp.Body)
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						break
					}
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "data: ") {
						data := strings.TrimPrefix(line, "data: ")
						var e Event
						if err := json.Unmarshal([]byte(data), &e); err == nil {
							r.broadcast(e)
						}
					}
				}
				resp.Body.Close()
			} else if resp != nil {
				resp.Body.Close()
			}
		}
		
		select {
		case <-r.ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}
