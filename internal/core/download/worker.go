package download

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	corehttp "github.com/Paraspandey-debugs/Relay/internal/core/httpclient"
)

// workerBatchSize is the accumulated byte count before we flush to totalWritten.
const workerBatchSize = 512 * 1024 // 512 KB

// bufPool recycles worker read buffers to reduce GC pressure.
var bufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, copyBufSize)
		return &buf
	},
}

// workerCtx bundles per-download context passed to every worker goroutine.
type workerCtx struct {
	url          string          // primary URL
	mirrors      []string        // all URLs incl. primary (index 0 = primary)
	mirrorHosts  []string        // host portion of each mirror (for penalty keying)
	file         *os.File
	queue        *TaskQueue
	balancer     *Balancer
	client       *http.Client
	hostLimiter  *HostRateLimiter
	rateLimiter  ByteLimiter // may be nil
	totalSize    int64
	totalWritten *atomic.Int64
	cfg          Options
}

func worker(ctx context.Context, id int, wc *workerCtx) error {
	bufPtr := bufPool.Get().(*[]byte)
	defer bufPool.Put(bufPtr)
	buf := *bufPtr

	// Each worker starts on a different mirror to spread load.
	currentMirrorIdx := id % len(wc.mirrors)

	for {
		wc.queue.IncIdle()
		task, ok := wc.queue.Pop()
		wc.queue.DecIdle()
		if !ok {
			return nil
		}

		var lastErr error
		attempt := 0
		rlRetries := 0

		for {
			// Pick the best mirror (skipping penalised hosts).
			idx, wait := wc.hostLimiter.PickMirror(wc.mirrorHosts, currentMirrorIdx, time.Now())
			currentMirrorIdx = idx

			// All mirrors are penalised — sleep until the earliest recovers.
			if wait > 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(wait):
				}
			}

			// currentURL must be assigned AFTER the wait (idx may have changed).
			currentURL := wc.mirrors[currentMirrorIdx]

			taskCtx, taskCancel := context.WithCancel(ctx)
			now := time.Now()
			activeTask := &ActiveTask{
				Task:        task,
				StartTime:   now,
				Cancel:      taskCancel,
				WindowStart: now,
			}
			if task.SharedMaxOffset != nil {
				activeTask.SharedMaxOffset = task.SharedMaxOffset
				activeTask.Hedged.Store(1)
			}
			activeTask.CurrentOffset.Store(task.Offset)
			activeTask.StopAt.Store(task.Offset + task.Length)
			activeTask.LastActivity.Store(now.UnixNano())

			wc.balancer.RegisterWorker(id, activeTask)

			lastErr = downloadTask(taskCtx, currentURL, wc.file, activeTask, buf, wc.client, wc.totalSize, wc.cfg, wc.totalWritten, wc.rateLimiter)

			wasExternallyCancelled := taskCtx.Err() != nil
			taskCancel()

			if ctx.Err() != nil {
				wc.balancer.UnregisterWorker(id)
				return ctx.Err()
			}

			if wasExternallyCancelled && lastErr != nil {
				// Health monitor or steal cancelled us — requeue remaining.
				currentMirrorIdx = (currentMirrorIdx + 1) % len(wc.mirrors)
				if remaining := activeTask.RemainingTask(); remaining != nil {
					// Clamp to original task end to prevent overlap.
					originalEnd := task.Offset + task.Length
					if remaining.Offset+remaining.Length > originalEnd {
						remaining.Length = originalEnd - remaining.Offset
					}
					if remaining.Length > 0 {
						wc.queue.Push(*remaining)
					}
				}
				wc.balancer.UnregisterWorker(id)
				lastErr = nil
				break
			}

			wc.balancer.UnregisterWorker(id)

			if lastErr == nil {
				wc.hostLimiter.RecordSuccess(wc.mirrorHosts[currentMirrorIdx])
				break
			}

			// Rate-limit error — penalise this host and rotate.
			if re, ok := lastErr.(rateLimitError); ok {
				penalty := wc.hostLimiter.Penalize(
					wc.mirrorHosts[currentMirrorIdx],
					time.Duration(re.retryAfter)*time.Second,
					re.retryAfter > 0,
					time.Now(),
				)
				rlRetries++
				log.Printf("[Worker %d] Rate limited on %s (retry %d/%d, wait until %s)",
					id, wc.mirrors[currentMirrorIdx], rlRetries, wc.cfg.MaxRetries, penalty.Format("15:04:05"))

				if rlRetries >= wc.cfg.MaxRetries {
					// All retries exhausted — surface as a hard error so the
					// download fails cleanly rather than spinning forever.
					lastErr = fmt.Errorf("rate limited after %d retries on %s", rlRetries, currentURL)
					break
				}

				// Rotate to next mirror (wraps around when only one exists).
				currentMirrorIdx = (currentMirrorIdx + 1) % len(wc.mirrors)
				resumeOnRetryOffset(&task, activeTask)
				continue
			}

			// Generic error — exponential backoff.
			attempt++
			if attempt >= wc.cfg.MaxRetries {
				break
			}
			log.Printf("[Worker %d] Error: %v. Retrying %d/%d", id, lastErr, attempt, wc.cfg.MaxRetries)
			if len(wc.mirrors) > 1 {
				currentMirrorIdx = (currentMirrorIdx + 1) % len(wc.mirrors)
			} else {
				if err := sleepBackoff(ctx, wc.cfg.BaseBackoff, wc.cfg.MaxBackoff, attempt-1, 0); err != nil {
					return err
				}
			}
			resumeOnRetryOffset(&task, activeTask)
		}

		if lastErr != nil {
			return lastErr
		}
	}
}

// resumeOnRetryOffset adjusts task offset/length to resume from where the
// active task got to (avoiding re-downloading already-written bytes).
func resumeOnRetryOffset(task *Task, activeTask *ActiveTask) {
	current := activeTask.CurrentOffset.Load()
	if current > task.Offset {
		task.Length = task.Offset + task.Length - current
		task.Offset = current
	}
}

// rateLimitError signals a 429 response with an optional Retry-After delay.
type rateLimitError struct {
	retryAfter int // seconds; 0 = not specified
}

func (e rateLimitError) Error() string { return "rate limited" }

// downloadTask fetches a single byte range and writes it to file at the correct
// offset. It also maintains the per-worker EMA speed measurement.
func downloadTask(
	ctx context.Context,
	rawurl string,
	file *os.File,
	activeTask *ActiveTask,
	buf []byte,
	client *http.Client,
	totalSize int64,
	cfg Options,
	totalWritten *atomic.Int64,
	limiter ByteLimiter,
) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawurl, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", cfg.UserAgent)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", activeTask.Offset, activeTask.Offset+activeTask.Length-1))
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		ra, _ := ParseRetryAfter(resp, time.Now())
		return rateLimitError{retryAfter: int(ra.Seconds())}
	}
	if resp.StatusCode != http.StatusPartialContent {
		if resp.StatusCode == http.StatusOK && activeTask.Offset == 0 && activeTask.Length == totalSize {
			// Server returned the full file for task 0 — acceptable.
		} else {
			return fmt.Errorf("unexpected status: %d", resp.StatusCode)
		}
	}

	// ── Batch state for efficient progress updates ──
	const batchInterval = 50 * time.Millisecond
	var pendingBytes int64
	var pendingStart int64 = -1
	lastUpdate := time.Now()

	flushPending := func() {
		if pendingBytes > 0 {
			totalWritten.Add(pendingBytes)
			pendingBytes = 0
			pendingStart = -1
			lastUpdate = time.Now()
		}
	}
	defer flushPending()

	offset := activeTask.Offset
	for {
		stopAt := activeTask.StopAt.Load()
		if offset >= stopAt {
			return nil // stolen: another worker took the rest
		}

		remaining := stopAt - offset
		readSize := int64(len(buf))
		if readSize > remaining {
			readSize = remaining
		}

		// Fill the buffer.
		readSoFar := 0
		var readErr error
		for readSoFar < int(readSize) {
			n, err := resp.Body.Read(buf[readSoFar:readSize])
			if n > 0 {
				readSoFar += n
				activeTask.LastActivity.Store(time.Now().UnixNano())
			}
			if err != nil {
				readErr = err
				break
			}
			if n == 0 {
				readErr = io.ErrUnexpectedEOF
				break
			}
		}

		if readSoFar > 0 {
			// Clamp to current StopAt (which may have moved due to stealing).
			currentStopAt := activeTask.StopAt.Load()
			if offset+int64(readSoFar) > currentStopAt {
				readSoFar = int(currentStopAt - offset)
				if readSoFar <= 0 {
					return nil
				}
			}

			// Apply rate limit BEFORE writing; update stall clock to avoid
			// false health-monitor cancellation during the wait.
			if limiter != nil {
				activeTask.LastActivity.Store(time.Now().UnixNano())
				activeTask.WaitingOnLimiter.Store(true)
				if err := limiter.WaitN(ctx, int64(readSoFar)); err != nil {
					activeTask.WaitingOnLimiter.Store(false)
					return err
				}
				activeTask.WaitingOnLimiter.Store(false)
				activeTask.LastActivity.Store(time.Now().UnixNano())
			}

			if _, writeErr := file.WriteAt(buf[:readSoFar], offset); writeErr != nil {
				return writeErr
			}

			now := time.Now()
			rangeStart := offset
			offset += int64(readSoFar)

			// ── Deduplicate progress for hedged pairs ──
			var newlyWritten int64
			activeTask.SharedMaxOffsetMu.RLock()
			ptr := activeTask.SharedMaxOffset
			activeTask.SharedMaxOffsetMu.RUnlock()

			if ptr != nil {
				for {
					maxOff := ptr.Load()
					if offset <= maxOff {
						newlyWritten = 0
						break
					}
					if rangeStart >= maxOff {
						if ptr.CompareAndSwap(maxOff, offset) {
							newlyWritten = int64(readSoFar)
							break
						}
					} else {
						if ptr.CompareAndSwap(maxOff, offset) {
							newlyWritten = offset - maxOff
							break
						}
					}
				}
			} else {
				newlyWritten = int64(readSoFar)
			}

			activeTask.CurrentOffset.Store(offset)
			activeTask.LastActivity.Store(now.UnixNano())

			// Accumulate for batched progress flush.
			if newlyWritten > 0 {
				if pendingStart == -1 {
					pendingStart = offset - newlyWritten
				}
				pendingBytes += newlyWritten
			}
			if pendingBytes >= workerBatchSize || now.Sub(lastUpdate) >= batchInterval {
				flushPending()
			}

			// ── Update EMA speed (2-second window) ──
			activeTask.WindowBytes.Add(newlyWritten)
			windowElapsed := now.Sub(activeTask.WindowStart).Seconds()
			if windowElapsed >= 2.0 {
				windowBytes := activeTask.WindowBytes.Swap(0)
				recentSpeed := float64(windowBytes) / windowElapsed
				activeTask.SpeedMu.Lock()
				const alpha = 0.3 // EMA smoothing factor
				if activeTask.Speed == 0 {
					activeTask.Speed = recentSpeed
				} else {
					activeTask.Speed = (1-alpha)*activeTask.Speed + alpha*recentSpeed
				}
				activeTask.SpeedMu.Unlock()
				activeTask.WindowStart = now
			}
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	return nil
}

// buildWorkerMirrors returns a de-duplicated list with primary URL first.
func buildWorkerMirrors(primary string, mirrors []string) (urls []string, hosts []string) {
	urls = append(urls, primary)
	hosts = append(hosts, MirrorHost(primary))
	seen := map[string]bool{primary: true}
	for _, m := range mirrors {
		if !seen[m] {
			seen[m] = true
			urls = append(urls, m)
			hosts = append(hosts, MirrorHost(m))
		}
	}
	return
}

// interruptibleSleep sleeps for d, returning false if ctx is cancelled.
func interruptibleSleep(ctx context.Context, d time.Duration) bool {
	select {
	case <-time.After(d):
		return true
	case <-ctx.Done():
		return false
	}
}

// newHTTPClient creates a fresh *http.Client backed by the shared transport pool.
func newHTTPClient(cfg Options) (*http.Client, *http.Transport) {
	maxConns := cfg.Workers * 2
	t := DefaultTransportPool.AcquireTransport("", maxConns)
	return &http.Client{Transport: t}, t
}

// newRateLimiter builds a ByteLimiter from Options, or returns nil if unlimited.
func newRateLimiter(cfg Options) ByteLimiter {
	if cfg.RateLimitBps <= 0 {
		return nil
	}
	return NewTokenBucketLimiter(cfg.RateLimitBps)
}

// Ensure corehttp is still referenced (used in probe).
var _ = corehttp.Probe
