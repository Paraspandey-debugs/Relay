package download

import (
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// hostPenalty records the back-off state for a host after a 429 response.
type hostPenalty struct {
	until       time.Time
	consecutive int
	lastHit     time.Time
}

// HostRateLimiter tracks per-host 429 penalties and routes workers to the
// least-blocked mirror — mirroring Surge's engine/ratelimit.go logic.
type HostRateLimiter struct {
	mu    sync.Mutex
	hosts map[string]*hostPenalty
}

// Back-off parameters (match Surge defaults).
const (
	rlBaseBackoff    = 500 * time.Millisecond
	rlMinBackoff     = 1 * time.Second
	rlMaxBackoff     = 60 * time.Second
	rlJitter         = 0.2 // ±20% jitter fraction
	rlPenaltyDecay   = 5 * time.Minute
)

func newHostRateLimiter() *HostRateLimiter {
	return &HostRateLimiter{hosts: make(map[string]*hostPenalty)}
}

// Penalize records a 429 for host and returns when it will be unblocked.
// explicit=true means the server sent an explicit Retry-After value.
func (h *HostRateLimiter) Penalize(host string, retryAfter time.Duration, explicit bool, now time.Time) time.Time {
	h.mu.Lock()
	defer h.mu.Unlock()

	p, ok := h.hosts[host]
	if !ok {
		p = &hostPenalty{}
		h.hosts[host] = p
	}

	// Reset consecutive counter if enough time has passed.
	if now.Sub(p.lastHit) > rlPenaltyDecay {
		p.consecutive = 0
	}
	p.consecutive++
	p.lastHit = now

	var d time.Duration
	if explicit && retryAfter > 0 {
		d = retryAfter
	} else {
		d = rlBaseBackoff * time.Duration(int64(1)<<(p.consecutive-1))
	}
	if d < rlMinBackoff {
		d = rlMinBackoff
	}
	if d > rlMaxBackoff {
		d = rlMaxBackoff
	}

	// Add ±rlJitter jitter.
	jitterRange := int64(float64(d) * rlJitter)
	if jitterRange > 0 {
		delta := rand.Int63n(2*jitterRange) - jitterRange
		d += time.Duration(delta)
	}
	if d < rlMinBackoff {
		d = rlMinBackoff
	}
	if d > rlMaxBackoff {
		d = rlMaxBackoff
	}

	p.until = now.Add(d)
	h.cleanupLocked()
	return p.until
}

// RecordSuccess resets the penalty for host (called on a successful response).
func (h *HostRateLimiter) RecordSuccess(host string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if p, ok := h.hosts[host]; ok {
		p.consecutive = 0
		p.until = time.Time{}
	}
}

// PickMirror selects the best mirror starting from startIdx.
// Returns (idx, 0) if a free mirror is available, or (idx, wait) if all are
// penalised and the caller should wait before using the returned index.
func (h *HostRateLimiter) PickMirror(hosts []string, startIdx int, now time.Time) (int, time.Duration) {
	if len(hosts) == 0 {
		return 0, 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	n := len(hosts)
	firstFree := -1
	earliestIdx := -1
	var earliestDeadline time.Time

	for i := 0; i < n; i++ {
		idx := (startIdx + i) % n
		p, ok := h.hosts[hosts[idx]]
		if !ok || now.After(p.until) {
			firstFree = idx
			break
		}
		if earliestIdx == -1 || p.until.Before(earliestDeadline) {
			earliestIdx = idx
			earliestDeadline = p.until
		}
	}

	if firstFree >= 0 {
		return firstFree, 0
	}

	wait := earliestDeadline.Sub(now)
	if wait < 0 {
		wait = 0
	}
	return earliestIdx, wait
}

func (h *HostRateLimiter) cleanupLocked() {
	now := time.Now()
	for host, p := range h.hosts {
		if now.After(p.until) && now.Sub(p.lastHit) > rlPenaltyDecay {
			delete(h.hosts, host)
		}
	}
}

// ParseRetryAfter parses a Retry-After header value (seconds integer or HTTP
// date) and returns the duration to wait plus whether it was explicit.
func ParseRetryAfter(resp *http.Response, now time.Time) (time.Duration, bool) {
	header := resp.Header.Get("Retry-After")
	if header == "" {
		return 0, false
	}
	// Try numeric seconds first.
	var secs int
	if _, err := parseIntFast(header, &secs); err == nil {
		return time.Duration(secs) * time.Second, true
	}
	// Try HTTP date.
	t, err := http.ParseTime(header)
	if err != nil {
		return 0, false
	}
	d := t.Sub(now)
	if d < 0 {
		d = 0
	}
	return d, true
}

// MirrorHost extracts the host (authority) from a raw URL for penalty keying.
func MirrorHost(rawurl string) string {
	u, err := url.Parse(rawurl)
	if err != nil {
		return rawurl
	}
	return u.Host
}

// parseIntFast is a minimal int parser to avoid importing strconv in the hot path.
func parseIntFast(s string, out *int) (string, error) {
	if s == "" {
		return s, errEmpty
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return s, errNotInt
		}
		n = n*10 + int(c-'0')
	}
	*out = n
	return s, nil
}

var (
	errEmpty  = mirrorErr("empty")
	errNotInt = mirrorErr("not an integer")
)

type mirrorErr string

func (e mirrorErr) Error() string { return string(e) }
