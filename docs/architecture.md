# Architecture

## Components

### Frontend + API
- Stateless. Serves the website and exposes API endpoints.
- Manages user connections waiting for matchmaking; stores their state in the DB.
- On match creation, informs users.

### DB
- Stores all persistent data (users, queue, matches, etc.).

### Server Manager
- Creates matches from the database queue.
- Assigns game servers to run a match with a specific config.
- Sends custom events like e.g. bans

### Game Server
- Runs the actual game server.
- Wrapped by a Go script that communicates with the server manager through RabbitMQ and manages connections between server processes.

### RabbitMQ
- Task queue between server manager and game servers.
- Separate queue for sending events to game servers during a match.

## Matchmaking flow

1. User connects to the frontend/API and waits.
2. API creates a WebSocket connection with the user and keeps it alive. It also saves the user in the DB queue.
3. Server manager waits for any game server to become free, then creates a match if possible.
4. Game server receives the match config and starts running the server.
5. Server manager can send events that the game server reads during the match.
6. At the end of the game, the game server sends the match result to the server manager and waits for further jobs.
