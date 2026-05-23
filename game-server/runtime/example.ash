#!/bin/ash

set -eu

cleanup() {
  echo "SIGTERM received, cleaning up..."
  # shutdown logic here
  exit 0
}

trap 'cleanup' TERM

echo "Running (pid $$)"

sleep 15