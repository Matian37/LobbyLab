#!/bin/sh
# Simulates an unresponsive game server that still handles SIGTERM gracefully.
# It records its PID and appends a marker when SIGTERM is received.

trap 'echo "sigterm" >> "$SCRIPT_PID_FILE"; exit 0' TERM

if [ -n "$SCRIPT_PID_FILE" ]; then
    echo "$$" > "$SCRIPT_PID_FILE"
fi

sleep inf
