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
- **Auto-resume** — interrupted downloads pick up exactly where they left off using `.part` state files
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
go build -o relay ./cmd/dm
```

Or install directly:

```bash
go install github.com/Paraspandey-debugs/Relay/cmd/dm@latest
```

---

## Usage

```bash
# Launch with defaults
./relay

# Use the sunset theme with 5 concurrent downloads
./relay --theme sunset --concurrency 5

# Point to a custom state file
./relay --state ~/my-downloads.state.json

# Override individual colors
./relay --color-accent "#FF00FF" --color-background "#0D0D0D"
```

### All Flags

| Flag | Default | Description |
|---|---|---|
| `--state` | `relay-downloads.state.json` | Path to the persistent state file |
| `--concurrency` | `3` | Max number of simultaneous downloads |
| `--theme` | `ocean` | TUI color theme (`ocean` \| `sunset` \| `mono`) |
| `--refresh-ms` | `250` | UI refresh interval in milliseconds |
| `--workers` | `0` | Default parallel chunk workers per download (`0` = auto) |
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
| `a` | Add a new download |
| `p` | Pause the selected download |
| `r` | Resume the selected download |
| `x` | Remove the selected download |
| `K` | Move selected item up in the queue |
| `J` | Move selected item down in the queue |
| `R` | Force refresh the view |
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `?` / `h` | Toggle help overlay |
| `q` / `Ctrl+C` | Quit |

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
| `MinChunkSize` | `2 MB` | Minimum size for each chunk |
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

---

## License

Released under the [MIT License](LICENSE).

---

<div align="center">
  <sub>Made with ♥ and <a href="https://github.com/charmbracelet/bubbletea">Bubble Tea</a></sub>
</div>
