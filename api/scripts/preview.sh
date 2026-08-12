#!/bin/bash
set -e

cleanup() {
  docker rm -f postgres-test 2>/dev/null || true
  rm -rf /tmp/multiplayer-asset-e2e
}
trap cleanup EXIT TERM INT

docker rm -f postgres-test 2>/dev/null || true

docker run --name postgres-test \
  -e POSTGRES_PASSWORD=123 \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_DB=postgres \
  -p 5432:5432 \
  -v ../init.sql:/docker-entrypoint-initdb.d/init.sql \
  -d postgres

until docker exec postgres-test pg_isready -U postgres; do
  sleep 0.5
done

mkdir -p /tmp/multiplayer-asset-e2e

DATABASE_URL='postgresql://postgres:123@localhost:5432/postgres' vite preview
