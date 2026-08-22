#!/bin/sh
# Simulates an unresponsive game server that ignores SIGTERM and spawns child
# processes. It records its own PID and the PIDs of all children so tests can
# assert the whole process group is gone after a forceful kill.

if [ -n "$SCRIPT_PID_FILE" ]; then
    echo "$$" > "$SCRIPT_PID_FILE"
fi

trap '' TERM

sleep inf &
child1=$!
sleep inf &
child2=$!
echo "$child1" >> "$SCRIPT_PID_FILE"
echo "$child2" >> "$SCRIPT_PID_FILE"

wait
