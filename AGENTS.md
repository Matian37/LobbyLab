# Project Instructions

This is monorepo written in golang and svelte-kit; run with docker and nats broker. 
Project is a platform for running matchmaking with user provided game servers. 
It handles user login, game server management, and matchmaking logic. Runs everything in containers.

## Project Structure

- `api` - svelte-kit api+frontend server written in js
- `game-server` - runs actual game servers provided by user; written in golang
- `server-manager` - manages the lifecycle of game servers and matchmaking; written in golang
- `init.sql` - main postgres database schema
- `compose.yaml` - docker compose configuration
- `docs/architecture.md` contains detailed architecture documentation

## Building

- api:
  - use `npm run build`
- golang services:
  - use `go build -o <output name> ./...`
- project:
  - use `make build` with additional `BUILD_ARGS=<string docker build args>`

## Installing Dependencies

- api:
  - use `npm install <name>`
  - for dev tools use `npm install -D <name>`
- golang services:
  - use `go get <name>`
  - for dev tools use `go get -tool <name>`

## Testing

- api:
  - to run all tests use `npm test`
  - to run specific test file use `npm test -- <filename>`
  - to run specific test use `npm test -- --testNamePattern=<test pattern string>`
- server-manager:
  - to run specific test `go test -v --tags=<file test tag> -run=<testname> --timeout 1m ./...`
  - to run unit tests `go test -v --timeout 1m ./...`
  - to run race tests `go test -v -race -run="^TestRace" --timeout 1m ./...`
  - to run integration tests `go test -v -tags=integration -run="^TestIntegration" --timeout 1m ./...`
  - to run e2e tests `go test -v -tags=e2e -run="^TestE2E" --timeout 3m ./...`
- game-server:
  - to run tests `go test -v --timeout 1m ./...`
- project:
  - to run basic tests `act`
  - to run specific workflow `act -W ./.github/workflows/<workflowname>`
  - to run all tests including e2e `act pull_request`

## Formatting

- for golang use `go fmt`
- for svelte-kit use `npm run format`

## Linter

- for golang use `golangci-lint run ./...`
- for svelte-kit use `npm run lint`

## When Writing/Reviewing Code
- always run linter and formatter
- always run tests
  - firstly, run tests which only covers affected code
  - secondly, run a full test suite using `act` if possible

## When Blocked
- If tests fail after 3 attempts: stop and report the failing test with full output
- Never: delete files to resolve errors, or skip tests
