#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
BIN="$DIR/reverse-pong"
LOG="$DIR/server.log"
CONFIG="${DIR}/config.yaml"

config_value() {
	local key="$1"
	local file="$2"
	sed -nE "s/^${key}:[[:space:]]*\"?([^\"]*)\"?$/\\1/p" "$file" | head -n 1
}

PORT="${PORT:-}"
DB_PATH="${DB_PATH:-}"

if [ -f "$CONFIG" ]; then
	if [ -z "$PORT" ]; then
		PORT="$(config_value port "$CONFIG")"
	fi
	if [ -z "$DB_PATH" ]; then
		DB_PATH="$(config_value db_path "$CONFIG")"
	fi
fi

PORT="${PORT:-8073}"
DB_PATH="${DB_PATH:-$DIR/reverse_pong_scores.db}"

nohup env PORT="$PORT" DB_PATH="$DB_PATH" "$BIN" >> "$LOG" 2>&1 &
echo "deployed PID: $!"
