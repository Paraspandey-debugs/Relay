package manager

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/Paraspandey-debugs/Relay/internal/core/download"
)

type Status string

const (
	StatusQueued      Status = "queued"
	StatusDownloading Status = "downloading"
	StatusPaused      Status = "paused"
	StatusCompleted   Status = "completed"
	StatusErrored     Status = "errored"
)

type EventType string

const (
	EventQueued    EventType = "queued"
	EventStarted   EventType = "started"
	EventProgress  EventType = "progress"
	EventPaused    EventType = "paused"
	EventCompleted EventType = "completed"
	EventErrored   EventType = "errored"
	EventRemoved   EventType = "removed"
)

type Config struct {
	MaxConcurrent int
	StatePath     string
	EventBuffer   int
	AutoStart     bool
}

type AddRequest struct {
	URL         string
	Destination string
	Options     download.Options
}

type ProgressInfo struct {
	Downloaded int64         `json:"downloaded"`
	Total      int64         `json:"total"`
	SpeedBps   float64       `json:"speed_bps"`
	ETA        time.Duration `json:"eta"`
	Workers    int           `json:"workers"`
	Retries    int64         `json:"retries"`
}

type DownloadRecord struct {
	ID          string           `json:"id"`
	URL         string           `json:"url"`
	Destination string           `json:"destination"`
	Status      Status           `json:"status"`
	Progress    ProgressInfo     `json:"progress"`
	Options     download.Options `json:"options"`
	Error       string           `json:"error,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

type Event struct {
	Type     EventType     `json:"type"`
	ID       string        `json:"id"`
	Status   Status        `json:"status"`
	Progress *ProgressInfo `json:"progress,omitempty"`
	Error    string        `json:"error,omitempty"`
	At       time.Time     `json:"at"`
}

type persistedState struct {
	Version   int              `json:"version"`
	Queue     []string         `json:"queue"`
	Downloads []DownloadRecord `json:"downloads"`
	SavedAt   time.Time        `json:"saved_at"`
}

func newID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

func containsID(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func removeID(ids []string, id string) []string {
	if len(ids) == 0 {
		return ids
	}
	out := ids[:0]
	for _, v := range ids {
		if v != id {
			out = append(out, v)
		}
	}
	return out
}
