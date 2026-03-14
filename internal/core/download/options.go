package download

import "time"

type Options struct {
	Workers            int
	MinChunkSize       int64
	MaxChunkSize       int64
	Timeout            time.Duration
	MaxRetries         int
	BaseBackoff        time.Duration
	MaxBackoff         time.Duration
	UserAgent          string
	ExpectedSHA256Hex  string
	NoResume           bool
	ProgressInterval   time.Duration
	ForceSingle        bool
	RequireAcceptRange bool
}

func DefaultOptions() Options {
	return Options{
		Workers:            6,
		MinChunkSize:       2 * 1024 * 1024,
		MaxChunkSize:       16 * 1024 * 1024,
		Timeout:            30 * time.Second,
		MaxRetries:         10,
		BaseBackoff:        500 * time.Millisecond,
		MaxBackoff:         20 * time.Second,
		UserAgent:          "dlmgr/2.0",
		ProgressInterval:   500 * time.Millisecond,
		RequireAcceptRange: false,
	}
}

func mergeOptions(a, b Options) Options {
	if b.Workers != 0 {
		a.Workers = b.Workers
	}
	if b.MinChunkSize != 0 {
		a.MinChunkSize = b.MinChunkSize
	}
	if b.MaxChunkSize != 0 {
		a.MaxChunkSize = b.MaxChunkSize
	}
	if b.Timeout != 0 {
		a.Timeout = b.Timeout
	}
	if b.MaxRetries != 0 {
		a.MaxRetries = b.MaxRetries
	}
	if b.BaseBackoff != 0 {
		a.BaseBackoff = b.BaseBackoff
	}
	if b.MaxBackoff != 0 {
		a.MaxBackoff = b.MaxBackoff
	}
	if b.UserAgent != "" {
		a.UserAgent = b.UserAgent
	}
	if b.ExpectedSHA256Hex != "" {
		a.ExpectedSHA256Hex = b.ExpectedSHA256Hex
	}
	if b.NoResume {
		a.NoResume = true
	}
	if b.ProgressInterval != 0 {
		a.ProgressInterval = b.ProgressInterval
	}
	if b.ForceSingle {
		a.ForceSingle = true
	}
	if b.RequireAcceptRange {
		a.RequireAcceptRange = true
	}
	return a
}
