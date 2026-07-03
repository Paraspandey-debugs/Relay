package api

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Paraspandey-debugs/Relay/internal/core/download"
	"github.com/Paraspandey-debugs/Relay/internal/manager"
	"github.com/Paraspandey-debugs/Relay/web"
)

type Server struct {
	mgr  manager.Interface
	cfg  *manager.DaemonConfig
}

func NewServer(mgr manager.Interface, cfg *manager.DaemonConfig) *Server {
	return &Server{mgr: mgr, cfg: cfg}
}

func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()

	// CORS middleware wrapper
	withCORS := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			h(w, r)
		}
	}

	mux.HandleFunc("/api/downloads", withCORS(s.handleDownloads))
	mux.HandleFunc("/api/downloads/", withCORS(s.handleDownloadAction))
	mux.HandleFunc("/api/browser", withCORS(s.handleBrowser))
	mux.HandleFunc("/api/events", withCORS(s.handleEvents))
	mux.HandleFunc("/api/config", withCORS(s.handleConfig))

	// Serve the embedded static frontend
	distFS, err := fs.Sub(web.FrontendFS, "dist")
	if err == nil {
		mux.Handle("/", http.FileServer(http.FS(distFS)))
	}

	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleDownloads(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		list := s.mgr.List()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)
		return
	}

	if r.Method == "POST" {
		var req struct {
			URL         string `json:"url"`
			Destination string `json:"destination"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Use empty options so the executor applies the manager's default workers.
		addReq := manager.AddRequest{
			URL:         req.URL,
			Destination: req.Destination,
			Options:     download.Options{},
		}

		id, err := s.mgr.Add(addReq)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": id})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleDownloadAction(w http.ResponseWriter, r *http.Request) {
	// Path is like /api/downloads/{id}/{action}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/downloads/"), "/")
	if len(parts) < 1 || parts[0] == "" {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	id := parts[0]

	if r.Method == "DELETE" {
		if err := s.mgr.Remove(id, true); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method == "POST" {
		if len(parts) < 2 {
			http.Error(w, "Action required", http.StatusBadRequest)
			return
		}
		action := parts[1]

		var err error
		switch action {
		case "pause":
			err = s.mgr.Pause(id)
		case "resume":
			err = s.mgr.Resume(id)
		default:
			http.Error(w, "Unknown action", http.StatusBadRequest)
			return
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

type BrowserEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

func (s *Server) handleBrowser(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	target := r.URL.Query().Get("path")
	if target == "" {
		home, _ := os.UserHomeDir()
		target = home
	}
	target = filepath.Clean(target)

	entries, err := os.ReadDir(target)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	out := []BrowserEntry{}
	parent := filepath.Dir(target)
	if target != parent {
		out = append(out, BrowserEntry{Name: "..", Path: parent, IsDir: true})
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue // Web helper only shows directories for destination selection
		}
		out = append(out, BrowserEntry{
			Name:  e.Name(),
			Path:  filepath.Join(target, e.Name()),
			IsDir: true,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"current": target,
		"entries": out,
	})
}

// handleConfig returns or updates the daemon configuration.
// GET /api/config -> returns current config (read-only view)
// PUT /api/config -> applies runtime-settable fields (concurrency, workers)
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		resp := s.cfg.ToResponse()
		// Overlay live runtime values from the manager
		resp.Concurrency = s.mgr.GetMaxConcurrent()
		resp.Workers = s.mgr.GetDefaultWorkers()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)

	case "PUT":
		var update manager.DaemonConfigUpdate
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Persist the updated config to disk
		if s.cfg.ApplyUpdate(update) {
			if err := s.cfg.Save(); err != nil {
				http.Error(w, "Failed to save config: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		// Apply runtime fields to the manager immediately
		if update.Concurrency != nil {
			s.mgr.SetMaxConcurrent(*update.Concurrency)
		}
		if update.Workers != nil {
			s.mgr.SetDefaultWorkers(*update.Workers)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := s.mgr.Subscribe()
	defer s.mgr.Unsubscribe(ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
			flusher.Flush()
		}
	}
}
