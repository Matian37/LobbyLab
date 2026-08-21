#!/bin/sh

trap 'exit 0' TERM

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
if [ -z "$match_result_path" ]; then
    echo "error: --match-result flag is required" >&2
    exit 1
fi

match_config=$(cat "$match_config_path")
export match_config

timeout 5s socat -v UDP-LISTEN:7777,fork SYSTEM:"echo PING\"\$match_config\""

printf $match_config > "$match_result_path"