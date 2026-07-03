#!/bin/bash
set -e

# Build the binary
echo "Building Relay binary..."
go build -o dm ./cmd/dm

# Create the user systemd directory if it doesn't exist
mkdir -p ~/.config/systemd/user/

# Get the absolute path to the binary
BIN_PATH="$(pwd)/dm"
# Get the absolute path for the state file (we'll keep it in the user's config dir)
CONFIG_DIR="$HOME/.config/relay"
mkdir -p "$CONFIG_DIR"
STATE_FILE="$CONFIG_DIR/relay-downloads.state.json"

# Generate the systemd service file
SERVICE_FILE="$HOME/.config/systemd/user/relay.service"

echo "Creating systemd user service at $SERVICE_FILE..."
cat <<EOF > "$SERVICE_FILE"
[Unit]
Description=Relay Download Daemon
After=network.target

[Service]
Type=simple
# Run headless, don't auto-open web since it's a background service
ExecStart=$BIN_PATH -headless -state "$STATE_FILE" -log "$CONFIG_DIR/relay.log"
Restart=on-failure
RestartSec=5
# Run from the user's home directory so default download paths resolve correctly
WorkingDirectory=$HOME

[Install]
WantedBy=default.target
EOF

# Reload systemd and enable the service
systemctl --user daemon-reload
systemctl --user enable relay.service
systemctl --user start relay.service

echo "========================================================="
echo "✅ Relay installed and started as a background service!"
echo "   State file location: $STATE_FILE"
echo "   Log file location  : $CONFIG_DIR/relay.log"
echo "   Web UI is running at: http://localhost:8080"
echo ""
echo "To manage the service, use the following commands:"
echo "  - Check status : systemctl --user status relay"
echo "  - View logs    : journalctl --user -fu relay"
echo "  - Stop service : systemctl --user stop relay"
echo "  - Start service: systemctl --user start relay"
echo "========================================================="
