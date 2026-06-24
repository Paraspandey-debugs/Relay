package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Paraspandey-debugs/Relay/internal/core/download"
	"github.com/Paraspandey-debugs/Relay/internal/manager"
)

type Server struct {
	mgr *manager.Manager
}

func NewServer(mgr *manager.Manager) *Server {
	return &Server{mgr: mgr}
}

func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()

	// CORS middleware wrapper
	withCORS := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
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

		addReq := manager.AddRequest{
			URL:         req.URL,
			Destination: req.Destination,
			Options:     download.DefaultOptions(),
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
