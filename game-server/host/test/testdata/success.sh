#!/bin/sh
# Simulates a well-behaved game server that echoes the whole match config into
# its result, proving it received the correct config and can write real results.

match_config=""
match_result=""
while [ $# -gt 0 ]; do
    case "$1" in
        --match-config) match_config="$2"; shift 2 ;;
        --match-result) match_result="$2"; shift 2 ;;
        *) shift ;;
    esac
done

if [ -z "$match_config" ] || [ -z "$match_result" ]; then
    echo "error: --match-config and --match-result are required" >&2
    exit 1
fi

config=$(cat "$match_config")
printf '{"winner":"player1","score":10,"config":%s}' "$config" > "$match_result"
exit 0
