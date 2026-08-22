#!/bin/sh
# Simulates an unresponsive game server that ignores SIGTERM and can only be
# stopped with SIGKILL. It records its PID so tests can assert it is gone.

trap '' TERM

if [ -n "$SCRIPT_PID_FILE" ]; then
    echo "$$" > "$SCRIPT_PID_FILE"
fi

sleep inf
