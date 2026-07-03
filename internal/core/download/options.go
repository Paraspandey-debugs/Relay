package download

import "time"

// Options controls the behaviour of DownloadFileV2.
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
	HandleRateLimits   bool

	// RateLimitBps throttles download throughput (bytes/sec). 0 = unlimited.
	RateLimitBps int64

	// Mirrors is a list of alternative URLs that serve the same file.
	// Workers round-robin across the primary URL and mirrors.
	Mirrors []string

	// StallTimeout is how long a worker may go without receiving any bytes
	// before the health monitor cancels it and requeues its remaining work.
	// Default: 10s
	StallTimeout time.Duration

	// SlowWorkerGracePeriod is how long a new task is immune from slow-worker
	// cancellation. Default: 5s.
	SlowWorkerGracePeriod time.Duration

	// SlowWorkerThreshold is the fraction of mean speed below which a worker
	// is considered slow (e.g. 0.1 = slower than 10% of mean). 0 = disabled.
	SlowWorkerThreshold float64
}

func DefaultOptions() Options {
	return Options{
		Workers:               4,   // conservative default; increase if server supports it
		MinChunkSize:          2 * 1024 * 1024,  // 2 MB
		MaxChunkSize:          16 * 1024 * 1024, // 16 MB
		Timeout:               30 * time.Second,
		MaxRetries:            5,
		BaseBackoff:           500 * time.Millisecond,
		MaxBackoff:            60 * time.Second,
		UserAgent:             "Relay/2.0",
		ProgressInterval:      250 * time.Millisecond,
		RequireAcceptRange:    false,
		HandleRateLimits:      true,
		StallTimeout:          10 * time.Second,
		SlowWorkerGracePeriod: 5 * time.Second,
		SlowWorkerThreshold:   0.1, // cancel if < 10% of mean speed
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
	if b.RateLimitBps != 0 {
		a.RateLimitBps = b.RateLimitBps
	}
	if len(b.Mirrors) > 0 {
		a.Mirrors = b.Mirrors
	}
	if b.StallTimeout != 0 {
		a.StallTimeout = b.StallTimeout
	}
	if b.SlowWorkerGracePeriod != 0 {
		a.SlowWorkerGracePeriod = b.SlowWorkerGracePeriod
	}
	if b.SlowWorkerThreshold != 0 {
		a.SlowWorkerThreshold = b.SlowWorkerThreshold
	}
	// HandleRateLimits defaults to true; copy explicitly.
	a.HandleRateLimits = b.HandleRateLimits
	return a
}
