package download

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Paraspandey-debugs/Relay/internal/core/checksum"
	corehttp "github.com/Paraspandey-debugs/Relay/internal/core/httpclient"
)

const copyBufSize = 512 * 1024 // 512 KB

// DownloadFileV2 downloads url to dstPath using multi-connection parallelism.
func DownloadFileV2(ctx context.Context, url, dstPath string, opt *Options, progress chan<- ProgressMsg) error {
	cfg := DefaultOptions()
	if opt != nil {
		cfg = mergeOptions(cfg, *opt)
	}

	// ── Transport pool (shared across all workers) ──
	client, transport := newHTTPClient(cfg)
	defer DefaultTransportPool.ReleaseTransport(transport)

	partPath := dstPath + ".part"
	statePath := dstPath + ".part.state.json"

	log.Printf("[Download] Starting: %s -> %s (Workers: %d)", url, dstPath, cfg.Workers)

	meta, err := corehttp.Probe(ctx, client, url, cfg.UserAgent, cfg.MaxRetries, cfg.HandleRateLimits)
	if err != nil {
		return fmt.Errorf("probe failed: %w", err)
	}

	canMulti := meta.Total > 0 && meta.AcceptRanges
	if cfg.RequireAcceptRange && !meta.AcceptRanges {
		return fmt.Errorf("server does not support byte ranges")
	}
	if cfg.ForceSingle || !canMulti {
		if err := downloadSingleV2(ctx, client, url, dstPath, partPath, statePath, meta, cfg, progress); err != nil {
			return err
		}
		return verifyAndFinalize(client, dstPath, partPath, statePath, cfg.ExpectedSHA256Hex)
	}

	st, err := loadOrInitState(url, dstPath, partPath, statePath, meta, cfg)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(partPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_ = f.Truncate(st.Total)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var totalWritten atomic.Int64
	totalWritten.Store(computeDone(st.Segments))

	var retries atomic.Int64
	progressDone := make(chan struct{})
	go emitProgress(ctx, &totalWritten, st.Total, int32(cfg.Workers), &retries, cfg.ProgressInterval, progress, progressDone)

	// ── Build mirror list for workers ──
	mirrorURLs, mirrorHosts := buildWorkerMirrors(url, cfg.Mirrors)

	// ── Pre-warm connections ──
	prewarmConnections(ctx, client, url, cfg.UserAgent, cfg.Workers)

	// ── Task queue ──
	queue := NewTaskQueue()
	for _, seg := range st.Segments {
		if !seg.Done && seg.Next <= seg.End {
			queue.Push(Task{
				Offset: seg.Next,
				Length: seg.End - seg.Next + 1,
			})
		}
	}

	// ── Host rate limiter (shared across workers) ──
	hostLimiter := newHostRateLimiter()

	// ── Per-download byte rate limiter ──
	limiter := newRateLimiter(cfg)

	balancer := NewBalancer()
	// Balancer: work-stealing + hedging
	go balancer.Run(ctx, queue)
	// Health monitor: stall + slow-worker detection
	go balancer.RunHealthMonitor(ctx, queue, cfg)

	wc := &workerCtx{
		url:          url,
		mirrors:      mirrorURLs,
		mirrorHosts:  mirrorHosts,
		file:         f,
		queue:        queue,
		balancer:     balancer,
		client:       client,
		hostLimiter:  hostLimiter,
		rateLimiter:  limiter,
		totalSize:    st.Total,
		totalWritten: &totalWritten,
		cfg:          cfg,
	}

	var wg sync.WaitGroup
	errCh := make(chan error, cfg.Workers)

	for w := 0; w < cfg.Workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			if err := worker(ctx, workerID, wc); err != nil && err != context.Canceled {
				select {
				case errCh <- err:
				default:
				}
				cancel()
			}
		}(w)
	}

	// ── State flush goroutine ──
	stateFlushStop := make(chan struct{})
	stateFlushDone := make(chan struct{})
	go func() {
		defer close(stateFlushDone)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stateFlushStop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				segs := balancer.SnapshotTasks(queue)
				st.Segments = segs
				_ = writeState(statePath, st)
			}
		}
	}()

	// ── Completion monitor (50 ms tick — matches Surge) ──
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Primary fast path: all bytes accounted for.
				if totalWritten.Load() >= st.Total {
					queue.Close()
					cancel()
					return
				}
				// Secondary path: queue drained and all workers idle.
				if queue.Len() == 0 && queue.IdleWorkers() == int32(cfg.Workers) {
					if totalWritten.Load() >= st.Total {
						queue.Close()
						cancel()
						return
					}
				}
			}
		}
	}()

	go func() {
		wg.Wait()
		close(errCh)
	}()

	var finalErr error
	for e := range errCh {
		if finalErr == nil {
			finalErr = e
		}
	}

	close(stateFlushStop)
	<-stateFlushDone

	// Final state flush.
	st.Segments = balancer.SnapshotTasks(queue)
	_ = writeState(statePath, st)

	close(progressDone)

	if finalErr != nil {
		return finalErr
	}
	if totalWritten.Load() != st.Total {
		return fmt.Errorf("incomplete: got %d want %d", totalWritten.Load(), st.Total)
	}
	return verifyAndFinalize(client, dstPath, partPath, statePath, cfg.ExpectedSHA256Hex)
}

// ── State helpers ──

func loadOrInitState(url, dst, part, statePath string, meta corehttp.ProbeMeta, cfg Options) (*downloadState, error) {
	if cfg.NoResume {
		_ = os.Remove(statePath)
		_ = os.Remove(part)
	}
	if st, err := readState(statePath); err == nil {
		if st.URL == url && st.Total == meta.Total &&
			(st.ETag == "" || meta.ETag == "" || st.ETag == meta.ETag) &&
			(st.LastModified == "" || meta.LastModified == "" || st.LastModified == meta.LastModified) {
			return st, nil
		}
		_ = os.Remove(statePath)
		_ = os.Remove(part)
	}

	segs := buildSegments(meta.Total, cfg.Workers, cfg.MinChunkSize, cfg.MaxChunkSize)
	st := &downloadState{
		URL:          url,
		FinalPath:    dst,
		PartPath:     part,
		Total:        meta.Total,
		ETag:         meta.ETag,
		LastModified: meta.LastModified,
		Segments:     segs,
		UpdatedAt:    time.Now(),
	}
	return st, writeState(statePath, st)
}

func buildSegments(total int64, workers int, minChunk, maxChunk int64) []segmentState {
	if total <= 0 {
		return []segmentState{{Start: 0, End: -1, Next: 0, Done: true}}
	}
	target := int64(workers * 4)
	chunk := int64(math.Ceil(float64(total) / float64(target)))
	if chunk < minChunk {
		chunk = minChunk
	}
	if chunk > maxChunk {
		chunk = maxChunk
	}
	var segs []segmentState
	for start := int64(0); start < total; start += chunk {
		end := start + chunk - 1
		if end >= total {
			end = total - 1
		}
		segs = append(segs, segmentState{Start: start, End: end, Next: start, Done: false})
	}
	return segs
}

// verifyAndFinalize fsyncs the part file, verifies checksum if required, then
// renames it to the final path. §8: fsync before rename prevents data loss.
func verifyAndFinalize(client *http.Client, finalPath, partPath, statePath, expectedSHA string) error {
	// fsync: flush OS page cache → disk before rename.
	f, err := os.OpenFile(partPath, os.O_RDWR, 0)
	if err == nil {
		_ = f.Sync()
		_ = f.Close()
	}

	if expectedSHA != "" {
		sum, ok, err := checksum.MatchesSHA256(partPath, expectedSHA)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("checksum mismatch: got %s", sum)
		}
	}
	if err := os.Rename(partPath, finalPath); err != nil {
		return err
	}
	_ = os.Remove(statePath)
	return nil
}

// ── Single-stream fallback ──

func downloadSingleV2(
	ctx context.Context,
	client *http.Client,
	url, finalPath, partPath, statePath string,
	meta corehttp.ProbeMeta,
	cfg Options,
	progress chan<- ProgressMsg,
) error {
	var existing int64
	if !cfg.NoResume {
		if fi, err := os.Stat(partPath); err == nil {
			existing = fi.Size()
		}
	} else {
		_ = os.Remove(partPath)
		_ = os.Remove(statePath)
	}

	out, err := os.OpenFile(partPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	var resp *http.Response

	for i := 0; i < cfg.MaxRetries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		req.Header.Set("User-Agent", cfg.UserAgent)
		if existing > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existing))
			if meta.ETag != "" {
				req.Header.Set("If-Range", meta.ETag)
			}
		}

		resp, err = client.Do(req)
		if err != nil {
			if i == cfg.MaxRetries-1 {
				return err
			}
			if bErr := sleepBackoff(ctx, cfg.BaseBackoff, cfg.MaxBackoff, i, 0); bErr != nil {
				return bErr
			}
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			ra, _ := ParseRetryAfter(resp, time.Now())
			resp.Body.Close()
			if i == cfg.MaxRetries-1 {
				return fmt.Errorf("download failed: rate limited")
			}
			retrySec := int(ra.Seconds())
			if cfg.HandleRateLimits && retrySec == 0 {
				retrySec = 5 * (1 << i)
			}
			if bErr := sleepBackoff(ctx, cfg.BaseBackoff, cfg.MaxBackoff, i, retrySec); bErr != nil {
				return bErr
			}
			continue
		}

		if resp.StatusCode >= 400 && resp.StatusCode != http.StatusRequestedRangeNotSatisfiable {
			resp.Body.Close()
			if i == cfg.MaxRetries-1 {
				return fmt.Errorf("download failed with status %d", resp.StatusCode)
			}
			ra, _ := ParseRetryAfter(resp, time.Now())
			if bErr := sleepBackoff(ctx, cfg.BaseBackoff, cfg.MaxBackoff, i, int(ra.Seconds())); bErr != nil {
				return bErr
			}
			continue
		}

		break
	}

	if resp == nil {
		return fmt.Errorf("max retries exceeded")
	}
	defer resp.Body.Close()

	if existing > 0 && resp.StatusCode == http.StatusOK {
		out.Close()
		_ = os.Remove(partPath)
		noResumeCfg := cfg
		noResumeCfg.NoResume = true
		noResumeCfg.ForceSingle = true
		return downloadSingleV2(ctx, client, url, finalPath, partPath, statePath, meta, noResumeCfg, progress)
	}

	if !(resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent) {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	total := meta.Total
	if total <= 0 {
		total = resp.ContentLength
		if resp.StatusCode == http.StatusPartialContent && existing > 0 && total > 0 {
			total += existing
		}
	}

	var written atomic.Int64
	written.Store(existing)
	done := make(chan struct{})
	var retriesA atomic.Int64
	go emitProgress(ctx, &written, total, 1, &retriesA, cfg.ProgressInterval, progress, done)

	bufPtr := bufPool.Get().(*[]byte)
	defer bufPool.Put(bufPtr)
	buf := *bufPtr

	pw := &passThroughWriter{
		Writer: out,
		OnWrite: func(n int) {
			written.Add(int64(n))
		},
	}
	_, err = io.CopyBuffer(pw, resp.Body, buf)
	close(done)
	return err
}
