# Architecture

## Components

### Frontend + API
- SvelteKit.
- Serves the website and exposes REST endpoints.
- Manages user connections waiting for matchmaking and stores their state in the DB.

### DB
- PostgreSQL.
- Stores all persistent data (users, queue, matches, etc.).
- Notification channel for match assignment.

### Server Manager
- Go.
- Creates matches from the DB queue, assigns workers, and manages containers.
- Consumes JetStream results and saves them to the DB.

### Game Server
- Go wrapper.
- Runs the actual game server process.
- Responds to health pings and assignments via NATS.
- Publishes results to NATS.

### NATS
- JetStream with stream `RESULT` on `workers.results` (LimitsPolicy, FileStorage, S2).
- Messaging backbone between server manager and game servers.

## NATS subjects

- `workers.health`
  - Type: request/reply
  - Direction: server manager → workers
  - Purpose: health check ping. Workers reply with their container ID.
- `workers.assign.<container_id>`
  - Type: request/reply
  - Direction: server manager → worker
  - Purpose: match assignment with match config. Worker acknowledges by reply.
- `workers.results`
  - Type: JetStream
  - Direction: worker → server manager
  - Purpose: durable match result stream.

Assign payload:

```json
{"matchID": 1234, "config": {}}
```

- `matchID`: unique identifier of the match.
- `config`: match-specific config data used by the actual game server.

Result payload:

```json
{"success": true, "matchID": 1234, "details": {}}
```

- `success`: `true` when the match completed, `false` when cancelled.
- `matchID`: unique identifier of the match.
- `details`: match-specific result data. Empty object on cancellation.

## Matchmaking flow

1. User registers, logs in, clicks Play. API adds them to the DB waiting queue and opens an SSE connection.
2. Server manager waits for a free worker.
3. Server manager polls the DB for enough players, gets a match ID, and looks up the worker's game port via Docker.
4. Server manager sends the match config to `workers.assign.<container_id>` and saves the match in the DB.
5. Game server receives the config, starts the game server process, and waits for it to exit.
6. Game server publishes the result to `workers.results`. Server manager saves to DB, and marks the worker and users free.
