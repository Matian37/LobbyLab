#!/bin/bash

docker stop postgres-test 2>/dev/null
docker rm postgres-test 2>/dev/null

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

DATABASE_URL='postgresql://postgres:123@localhost:5432/postgres' vite dev

docker stop postgres-test 2>/dev/null
docker rm postgres-test 2>/dev/null
