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
echo "KURWADAS"
if [ -z "$match_result" ]; then
    echo "error: --match-result flag is required" >&2
    exit 1
fi

socat UDP-LISTEN:$PORT,bind=$HOST,fork,reuseaddr SYSTEM:'
    read PROSBA
    echo "[Odebrano]: $PROSBA" >&2
    echo "Odebralam: $PROSBA (pozdrawia serwer UDP)"
'

sleep 3
printf '{hej seojihfsjkdfds}' > "$match_result"