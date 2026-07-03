package httpclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type ProbeMeta struct {
	Total        int64
	AcceptRanges bool
	ETag         string
	LastModified string
}

type rateLimitTransport struct {
	base      http.RoundTripper
	limiters  map[string]*rate.Limiter
	mu        sync.Mutex
	rateLimit rate.Limit
}

func (t *rateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.URL.Host
	t.mu.Lock()
	lim, ok := t.limiters[host]
	if !ok {
		lim = rate.NewLimiter(t.rateLimit, 1) // burst=1
		t.limiters[host] = lim
	}
	t.mu.Unlock()
	if err := lim.Wait(req.Context()); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(req)
}

// New creates a high-performance HTTP client tuned for parallel downloads.
// The transport is configured to maximize throughput:
//   - Aggressive connection pooling (unlimited per host)
//   - Large read/write buffers
//   - TCP keepalive to avoid connection resets on slow links
//   - HTTP/2 disabled to allow true parallel TCP streams
func New(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		MaxIdleConns:          256,
		MaxIdleConnsPerHost:   64,
		MaxConnsPerHost:       0, // unlimited
		IdleConnTimeout:       120 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 2 * time.Second,
		WriteBufferSize:       256 * 1024, // 256KB write buffer
		ReadBufferSize:        256 * 1024, // 256KB read buffer
		ForceAttemptHTTP2:     false,      // HTTP/1.1 for true parallel streams
		DisableCompression:    true,       // We want raw bytes, no gzip overhead
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
	}

	rt := &rateLimitTransport{
		base:      transport,
		limiters:  make(map[string]*rate.Limiter),
		// 10 requests per second per host by default for Relay
		rateLimit: rate.Every(100 * time.Millisecond), 
	}

	return &http.Client{
		Transport: rt,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}

func Probe(ctx context.Context, client *http.Client, url, userAgent string, maxRetries int, handleRateLimits bool) (ProbeMeta, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return ProbeMeta{}, ctx.Err()
		default:
		}

		req, _ := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
		req.Header.Set("User-Agent", userAgent)
		resp, err := client.Do(req)
		
		var statusCode int
		var retrySec int
		
		if err == nil {
			statusCode = resp.StatusCode
			retrySec = RetryAfterSeconds(resp)
			resp.Body.Close()
		}

		if err != nil || statusCode >= 400 {
			if statusCode == http.StatusTooManyRequests {
				if handleRateLimits && retrySec == 0 {
					retrySec = 5 * (1 << i)
				}
				sleepBackoffProbe(ctx, 500*time.Millisecond, 20*time.Second, i, retrySec)
				lastErr = fmt.Errorf("rate limited (429)")
				continue
			}

			// If HEAD fails, try GET
			req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			req2.Header.Set("Range", "bytes=0-0")
			req2.Header.Set("User-Agent", userAgent)
			r2, e2 := client.Do(req2)
			
			if e2 != nil {
				lastErr = e2
				sleepBackoffProbe(ctx, 500*time.Millisecond, 20*time.Second, i, 0)
				continue
			}
			
			statusCode = r2.StatusCode
			retrySec = RetryAfterSeconds(r2)
			
			if statusCode == http.StatusTooManyRequests {
				r2.Body.Close()
				if handleRateLimits && retrySec == 0 {
					retrySec = 5 * (1 << i)
				}
				sleepBackoffProbe(ctx, 500*time.Millisecond, 20*time.Second, i, retrySec)
				lastErr = fmt.Errorf("rate limited (429)")
				continue
			}
			
			defer r2.Body.Close()
			m := ProbeMeta{
				ETag:         r2.Header.Get("ETag"),
				LastModified: r2.Header.Get("Last-Modified"),
			}
			if r2.StatusCode == http.StatusPartialContent {
				m.AcceptRanges = true
				if total, ok := parseContentRangeTotal(r2.Header.Get("Content-Range")); ok {
					m.Total = total
				}
			} else if r2.StatusCode >= 200 && r2.StatusCode < 300 {
				m.Total = r2.ContentLength
				m.AcceptRanges = strings.EqualFold(strings.TrimSpace(r2.Header.Get("Accept-Ranges")), "bytes")
			}
			io.Copy(io.Discard, r2.Body)
			return m, nil
		}

		return ProbeMeta{
			Total:        resp.ContentLength,
			AcceptRanges: strings.EqualFold(strings.TrimSpace(resp.Header.Get("Accept-Ranges")), "bytes"),
			ETag:         resp.Header.Get("ETag"),
			LastModified: resp.Header.Get("Last-Modified"),
		}, nil
	}
	if lastErr != nil {
		return ProbeMeta{}, fmt.Errorf("probe max retries exceeded: %w", lastErr)
	}
	return ProbeMeta{}, fmt.Errorf("probe max retries exceeded")
}

func sleepBackoffProbe(ctx context.Context, base, max time.Duration, attempt int, retryAfterSec int) {
	if retryAfterSec > 0 {
		select {
		case <-time.After(time.Duration(retryAfterSec) * time.Second):
			return
		case <-ctx.Done():
			return
		}
	}
	// simplified backoff since we don't have math/rand here
	multiplier := float64(uint64(1) << uint(attempt))
	backoff := time.Duration(float64(base) * multiplier)
	if backoff > max {
		backoff = max
	}
	select {
	case <-time.After(backoff):
		return
	case <-ctx.Done():
		return
	}
}

func RetryAfterSeconds(resp *http.Response) int {
	ra := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if ra == "" {
		return 0
	}
	// Try parsing as integer seconds
	if n, err := strconv.Atoi(ra); err == nil && n >= 0 {
		return n
	}
	// Try parsing as HTTP Date
	if t, err := time.Parse(http.TimeFormat, ra); err == nil {
		delta := time.Until(t)
		if delta > 0 {
			return int(delta.Seconds())
		}
	}
	return 0
}

func parseContentRangeTotal(contentRange string) (int64, bool) {
	parts := strings.Split(contentRange, "/")
	if len(parts) != 2 {
		return 0, false
	}
	t, err := strconv.ParseInt(parts[1], 10, 64)
	return t, err == nil
}
