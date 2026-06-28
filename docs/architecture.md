# Architecture

## Components

### Frontend + API
- Stateless. Serves the website and exposes API endpoints.
- Manages user connections waiting for matchmaking; stores their state in the
  DB.
- On match creation, informs users.

### DB
- Stores all persistent data (users, queue, matches, etc.).

### Server Manager
- Creates matches from the database queue.
- Assigns game servers to run a match with a specific config.
- Creates and manages game server containers.

### Game Server
- Runs the actual game server.
- Wrapped by a Go script that communicates with the server manager through NATS
  and manages game server process.

### NATS
- Messaging system for communication between server manager and game servers.

## NATS subjects

- `workers.health`
  - Type: pub/sub
  - Direction: server manager -> workers
  - Purpose: health check ping. Every worker subscribes to this subject.
- `workers.health.<container_id>`
  - Type: pub/sub
  - Direction: worker -> server manager
  - Purpose: health check pong. A worker replies on its container-specific subject.
- `workers.assign.<container_id>`
  - Type: request/reply
  - Direction: server manager -> worker
  - Purpose: match start request. The message body is the match config. The
    worker acknowledges the assignment by publishing an empty response to the
    NATS reply subject.
- `workers.results`
  - Type: JetStream
  - Direction: worker -> server manager
  - Purpose: durable match result event stream. Workers publish one event when
    a match finishes successfully or is cancelled.

Result event payload:

```json
{
  "success": true,
  "details": {}
}
```

- `success`: `true` when the match completed and produced a result, `false`
  when the match was cancelled.
- `details`: match-specific result data. For cancellation events this is an
  empty object.

## Matchmaking flow

1. User connects to the frontend/API and waits.
2. API creates a WebSocket connection with the user and keeps it alive. It also
   saves the user in the DB queue.
3. Server manager waits for any game server to become free, then creates a
   match if possible.
4. Game server receives the match config and starts running the server.
5. At the end of the game, the game server sends the match result to the server
   manager and waits for further jobs.
