package binding

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/Paraspandey-debugs/Relay/internal/core/download"
	"github.com/Paraspandey-debugs/Relay/internal/manager"
)

type apiResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	Data  any    `json:"data,omitempty"`
}

type startRequest struct {
	StatePath       string `json:"state_path"`
	MaxConcurrent   int    `json:"max_concurrent"`
	EventBuffer     int    `json:"event_buffer"`
	AutoStart       *bool  `json:"auto_start,omitempty"`
	MaxEventHistory int    `json:"max_event_history"`
}

type addDownloadRequest struct {
	URL         string             `json:"url"`
	Destination string             `json:"destination"`
	Options     *downloadOptionsIn `json:"options,omitempty"`
}

type removeDownloadRequest struct {
	ID              string `json:"id"`
	CleanupPartials bool   `json:"cleanup_partials"`
}

type downloadOptionsIn struct {
	Workers            *int    `json:"workers,omitempty"`
	MinChunkSize       *int64  `json:"min_chunk_size,omitempty"`
	MaxChunkSize       *int64  `json:"max_chunk_size,omitempty"`
	TimeoutMS          *int64  `json:"timeout_ms,omitempty"`
	MaxRetries         *int    `json:"max_retries,omitempty"`
	BaseBackoffMS      *int64  `json:"base_backoff_ms,omitempty"`
	MaxBackoffMS       *int64  `json:"max_backoff_ms,omitempty"`
	UserAgent          *string `json:"user_agent,omitempty"`
	ExpectedSHA256Hex  *string `json:"expected_sha256_hex,omitempty"`
	NoResume           *bool   `json:"no_resume,omitempty"`
	ProgressIntervalMS *int64  `json:"progress_interval_ms,omitempty"`
	ForceSingle        *bool   `json:"force_single,omitempty"`
	RequireAcceptRange *bool   `json:"require_accept_range,omitempty"`
}

type progressOut struct {
	Downloaded int64   `json:"downloaded"`
	Total      int64   `json:"total"`
	SpeedBps   float64 `json:"speed_bps"`
	ETAMS      int64   `json:"eta_ms"`
	Workers    int     `json:"workers"`
	Retries    int64   `json:"retries"`
}

type downloadOptionsOut struct {
	Workers            int    `json:"workers"`
	MinChunkSize       int64  `json:"min_chunk_size"`
	MaxChunkSize       int64  `json:"max_chunk_size"`
	TimeoutMS          int64  `json:"timeout_ms"`
	MaxRetries         int    `json:"max_retries"`
	BaseBackoffMS      int64  `json:"base_backoff_ms"`
	MaxBackoffMS       int64  `json:"max_backoff_ms"`
	UserAgent          string `json:"user_agent"`
	ExpectedSHA256Hex  string `json:"expected_sha256_hex,omitempty"`
	NoResume           bool   `json:"no_resume"`
	ProgressIntervalMS int64  `json:"progress_interval_ms"`
	ForceSingle        bool   `json:"force_single"`
	RequireAcceptRange bool   `json:"require_accept_range"`
}

type downloadRecordOut struct {
	ID          string             `json:"id"`
	URL         string             `json:"url"`
	Destination string             `json:"destination"`
	Status      string             `json:"status"`
	Progress    progressOut        `json:"progress"`
	Options     downloadOptionsOut `json:"options"`
	Error       string             `json:"error,omitempty"`
	StartedAt   string             `json:"started_at,omitempty"`
	CompletedAt string             `json:"completed_at,omitempty"`
	ActiveForMS int64              `json:"active_for_ms"`
	CreatedAt   string             `json:"created_at"`
	UpdatedAt   string             `json:"updated_at"`
}

type eventOut struct {
	Type     string       `json:"type"`
	ID       string       `json:"id"`
	Status   string       `json:"status"`
	Progress *progressOut `json:"progress,omitempty"`
	Error    string       `json:"error,omitempty"`
	At       string       `json:"at"`
}

func marshalResponse(v apiResponse) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `{"ok":false,"error":"failed to encode response"}`
	}
	return string(b)
}

func okResponse(data any) string {
	return marshalResponse(apiResponse{OK: true, Data: data})
}

func errResponse(err error) string {
	if err == nil {
		return marshalResponse(apiResponse{OK: false, Error: "unknown error"})
	}
	return marshalResponse(apiResponse{OK: false, Error: err.Error()})
}

func toManagerOptions(in *downloadOptionsIn) download.Options {
	opt := download.DefaultOptions()
	if in == nil {
		return opt
	}

	if in.Workers != nil {
		opt.Workers = *in.Workers
	}
	if in.MinChunkSize != nil {
		opt.MinChunkSize = *in.MinChunkSize
	}
	if in.MaxChunkSize != nil {
		opt.MaxChunkSize = *in.MaxChunkSize
	}
	if in.TimeoutMS != nil {
		opt.Timeout = time.Duration(*in.TimeoutMS) * time.Millisecond
	}
	if in.MaxRetries != nil {
		opt.MaxRetries = *in.MaxRetries
	}
	if in.BaseBackoffMS != nil {
		opt.BaseBackoff = time.Duration(*in.BaseBackoffMS) * time.Millisecond
	}
	if in.MaxBackoffMS != nil {
		opt.MaxBackoff = time.Duration(*in.MaxBackoffMS) * time.Millisecond
	}
	if in.UserAgent != nil {
		opt.UserAgent = *in.UserAgent
	}
	if in.ExpectedSHA256Hex != nil {
		opt.ExpectedSHA256Hex = *in.ExpectedSHA256Hex
	}
	if in.NoResume != nil {
		opt.NoResume = *in.NoResume
	}
	if in.ProgressIntervalMS != nil {
		opt.ProgressInterval = time.Duration(*in.ProgressIntervalMS) * time.Millisecond
	}
	if in.ForceSingle != nil {
		opt.ForceSingle = *in.ForceSingle
	}
	if in.RequireAcceptRange != nil {
		opt.RequireAcceptRange = *in.RequireAcceptRange
	}

	return opt
}

func toProgressOut(in manager.ProgressInfo) progressOut {
	return progressOut{
		Downloaded: in.Downloaded,
		Total:      in.Total,
		SpeedBps:   in.SpeedBps,
		ETAMS:      in.ETA.Milliseconds(),
		Workers:    in.Workers,
		Retries:    in.Retries,
	}
}

func toOptionsOut(in download.Options) downloadOptionsOut {
	return downloadOptionsOut{
		Workers:            in.Workers,
		MinChunkSize:       in.MinChunkSize,
		MaxChunkSize:       in.MaxChunkSize,
		TimeoutMS:          in.Timeout.Milliseconds(),
		MaxRetries:         in.MaxRetries,
		BaseBackoffMS:      in.BaseBackoff.Milliseconds(),
		MaxBackoffMS:       in.MaxBackoff.Milliseconds(),
		UserAgent:          in.UserAgent,
		ExpectedSHA256Hex:  in.ExpectedSHA256Hex,
		NoResume:           in.NoResume,
		ProgressIntervalMS: in.ProgressInterval.Milliseconds(),
		ForceSingle:        in.ForceSingle,
		RequireAcceptRange: in.RequireAcceptRange,
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func toRecordOut(in manager.DownloadRecord) downloadRecordOut {
	return downloadRecordOut{
		ID:          in.ID,
		URL:         in.URL,
		Destination: in.Destination,
		Status:      string(in.Status),
		Progress:    toProgressOut(in.Progress),
		Options:     toOptionsOut(in.Options),
		Error:       in.Error,
		StartedAt:   formatTime(in.StartedAt),
		CompletedAt: formatTime(in.CompletedAt),
		ActiveForMS: in.ActiveFor.Milliseconds(),
		CreatedAt:   formatTime(in.CreatedAt),
		UpdatedAt:   formatTime(in.UpdatedAt),
	}
}

func toEventOut(in manager.Event) eventOut {
	out := eventOut{
		Type:   string(in.Type),
		ID:     in.ID,
		Status: string(in.Status),
		Error:  in.Error,
		At:     formatTime(in.At),
	}
	if in.Progress != nil {
		progress := toProgressOut(*in.Progress)
		out.Progress = &progress
	}
	return out
}

func toSortedRecordOutList(in []manager.DownloadRecord) []downloadRecordOut {
	out := make([]downloadRecordOut, 0, len(in))
	for _, rec := range in {
		out = append(out, toRecordOut(rec))
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt == out[j].CreatedAt {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt < out[j].CreatedAt
	})

	return out
}
