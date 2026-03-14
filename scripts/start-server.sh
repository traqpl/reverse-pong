#!/usr/bin/env bash
set -e

DIR="$(cd "$(dirname "$0")" && pwd)"
BIN="$DIR/reverse-pong"
LOG="$DIR/server.log"

sudo setcap cap_net_bind_service=+ep "$BIN"
nohup env PORT=443 "$BIN" >> "$LOG" 2>&1 &
echo "deployed PID: $!"
