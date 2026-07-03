package download

import (
	"context"
	"io"
	"log"
	"net/http"
	"time"
)

// prewarmConnections establishes TCP/TLS handshakes before actual downloads start.
func prewarmConnections(ctx context.Context, client *http.Client, url, userAgent string, numRequired int) {
	totalToStart := numRequired
	if totalToStart > 64 {
		totalToStart = 64
	}

	ready := make(chan struct{}, totalToStart)
	pingCtx, cancelPings := context.WithCancel(ctx)
	defer cancelPings()

	for i := 0; i < totalToStart; i++ {
		go func() {
			req, err := http.NewRequestWithContext(pingCtx, http.MethodGet, url, nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", userAgent)
			req.Header.Set("Range", "bytes=0-0")

			resp, err := client.Do(req)
			if err != nil {
				return
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			ready <- struct{}{}
		}()
	}

	completed := 0
	timeout := time.After(3 * time.Second)

	for completed < numRequired {
		select {
		case <-ready:
			completed++
		case <-timeout:
			log.Printf("[Pre-warm] Timed out after %d/%d connections", completed, numRequired)
			return
		case <-ctx.Done():
			return
		}
	}
	log.Printf("[Pre-warm] Complete: %d connections established", completed)
}
