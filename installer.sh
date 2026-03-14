#!/usr/bin/env bash
# installer.sh — install Relay download manager from source
#
# Usage:
#   bash installer.sh
#   INSTALL_DIR=~/.local/bin bash installer.sh
set -euo pipefail

REPO_URL="https://github.com/Paraspandey-debugs/Relay"
BIN_NAME="relayd"
MIN_GO_MAJOR=1
MIN_GO_MINOR=22

# ── Helpers ────────────────────────────────────────────────────────────────────
info()  { printf '\033[0;32m==>\033[0m %s\n' "$*"; }
warn()  { printf '\033[0;33mWarn:\033[0m %s\n' "$*" >&2; }
die()   { printf '\033[0;31mError:\033[0m %s\n' "$*" >&2; exit 1; }

# ── Determine install directory ────────────────────────────────────────────────
if [[ -n "${INSTALL_DIR:-}" ]]; then
    DEST="$INSTALL_DIR"
elif [[ -d "/usr/local/bin" ]]; then
    DEST="/usr/local/bin"
else
    DEST="$HOME/.local/bin"
fi

echo ""
echo "  ____  _____ _         _ __   __"
echo "  |  _ \| ____| |      / \\ \ / /"
echo "  | |_) |  _| | |     / _ \\ V / "
echo "  |  _ <| |___| |___ / ___ \| |  "
echo "  |_| \_\_____|_____/_/   \_\_|  "
echo ""
echo "  Relay — TUI Download Manager Installer"
echo ""

# ── Dependency checks ──────────────────────────────────────────────────────────
if ! command -v go &>/dev/null; then
    die "Go is not installed. Install it from: https://go.dev/dl/"
fi

GO_RAW=$(go version | awk '{print $3}' | sed 's/go//')
GO_MAJOR=$(echo "$GO_RAW" | cut -d. -f1)
GO_MINOR=$(echo "$GO_RAW" | cut -d. -f2)

if [[ "$GO_MAJOR" -lt "$MIN_GO_MAJOR" ]] || \
   { [[ "$GO_MAJOR" -eq "$MIN_GO_MAJOR" ]] && [[ "$GO_MINOR" -lt "$MIN_GO_MINOR" ]]; }; then
    die "Go ${MIN_GO_MAJOR}.${MIN_GO_MINOR}+ is required (found ${GO_RAW})."
fi

info "Go ${GO_RAW} detected — OK"

# ── Locate source (local repo or clone) ────────────────────────────────────────
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [[ -f "$SCRIPT_DIR/go.mod" && -d "$SCRIPT_DIR/cmd/dm" ]]; then
    info "Building from local source: ${SCRIPT_DIR}"
    SRC_DIR="$SCRIPT_DIR"
else
    if ! command -v git &>/dev/null; then
        die "git is not installed. Please install git and try again."
    fi
    info "Cloning ${REPO_URL}..."
    git clone --depth=1 "$REPO_URL" "$TMP/relay" 2>&1 | sed 's/^/    /'
    SRC_DIR="$TMP/relay"
fi

info "Building relay binary..."
(
    cd "$SRC_DIR"
    go build -ldflags="-s -w" -o "$TMP/relay-bin" ./cmd/dm
)

# ── Install ────────────────────────────────────────────────────────────────────
info "Installing to ${DEST}/${BIN_NAME}..."
mkdir -p "$DEST"

if [[ -w "$DEST" ]]; then
    mv "$TMP/relay-bin" "$DEST/$BIN_NAME"
else
    echo "    (requires sudo for ${DEST})"
    sudo mv "$TMP/relay-bin" "$DEST/$BIN_NAME"
fi

chmod 755 "$DEST/$BIN_NAME"

echo ""
info "Relay installed successfully!"
echo ""
echo "  Run: ${BIN_NAME}"
echo ""

# ── PATH hint ──────────────────────────────────────────────────────────────────
if ! printf '%s\n' "${PATH//:/$'\n'}" | grep -qxF "$DEST"; then
    warn "${DEST} is not in your PATH."
    echo "  Add this line to your shell config (~/.bashrc, ~/.zshrc, etc.):"
    echo ""
    echo "    export PATH=\"\$PATH:${DEST}\""
    echo ""
fi
