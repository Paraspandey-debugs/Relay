package main

import (
	"context"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("usage : download url filename")
		os.Exit(1)
	}
	url := os.Args[1]
	filename := os.Args[2]

	err := DownloadFile(context.Background(), url, filename, nil)
	if err != nil {
		panic(err)
	}
}

const maxRetries = 10
const baseTime = 1 * time.Second
const maxBackoff = 32 * time.Second

type ProgressMsg struct {
	Downloaded int64
	Total      int64
}

func DownloadFile(ctx context.Context, url string, filepath string, progressChan chan<- ProgressMsg) error {
	client := &http.Client{Timeout: 15 * time.Second}

	// HEAD request to check for range support
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return err
	}
	headResp, err := client.Do(req)
	if err != nil {
		// Fall back to single download
		return downloadSingle(ctx, url, filepath, client, progressChan, 0)
	}
	defer headResp.Body.Close()

	if headResp.StatusCode != 200 {
		return fmt.Errorf("head request failed: %d", headResp.StatusCode)
	}

	contentLength := headResp.ContentLength
	acceptRanges := headResp.Header.Get("Accept-Ranges")

	if contentLength <= 0 || acceptRanges != "bytes" {
		// Fall back to single download
		return downloadSingle(ctx, url, filepath, client, progressChan, contentLength)
	}

	// Create file
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Pre-allocate file size if possible
	if err := file.Truncate(contentLength); err != nil {
		// Ignore error, continue
	}

	var totalWritten atomic.Int64
	numSegments := 1
	if contentLength > 0 {
		segmentSize := int64(10 * 1024 * 1024) // 10MB per segment
		numSegments = int((contentLength + segmentSize - 1) / segmentSize)
		if numSegments > 8 {
			numSegments = 8
		}
		if numSegments < 1 {
			numSegments = 1
		}
	}
	segmentSize := contentLength / int64(numSegments)
	var wg sync.WaitGroup
	errChan := make(chan error, numSegments)

	for i := 0; i < numSegments; i++ {
		start := int64(i) * segmentSize
		end := start + segmentSize - 1
		if i == numSegments-1 {
			end = contentLength - 1
		}
		wg.Add(1)
		go func(start, end int64) {
			defer wg.Done()
			err := downloadSegment(ctx, client, url, file, start, end, &totalWritten, contentLength, progressChan)
			if err != nil {
				select {
				case errChan <- err:
				default:
				}
			}
		}(start, end)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}

func downloadSegment(ctx context.Context, client *http.Client, url string, file *os.File, start, end int64, totalWritten *atomic.Int64, total int64, progressChan chan<- ProgressMsg) error {
	currentStart := start

	for i := 0; i < maxRetries; i++ {
		if currentStart > end {
			return nil
		}

		rangeHeader := fmt.Sprintf("bytes=%d-%d", currentStart, end)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Range", rangeHeader)

		resp, err := client.Do(req)
		if err != nil {
			if i == maxRetries-1 {
				return err
			}
			goto RETRY
		}

		if resp.StatusCode == 206 {
			writer := &passThroughWriter{
				Writer:       &offsetWriter{file: file, offset: currentStart},
				TotalWritten: totalWritten,
				Total:        total,
				ProgressChan: progressChan,
			}
			buf := make([]byte, 32*1024)
			written, err := io.CopyBuffer(writer, resp.Body, buf)
			resp.Body.Close()

			currentStart += written

			if err != nil {
				if i == maxRetries-1 {
					return err
				}
				goto RETRY
			}
			return nil
		} else {
			resp.Body.Close()
			if i == maxRetries-1 {
				return fmt.Errorf("segment download failed: %d", resp.StatusCode)
			}
		}

	RETRY:
		backoff := time.Duration(math.Min(
			float64(baseTime)*math.Pow(2, float64(i)),
			float64(maxBackoff),
		))
		jitter := time.Duration(rand.Float64() * float64(backoff) * 0.5)
		select {
		case <-time.After(backoff + jitter):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return fmt.Errorf("max retries exceeded for segment")
}

type offsetWriter struct {
	file   *os.File
	offset int64
}

func (w *offsetWriter) Write(p []byte) (n int, err error) {
	n, err = w.file.WriteAt(p, w.offset)
	w.offset += int64(n)
	return
}

type passThroughWriter struct {
	Writer       io.Writer
	TotalWritten *atomic.Int64
	Total        int64
	ProgressChan chan<- ProgressMsg
}

func (w *passThroughWriter) Write(p []byte) (n int, err error) {
	n, err = w.Writer.Write(p)
	(*w.TotalWritten).Add(int64(n))
	if w.ProgressChan != nil {
		select {
		case w.ProgressChan <- ProgressMsg{Downloaded: (*w.TotalWritten).Load(), Total: w.Total}:
		default:
		}
	}
	return
}

func downloadSingle(ctx context.Context, url, filepath string, client *http.Client, progressChan chan<- ProgressMsg, knownTotal int64) error {
	var existingSize int64
	if fi, err := os.Stat(filepath); err == nil {
		existingSize = fi.Size()
	}

	var out *os.File
	var err error
	if existingSize > 0 {
		out, err = os.OpenFile(filepath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	} else {
		out, err = os.Create(filepath)
	}
	if err != nil {
		return err
	}
	defer out.Close()

	var resp *http.Response
	for i := 0; i < maxRetries; i++ {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return err
		}
		if existingSize > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
		}

		resp, err = client.Do(req)
		if err != nil {
			if i == maxRetries-1 {
				return err
			}
			goto RETRY
		}

		if resp.StatusCode >= 500 || resp.StatusCode == 408 {
			resp.Body.Close()
			if i == maxRetries-1 {
				return fmt.Errorf("server error: %d", resp.StatusCode)
			}
		} else if resp.StatusCode >= 200 && resp.StatusCode < 300 || resp.StatusCode == 206 {
			break
		} else {
			resp.Body.Close()
			if i == maxRetries-1 {
				return fmt.Errorf("request failed: %d", resp.StatusCode)
			}
		}

	RETRY:
		backoff := time.Duration(math.Min(
			float64(baseTime)*math.Pow(2, float64(i)),
			float64(maxBackoff),
		))
		jitter := time.Duration(rand.Float64() * float64(backoff) * 0.5)
		select {
		case <-time.After(backoff + jitter):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	total := resp.ContentLength
	if knownTotal > 0 {
		total = knownTotal
	}
	if existingSize > 0 && resp.StatusCode == 206 {
		total += existingSize
	}

	var totalWritten atomic.Int64
	totalWritten.Store(existingSize)
	writer := &passThroughWriter{
		Writer:       out,
		TotalWritten: &totalWritten,
		Total:        total,
		ProgressChan: progressChan,
	}
	buf := make([]byte, 32*1024)
	_, err = io.CopyBuffer(writer, resp.Body, buf)
	return err
}
