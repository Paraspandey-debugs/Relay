<div align="center">

```
____  _____ _         _ __   __
|  _ \| ____| |      / \\ \ / /
| |_) |  _| | |     / _ \\ V / 
|  _ <| |___| |___ / ___ \| |  
|_| \_\_____|_____/_/   \_\_|  
```

### A blazing-fast, themeable TUI download manager for your terminal.

<p>
  <a href="https://golang.org"><img alt="Go" src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go&logoColor=white"/></a>
  <a href="https://github.com/charmbracelet/bubbletea"><img alt="Built with Bubble Tea" src="https://img.shields.io/badge/Built%20with-Bubble%20Tea-ff87d7?style=for-the-badge&logo=github&logoColor=white"/></a>
  <a href="https://github.com/charmbracelet/bubbles"><img alt="Bubbles" src="https://img.shields.io/badge/Bubbles-components-ffd700?style=for-the-badge&logo=github&logoColor=black"/></a>
  <a href="https://github.com/charmbracelet/lipgloss"><img alt="Lip Gloss" src="https://img.shields.io/badge/Lip%20Gloss-styling-ff69b4?style=for-the-badge&logo=github&logoColor=white"/></a>
  <a href="LICENSE"><img alt="License: MIT" src="https://img.shields.io/badge/License-MIT-5BD1D7?style=for-the-badge"/></a>
</p>

<p>
  <img alt="ocean theme" src="https://img.shields.io/badge/theme-ocean-5BD1D7?style=flat-square"/>
  <img alt="sunset theme" src="https://img.shields.io/badge/theme-sunset-F39C6B?style=flat-square"/>
  <img alt="mono theme" src="https://img.shields.io/badge/theme-mono-B0BEC5?style=flat-square"/>
</p>

</div>

---

## What is Relay?

**Relay** is a terminal-native download manager that runs entirely in your terminal. It uses parallel chunked HTTP downloads, supports resuming interrupted transfers, verifies SHA-256 checksums, and presents everything in a beautiful live TUI — all driven by a single binary.

> Built on the [Charm](https://charm.sh) ecosystem — [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles), and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

---

## Features

- **Parallel chunked downloads** — splits files into chunks and downloads them simultaneously across multiple workers
- **Resilient resume** — paused/interrupted downloads use `.part` byte-range files; restarting the daemon resumes them automatically from the last byte
- **Queue management** — add as many URLs as you want; Relay schedules them based on your concurrency limit
- **Pause & Resume** — stop any active download and bring it back on demand
- **SHA-256 verification** — optionally validate every download against an expected checksum
- **Three beautiful themes** — `ocean`, `sunset`, and `mono`, plus full per-color overrides via CLI flags
- **Persistent state** — your download queue survives restarts via a JSON state file
- **Fully keyboard-driven** — every action is a single keypress

---

## Installation

```bash
git clone https://github.com/Paraspandey-debugs/Relay.git
cd Relay
bash installer.sh
```

This installs `relayd` into your terminal, typically at `/usr/local/bin/relayd` or `~/.local/bin/relayd`.

If you want a custom install location:

```bash
INSTALL_DIR=~/.local/bin bash installer.sh
```

If you only want a local binary without installing to your `PATH`:

```bash
go build -o relayd ./cmd/dm
```

---

## Usage

```bash
# Launch after installing with installer.sh
relayd

# Use the sunset theme with 5 concurrent downloads
relayd --theme sunset --concurrency 5

# Point to a custom state file
relayd --state ~/my-downloads.state.json

# Override individual colors
relayd --color-accent "#FF00FF" --color-background "#0D0D0D"

# If you built locally instead of installing
./relayd
```

### All Flags

| Flag | Default | Description |
|---|---|---|
| `--state` | `relay-downloads.state.json` | Path to the persistent state file |
| `--concurrency` | `3` | Max number of simultaneous downloads |
| `--theme` | `ocean` | TUI color theme (`ocean` \| `sunset` \| `mono`) |
| `--refresh-ms` | `250` | UI refresh interval in milliseconds |
| `--workers` | `0` | Default parallel chunk workers per download (`0` = built-in default, capped at 8 per host) |
| `--cleanup` | `true` | Remove partial files when a download is deleted |
| `--color-background` | | Override background color (hex or ANSI) |
| `--color-foreground` | | Override foreground color |
| `--color-accent` | | Override accent color |
| `--color-secondary` | | Override secondary color |
| `--color-success` | | Override success color |
| `--color-warning` | | Override warning color |
| `--color-error` | | Override error color |
| `--color-muted` | | Override muted color |
| `--color-header` | | Override header color |
| `--color-card` | | Override card background color |
| `--color-selected-card` | | Override selected card color |

---

## Keybindings

| Key | Action |
|---|---|
| `1` | Show queued/paused/errored tab |
| `2` | Show active downloads tab |
| `3` | Show completed downloads tab |
| `Tab` | Cycle between tabs |
| `f` | Start/clear list filter search |
| `l` | Toggle event log panel |
| `g` / `G` | Jump log view to top/bottom |
| `a` | Add a new download |
| `p` | Pause the selected download |
| `r` | Resume the selected download |
| `x` | Prompt to remove selected download |
| `y` / `n` | Confirm/cancel destructive prompts |
| `s` | Open the download settings panel |
| `K` | Move selected item up in the queue |
| `J` | Move selected item down in the queue |
| `R` | Force refresh the view |
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `?` / `h` | Toggle help overlay |
| `Ctrl+Q` / `Ctrl+C` | Quit |

---

## Themes

Relay ships with three handcrafted themes. Switch with `--theme <name>`.

<table>
<tr>
<td align="center"><b>🌊 Ocean</b></td>
<td align="center"><b>🌅 Sunset</b></td>
<td align="center"><b>⬜ Mono</b></td>
</tr>
<tr>
<td>

Deep navy background with cool cyan accents and soft blue cards. The default experience.

`--theme ocean`

</td>
<td>

Warm dark background with amber and coral highlights — easy on the eyes at night.

`--theme sunset`

</td>
<td>

Minimal greyscale palette. Clean, distraction-free, works everywhere.

`--theme mono`

</td>
</tr>
</table>

---

## Download Options

Each download inherits from a set of defaults that can be overridden per-job:

| Option | Default | Description |
|---|---|---|
| `Workers` | `12` | Number of parallel chunk workers |
| `MinChunkSize` | `1 MB` | Minimum size for each chunk |
| `MaxChunkSize` | `8 MB` | Maximum size for each chunk |
| `Timeout` | `30s` | Per-request HTTP timeout |
| `MaxRetries` | `10` | Max retry attempts on failure |
| `BaseBackoff` | `500ms` | Initial backoff delay between retries |
| `MaxBackoff` | `20s` | Maximum backoff delay |
| `ExpectedSHA256Hex` | — | Optional SHA-256 hex checksum to verify |
| `ForceSingle` | `false` | Force single-stream download (no chunking) |
| `NoResume` | `false` | Disable resume, re-download from scratch |

---

## Architecture

```
cmd/dm/          → entry point & CLI flags
internal/
  core/
    download/    → chunked HTTP downloader, state & progress
    checksum/    → SHA-256 verification
    httpclient/  → HTTP client with probe (HEAD) support
  manager/       → queue, concurrency scheduling, state persistence
  tui/           → Bubble Tea model, views, themes, keybindings
```

### Implementation details

**Download engine**
- `DownloadFileV2` probes the URL with a HEAD request to confirm byte-range support, then splits the file into chunk-sized segments.
- A `TaskQueue` holds pending byte ranges; worker goroutines pop segments and write to a shared `.part` file using `os.File` with offset writes.
- A `Balancer` tracks active workers and can steal remaining work if a worker falls behind or gets rate-limited.

**Concurrency & scheduling**
- The `Manager` owns a bounded queue and an `active` set. `scheduleLocked()` starts new downloads until either the queue is empty or `maxConcurrent` is reached.
- Each download configures per-file workers; the executor merges user options with defaults and enforces an 8-worker host cap.

**Resume mechanics**
- Progress is recorded as completed byte ranges inside a sidecar `.part.state.json` file. On restart, `loadState()` rebuilds the queue and the executor resumes from the last written offset.
- Pausing cancels the download context; the partial file remains on disk. Resuming re-queues the job and continues from the saved offset.

**Rate-limit resilience**
- Workers detect HTTP 429 and back off with exponential jitter up to `maxRetries`.
- If all retries are exhausted, the executor auto-fallback path runs: it retries once with `ForceSingle=true, Workers=1`, logging `[429 fallback] ...` on failure.

**Web / daemon mode**
- `cmd/dm/main.go` builds a single `relayd` binary. In headless mode it starts an HTTP API server that serves the embedded React frontend and JSON endpoints.
- The embedded frontend is built from `web/` and packaged via `embed.FS`.
- Runtime settings are exposed through `GET/PUT /api/config`; changes are applied immediately and persisted to `~/.config/relay/daemon.json`.

**TUI mode**
- The terminal UI is built with Bubble Tea. The model subscribes to manager events and re-renders on progress updates.
- Data flows: goroutine workers → `ProgressMsg` channel → executor → `Event` subscribers → TUI view update.

---

## License

Released under the [MIT License](LICENSE).

---

<div align="center">
  <sub>Made with ♥ and <a href="https://github.com/charmbracelet/bubbletea">Bubble Tea</a></sub>
</div>
