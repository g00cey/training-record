#!/bin/bash
# Spin Bike Image Extraction Server - 手動起動スクリプト
# 使い方: ./start.sh [--daemon]

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SERVER_SCRIPT="$SCRIPT_DIR/server.py"
PID_FILE="$SCRIPT_DIR/server.pid"
LOG_FILE="$SCRIPT_DIR/server.log"

# 環境変数読み込み
if [ -f /opt/data/.env ]; then
    set -a
    . /opt/data/.env
    set +a
fi

# 既存プロセス確認
if [ -f "$PID_FILE" ]; then
    OLD_PID=$(cat "$PID_FILE")
    if kill -0 "$OLD_PID" 2>/dev/null; then
        echo "Server already running (PID: $OLD_PID)"
        echo "Stop with: kill $OLD_PID"
        exit 1
    fi
    rm -f "$PID_FILE"
fi

# 引数確認
if [ "$1" = "--daemon" ]; then
    echo "Starting server in daemon mode..."
    nohup python3 "$SERVER_SCRIPT" > "$LOG_FILE" 2>&1 &
    echo $! > "$PID_FILE"
    echo "Server started (PID: $(cat $PID_FILE))"
    echo "Log: $LOG_FILE"
    echo "Stop: kill \$(cat $PID_FILE)"
else
    echo "Starting server in foreground (Ctrl+C to stop)..."
    python3 "$SERVER_SCRIPT"
fi
