#!/bin/sh
# A configurable game server driven by the "mode" field in the match config.
#   success  -> echo config into a valid JSON result and exit 0
#   fail     -> exit with an error
#   invalid  -> write a non-JSON result and exit 0
#   hang     -> ignore SIGTERM, record PID, and sleep forever

match_config=""
match_result=""
while [ $# -gt 0 ]; do
    case "$1" in
        --match-config) match_config="$2"; shift 2 ;;
        --match-result) match_result="$2"; shift 2 ;;
        *) shift ;;
    esac
done

config=$(cat "$match_config")

case "$config" in
    *'"mode":"success"'*)
        printf '{"winner":"player1","score":10,"config":%s}' "$config" > "$match_result"
        exit 0
        ;;
    *'"mode":"fail"'*)
        exit 1
        ;;
    *'"mode":"invalid"'*)
        printf 'this is not valid json' > "$match_result"
        exit 0
        ;;
    *'"mode":"hang"'*)
        trap '' TERM
        if [ -n "$SCRIPT_PID_FILE" ]; then
            echo "$$" > "$SCRIPT_PID_FILE"
        fi
        sleep inf
        ;;
    *)
        echo "error: unknown mode" >&2
        exit 1
        ;;
esac
