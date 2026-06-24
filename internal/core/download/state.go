package download

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type downloadState struct {
	URL          string         `json:"url"`
	FinalPath    string         `json:"final_path"`
	PartPath     string         `json:"part_path"`
	Total        int64          `json:"total"`
	ETag         string         `json:"etag"`
	LastModified string         `json:"last_modified"`
	Segments     []segmentState `json:"segments"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type segmentState struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
	Next  int64 `json:"next"`
	Done  bool  `json:"done"`
}

func readState(path string) (*downloadState, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var st downloadState
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, err
	}
	if st.PartPath == "" || st.FinalPath == "" || st.URL == "" {
		return nil, errors.New("invalid state")
	}
	return &st, nil
}

func writeState(path string, st *downloadState) error {
	st.UpdatedAt = time.Now()
	tmp := path + ".tmp"
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func computeDone(segs []segmentState) int64 {
	var n int64
	for _, s := range segs {
		if s.Next > s.Start {
			n += s.Next - s.Start
		}
	}
	return n
}
