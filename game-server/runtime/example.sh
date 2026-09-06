#!/bin/sh

match_result_path=""
match_config_path=""
while [ $# -gt 0 ]; do
    case "$1" in
        --match-result)
            match_result_path="$2"
            shift 2
            ;;
        --match-config)
            match_config_path="$2"
            shift 2
            ;;
        *)
            shift
            ;;
    esac
done

if [ -z "$match_config_path" ]; then
    echo "error: --match-config flag is required" >&2
    exit 1
fi

if [ -z "$match_result_path" ]; then
    echo "error: --match-result flag is required" >&2
    exit 1
fi

cleanup() {
    if [ -n "$socat_pid" ]; then
        kill "$socat_pid" 2>/dev/null
    fi
    echo '{"success": true}' > "$match_result_path"
    exit 0
}
trap cleanup INT TERM EXIT

SCRIPT_PID=$$
export SCRIPT_PID

match_config=$(cat "$match_config_path")
export match_config

socat -v UDP-LISTEN:7777,fork SYSTEM:'sh -c "
    msg=\$(head -c 4)
    if [ \"\$msg\" = STOP ]; then
        kill -TERM \$SCRIPT_PID
    else
        printf 'PONG%s' "$match_config"
    fi
"' &

socat_pid=$!

wait "$socat_pid"
