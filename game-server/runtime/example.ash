#!/bin/sh

trap 'exit 0' TERM

match_result=""
while [ $# -gt 0 ]; do
    case "$1" in
        --match-result)
            match_result="$2"
            shift 2
            ;;
        *)
            shift
            ;;
    esac
done

if [ -z "$match_result" ]; then
    echo "error: --match-result flag is required" >&2
    exit 1
fi

sleep 3
printf '{}' > "$match_result"