package manager

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// DaemonConfig holds the daemon's startup configuration that can be
// persisted to a JSON file and edited via the web UI.
type DaemonConfig struct {
	mu sync.RWMutex

	// Path to the config file on disk.
	filePath string

	// Startup flags (require daemon restart to take full effect).
	StatePath   string `json:"state_path"`
	LogPath     string `json:"log_path"`
	APIPort     int    `json:"api_port"`
	Concurrency int    `json:"concurrency"`
	Workers     int    `json:"workers"`
	Headless    bool   `json:"headless"`
	OpenWeb     bool   `json:"open_web"`
	Theme       string `json:"theme"`
	RefreshMS   int    `json:"refresh_ms"`
	Cleanup     bool   `json:"cleanup"`
}

// DaemonConfigResponse is the JSON-serializable response for the API.
type DaemonConfigResponse struct {
	StatePath   string `json:"state_path"`
	LogPath     string `json:"log_path"`
	APIPort     int    `json:"api_port"`
	Concurrency int    `json:"concurrency"`
	Workers     int    `json:"workers"`
	Headless    bool   `json:"headless"`
	OpenWeb     bool   `json:"open_web"`
	Theme       string `json:"theme"`
	RefreshMS   int    `json:"refresh_ms"`
	Cleanup     bool   `json:"cleanup"`
}

// DaemonConfigUpdate is the JSON body accepted by PUT /api/config.
// Only non-zero fields are applied.
type DaemonConfigUpdate struct {
	Concurrency *int `json:"concurrency,omitempty"`
	Workers     *int `json:"workers,omitempty"`
}

// DefaultConfigPath returns the default path for the daemon config file.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "relay-daemon.json"
	}
	return filepath.Join(home, ".config", "relay", "daemon.json")
}

// LoadDaemonConfig reads the config file at the given path, or returns
// defaults if the file does not exist.
func LoadDaemonConfig(filePath string) (*DaemonConfig, error) {
	dc := &DaemonConfig{
		filePath:    filePath,
		Concurrency: 3,
		Workers:     4,
		APIPort:     8080,
		Headless:    true,
		Theme:       "ocean",
		RefreshMS:   250,
		Cleanup:     true,
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return dc, nil // defaults
		}
		return nil, fmt.Errorf("reading daemon config: %w", err)
	}

	if err := json.Unmarshal(data, dc); err != nil {
		return nil, fmt.Errorf("parsing daemon config: %w", err)
	}
	dc.filePath = filePath
	return dc, nil
}

// Save persists the current config to disk.
func (dc *DaemonConfig) Save() error {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	if dc.filePath == "" {
		return nil
	}

	dir := filepath.Dir(dc.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := json.MarshalIndent(dc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(dc.filePath, data, 0o644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

// ToResponse returns a JSON-safe copy of the config.
func (dc *DaemonConfig) ToResponse() DaemonConfigResponse {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	return DaemonConfigResponse{
		StatePath:   dc.StatePath,
		LogPath:     dc.LogPath,
		APIPort:     dc.APIPort,
		Concurrency: dc.Concurrency,
		Workers:     dc.Workers,
		Headless:    dc.Headless,
		OpenWeb:     dc.OpenWeb,
		Theme:       dc.Theme,
		RefreshMS:   dc.RefreshMS,
		Cleanup:     dc.Cleanup,
	}
}

// ApplyUpdate applies runtime-settable fields from an update request.
// Returns true if any value changed.
func (dc *DaemonConfig) ApplyUpdate(update DaemonConfigUpdate) bool {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	changed := false
	if update.Concurrency != nil && *update.Concurrency != dc.Concurrency {
		dc.Concurrency = *update.Concurrency
		changed = true
	}
	if update.Workers != nil && *update.Workers != dc.Workers {
		dc.Workers = *update.Workers
		changed = true
	}
	return changed
}