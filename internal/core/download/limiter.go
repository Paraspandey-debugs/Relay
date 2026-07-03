package download

import (
	"context"
	"sync"

	"golang.org/x/time/rate"
)

// ByteLimiter throttles the number of bytes that may be written per second.
type ByteLimiter interface {
	// WaitN blocks until n bytes may be consumed or ctx is done.
	WaitN(ctx context.Context, n int64) error
}

// TokenBucketLimiter wraps golang.org/x/time/rate.Limiter and implements
// ByteLimiter. A rate of 0 means unlimited (WaitN is a no-op).
type TokenBucketLimiter struct {
	mu      sync.RWMutex
	limiter *rate.Limiter
	bps     int64
}

// NewTokenBucketLimiter creates a limiter at bps bytes/sec.
// bps = 0 disables throttling.
func NewTokenBucketLimiter(bps int64) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		limiter: makeLimiter(bps),
		bps:     bps,
	}
}

func makeLimiter(bps int64) *rate.Limiter {
	if bps <= 0 {
		return rate.NewLimiter(rate.Inf, 0)
	}
	// Burst = 1 second worth of bytes so the first write is never blocked.
	return rate.NewLimiter(rate.Limit(bps), int(bps))
}

// SetRate updates the token bucket rate at runtime (thread-safe).
func (l *TokenBucketLimiter) SetRate(bps int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.bps = bps
	l.limiter = makeLimiter(bps)
}

// WaitN blocks until n bytes may be consumed.
func (l *TokenBucketLimiter) WaitN(ctx context.Context, n int64) error {
	l.mu.RLock()
	lim := l.limiter
	l.mu.RUnlock()

	if lim.Limit() == rate.Inf {
		return ctx.Err() // unlimited — check cancellation only
	}
	// rate.Limiter.WaitN accepts int, so split if n > MaxInt.
	const maxChunk = 1<<30 // 1 GiB — safely fits in int on all platforms
	for n > 0 {
		chunk := n
		if chunk > maxChunk {
			chunk = maxChunk
		}
		if err := lim.WaitN(ctx, int(chunk)); err != nil {
			return err
		}
		n -= chunk
	}
	return nil
}

// MultiLimiter combines a global and a per-download limiter; both must grant
// tokens before a write proceeds (mirrors Surge's engine/multi_limiter.go).
type MultiLimiter struct {
	limiters []ByteLimiter
}

// NewMultiLimiter creates a combined limiter from the provided limiters.
// nil entries are silently skipped.
func NewMultiLimiter(limiters ...ByteLimiter) *MultiLimiter {
	var active []ByteLimiter
	for _, l := range limiters {
		if l != nil {
			active = append(active, l)
		}
	}
	return &MultiLimiter{limiters: active}
}

// WaitN blocks until all component limiters have granted n bytes.
func (m *MultiLimiter) WaitN(ctx context.Context, n int64) error {
	for _, l := range m.limiters {
		if err := l.WaitN(ctx, n); err != nil {
			return err
		}
	}
	return nil
}
