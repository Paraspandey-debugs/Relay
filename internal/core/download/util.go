package download

import (
	"context"
	"math"
	"math/rand"
	"time"
)

func sleepBackoff(ctx context.Context, base, max time.Duration, attempt int, retryAfterSec int) error {
	if retryAfterSec > 0 {
		// Add ±10% jitter
		jitter := time.Duration(rand.Int63n(int64(float64(retryAfterSec)*0.2))) - time.Duration(float64(retryAfterSec)*0.1)
		sleep := time.Duration(retryAfterSec)*time.Second + jitter
		select {
		case <-time.After(sleep):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	backoff := time.Duration(math.Min(float64(base)*math.Pow(2, float64(attempt)), float64(max)))
	// Add jitter: up to ±20% of backoff
	jitter := time.Duration(rand.Int63n(int64(float64(backoff) * 0.4))) - time.Duration(float64(backoff)*0.2)
	sleep := backoff + jitter
	if sleep < 0 {
		sleep = 0
	}
	
	select {
	case <-time.After(sleep):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
