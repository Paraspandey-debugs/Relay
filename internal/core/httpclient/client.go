package httpclient

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type ProbeMeta struct {
	Total        int64
	AcceptRanges bool
	ETag         string
	LastModified string
}

func New(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

func Probe(ctx context.Context, client *http.Client, url, userAgent string) (ProbeMeta, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode >= 400 {
		if resp != nil {
			resp.Body.Close()
		}

		req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		req2.Header.Set("Range", "bytes=0-0")
		req2.Header.Set("User-Agent", userAgent)
		r2, e2 := client.Do(req2)
		if e2 != nil {
			return ProbeMeta{}, e2
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
	defer resp.Body.Close()

	return ProbeMeta{
		Total:        resp.ContentLength,
		AcceptRanges: strings.EqualFold(strings.TrimSpace(resp.Header.Get("Accept-Ranges")), "bytes"),
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
	}, nil
}

func RetryAfterSeconds(resp *http.Response) int {
	ra := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if ra == "" {
		return 0
	}
	n, err := strconv.Atoi(ra)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func parseContentRangeTotal(contentRange string) (int64, bool) {
	parts := strings.Split(contentRange, "/")
	if len(parts) != 2 {
		return 0, false
	}
	t, err := strconv.ParseInt(parts[1], 10, 64)
	return t, err == nil
}
