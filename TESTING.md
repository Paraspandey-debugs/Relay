# Relay — Test Results & Resume Guide

## Test Results

### Mirror used
- Primary: `http://212.183.159.230/50MB.zip` (50 MB, single-thread friendly)
- Note: Hetzner mirrors enforce strict per-host rate limits and are better suited for single-worker fallback validation only.

### Benchmark: Relay (workers=4) vs curl (single connection)
| Tool | File | Time | Throughput | Workers |
|---|---|---|---|---|
| curl | 50 MB | 35.6s | ~1.4 MB/s | 1 |
| Relay | 50 MB | ~12–15s | ~3.4–4.5 MB/s | 4 |

Observations:
- Relay completes the same 50 MB file roughly 2.5–3x faster than curl on this mirror.
- curl shows a warming curve: starts very slow and gradually reaches the final speed.
- Relay reaches near-peak throughput much faster due to parallel chunk workers, giving better real-world UX.



## Resume Behavior

### Key points
- Partial files are written as `.part` files at the destination.
- Progress is checkpointed by byte-range, so interruption does not require restarting from 0.
- Pausing stops active workers cleanly; resumed downloads continue from the saved byte offset.
- After a daemon restart, persisted state reloads and downloads resume automatically.
- Deleting a download removes both the queued entry and the partial file when cleanup is enabled.

### Practical impact
- Unreliable networks: safe to restart daemon or loose connectivity; downloads pick up where they left off.
- Large files: partial progress is preserved across restarts, making long transfers resilient.
- Rate-limit recovery: if a 429 forces a single-worker fallback, the download keeps its existing progress instead of starting over.