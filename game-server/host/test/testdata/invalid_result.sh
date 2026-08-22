#!/bin/sh
# Simulates a game server that writes a non-JSON result and exits cleanly.

match_result=""
while [ $# -gt 0 ]; do
    case "$1" in
        --match-result) match_result="$2"; shift 2 ;;
        *) shift ;;
    esac
done

if [ -z "$match_result" ]; then
    echo "error: --match-result flag is required" >&2
    exit 1
fi

printf 'this is not valid json' > "$match_result"
exit 0
