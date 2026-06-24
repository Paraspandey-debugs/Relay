package download

import (
	"context"
	"math"
	"math/rand"
	"time"
)

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
