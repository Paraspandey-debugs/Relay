package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	var h hash.Hash = sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func parseContentRangeTotal(cr string) (int64, bool) {
	parts := strings.Split(cr, "/")
	if len(parts) != 2 {
		return 0, false
	}
	t, err := strconv.ParseInt(parts[1], 10, 64)
	return t, err == nil
}

func retryAfterSeconds(resp *http.Response) int {
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

func sleepBackoff(ctx context.Context, base, max time.Duration, attempt int, retryAfterSec int) error {
	if retryAfterSec > 0 {
		select {
		case <-time.After(time.Duration(retryAfterSec) * time.Second):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	backoff := time.Duration(math.Min(float64(base)*math.Pow(2, float64(attempt)), float64(max)))
	jitter := time.Duration(rand.Float64() * float64(backoff) * 0.5)
	select {
	case <-time.After(backoff + jitter):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
