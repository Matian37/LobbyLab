#!/bin/bash

SCRIPT_DIR="$(dirname "$(realpath "${BASH_SOURCE[0]}")")"
INIT_SQL="$SCRIPT_DIR/../../init.sql"

docker stop postgres-test 2>/dev/null
docker rm postgres-test 2>/dev/null

docker run --name postgres-test \
  -e POSTGRES_PASSWORD=123 \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_DB=postgres \
  -p 5432:5432 \
  -v "$INIT_SQL:/docker-entrypoint-initdb.d/init.sql" \
  -d postgres:18.6-alpine

until docker exec postgres-test pg_isready -U postgres; do
  sleep 0.5
done

DATABASE_URL='postgresql://postgres:123@localhost:5432/postgres' vite dev

docker stop postgres-test 2>/dev/null
docker rm postgres-test 2>/dev/null
