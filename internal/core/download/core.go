package download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type probeMeta struct {
	Total        int64
	AcceptRanges bool
	ETag         string
	LastModified string
}

func DownloadFileV2(ctx context.Context, url, dstPath string, opt *Options, progress chan<- ProgressMsg) error {
	cfg := DefaultOptions()
	if opt != nil {
		cfg = mergeOptions(cfg, *opt)
	}
	if cfg.Workers < 1 {
		cfg.Workers = 1
	}
	if cfg.MinChunkSize <= 0 {
		cfg.MinChunkSize = 2 * 1024 * 1024
	}
	if cfg.MaxChunkSize < cfg.MinChunkSize {
		cfg.MaxChunkSize = cfg.MinChunkSize
	}

	client := &http.Client{Timeout: cfg.Timeout}
	partPath := dstPath + ".part"
	statePath := dstPath + ".part.state.json"

	meta, err := probe(ctx, client, url, cfg.UserAgent)
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
		return verifyAndFinalize(dstPath, partPath, statePath, cfg.ExpectedSHA256Hex)
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

	segCh := make(chan int, len(st.Segments))
	for i := range st.Segments {
		if !st.Segments[i].Done {
			segCh <- i
		}
	}
	close(segCh)

	var mu sync.Mutex
	var wg sync.WaitGroup
	errCh := make(chan error, 1)
	sendErr := func(e error) {
		select {
		case errCh <- e:
		default:
		}
	}

	worker := func() {
		defer wg.Done()
		for idx := range segCh {
			select {
			case <-ctx.Done():
				return
			default:
			}
			seg := &st.Segments[idx]
			if seg.Done || seg.Next > seg.End {
				continue
			}
			if err := fetchSegment(ctx, client, f, st, seg, cfg, &totalWritten, &retries, &mu, statePath); err != nil {
				sendErr(err)
				cancel()
				return
			}
		}
	}

	for i := 0; i < cfg.Workers; i++ {
		wg.Add(1)
		go worker()
	}
	wg.Wait()
	close(progressDone)

	select {
	case e := <-errCh:
		return e
	default:
	}

	if totalWritten.Load() != st.Total {
		return fmt.Errorf("incomplete: got %d want %d", totalWritten.Load(), st.Total)
	}
	return verifyAndFinalize(dstPath, partPath, statePath, cfg.ExpectedSHA256Hex)
}

func probe(ctx context.Context, client *http.Client, url, ua string) (probeMeta, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	req.Header.Set("User-Agent", ua)
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode >= 400 {
		if resp != nil {
			resp.Body.Close()
		}
		req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		req2.Header.Set("Range", "bytes=0-0")
		req2.Header.Set("User-Agent", ua)
		r2, e2 := client.Do(req2)
		if e2 != nil {
			return probeMeta{}, e2
		}
		defer r2.Body.Close()
		m := probeMeta{
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

	return probeMeta{
		Total:        resp.ContentLength,
		AcceptRanges: strings.EqualFold(strings.TrimSpace(resp.Header.Get("Accept-Ranges")), "bytes"),
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
	}, nil
}

func loadOrInitState(url, dst, part, statePath string, meta probeMeta, cfg Options) (*downloadState, error) {
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
	target := int64(workers * 2)
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
		segs = append(segs, segmentState{Start: start, End: end, Next: start})
	}
	return segs
}

func fetchSegment(
	ctx context.Context,
	client *http.Client,
	file *os.File,
	st *downloadState,
	seg *segmentState,
	cfg Options,
	totalWritten *atomic.Int64,
	retries *atomic.Int64,
	mu *sync.Mutex,
	statePath string,
) error {
	for i := 0; i < cfg.MaxRetries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if seg.Next > seg.End {
			mu.Lock()
			seg.Done = true
			errState := writeState(statePath, st)
			mu.Unlock()
			if errState != nil {
				return fmt.Errorf("persist state: %w", errState)
			}
			return nil
		}

		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, st.URL, nil)
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", seg.Next, seg.End))
		req.Header.Set("User-Agent", cfg.UserAgent)
		if st.ETag != "" {
			req.Header.Set("If-Range", st.ETag)
		} else if st.LastModified != "" {
			req.Header.Set("If-Range", st.LastModified)
		}

		resp, err := client.Do(req)
		if err != nil {
			retries.Add(1)
			if i == cfg.MaxRetries-1 {
				return err
			}
			if err := sleepBackoff(ctx, cfg.BaseBackoff, cfg.MaxBackoff, i, 0); err != nil {
				return err
			}
			continue
		}

		if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
			resp.Body.Close()
			mu.Lock()
			seg.Done = true
			errState := writeState(statePath, st)
			mu.Unlock()
			if errState != nil {
				return fmt.Errorf("persist state after 416: %w", errState)
			}
			return nil
		}

		if resp.StatusCode != http.StatusPartialContent {
			resp.Body.Close()
			retries.Add(1)
			if i == cfg.MaxRetries-1 {
				return fmt.Errorf("expected 206 for range, got %d", resp.StatusCode)
			}
			if err := sleepBackoff(ctx, cfg.BaseBackoff, cfg.MaxBackoff, i, retryAfterSeconds(resp)); err != nil {
				return err
			}
			continue
		}

		w := &offsetWriter{file: file, offset: seg.Next}
		n, cpErr := io.CopyBuffer(w, resp.Body, make([]byte, 64*1024))
		resp.Body.Close()
		totalWritten.Add(n)

		mu.Lock()
		seg.Next += n
		if seg.Next > seg.End {
			seg.Done = true
		}
		errState := writeState(statePath, st)
		mu.Unlock()
		if errState != nil {
			return fmt.Errorf("persist state: %w", errState)
		}

		if cpErr != nil {
			retries.Add(1)
			if i == cfg.MaxRetries-1 {
				return cpErr
			}
			if err := sleepBackoff(ctx, cfg.BaseBackoff, cfg.MaxBackoff, i, 0); err != nil {
				return err
			}
			continue
		}
		return nil
	}
	return errors.New("max retries exceeded")
}

func downloadSingleV2(
	ctx context.Context,
	client *http.Client,
	url, finalPath, partPath, statePath string,
	meta probeMeta,
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

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("User-Agent", cfg.UserAgent)
	if existing > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existing))
		if meta.ETag != "" {
			req.Header.Set("If-Range", meta.ETag)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if existing > 0 && resp.StatusCode == http.StatusOK {
		out.Close()
		_ = os.Remove(partPath)
		return downloadSingleV2(ctx, client, url, finalPath, partPath, statePath, meta, Options{
			Workers:            cfg.Workers,
			MinChunkSize:       cfg.MinChunkSize,
			MaxChunkSize:       cfg.MaxChunkSize,
			Timeout:            cfg.Timeout,
			MaxRetries:         cfg.MaxRetries,
			BaseBackoff:        cfg.BaseBackoff,
			MaxBackoff:         cfg.MaxBackoff,
			UserAgent:          cfg.UserAgent,
			ExpectedSHA256Hex:  cfg.ExpectedSHA256Hex,
			NoResume:           true,
			ProgressInterval:   cfg.ProgressInterval,
			ForceSingle:        true,
			RequireAcceptRange: cfg.RequireAcceptRange,
		}, progress)
	}

	if !(resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent) {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	total := resp.ContentLength
	if meta.Total > 0 {
		total = meta.Total
	}
	if resp.StatusCode == http.StatusPartialContent && existing > 0 && total > 0 {
		total += existing
	}

	var written atomic.Int64
	written.Store(existing)
	done := make(chan struct{})
	var retries atomic.Int64
	go emitProgress(ctx, &written, total, 1, &retries, cfg.ProgressInterval, progress, done)

	pw := &passThroughWriter{
		Writer: out,
		OnWrite: func(n int) {
			written.Add(int64(n))
		},
	}
	_, err = io.CopyBuffer(pw, resp.Body, make([]byte, 64*1024))
	close(done)
	return err
}

func verifyAndFinalize(finalPath, partPath, statePath, expectedSHA string) error {
	if expectedSHA != "" {
		sum, err := fileSHA256(partPath)
		if err != nil {
			return err
		}
		if !strings.EqualFold(sum, strings.TrimSpace(expectedSHA)) {
			return fmt.Errorf("checksum mismatch: got %s", sum)
		}
	}
	if err := os.Rename(partPath, finalPath); err != nil {
		return err
	}
	_ = os.Remove(statePath)
	return nil
}
