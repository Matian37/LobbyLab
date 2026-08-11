# Setup Guide

## Prerequisites

- [Docker](https://docs.docker.com/engine/install/)
- [Docker Compose](https://docs.docker.com/compose/install/)

## Configuration

### 1. Environment Variables

Copy the example environment file:

```sh
cp .env.example .env
```

| Variable | Description |
|---|---|
| `POSTGRES_DB` | Database name |
| `POSTGRES_USER` | Database user |
| `POSTGRES_PASSWORD` | Database password |
| `NATS_URI` | NATS connection URI |
| `PUBLIC_HOST` | Publicly reachable address of the Docker host |
| `PUBLIC_GAME_LAUNCH_URL` | Template URL used to launch the game client when a match is found |

The defaults are suitable for local development. In production, set `PUBLIC_HOST` to your server's public IP or domain name so game clients can connect to game server containers.

#### Game Launch URL

`PUBLIC_GAME_LAUNCH_URL` is a template that the API fills in with the matched game server's details and then opens in the browser to hand off to the game client. It must contain three placeholders, which are replaced with values from the user's active match:

| Placeholder | Replaced with                         |
| ----------- | ------------------------------------- |
| `{host}`    | Game server host (from `PUBLIC_HOST`) |
| `{port}`    | Game server port, e.g. `8080/udp`     |
| `{token}`   | The user's `match_auth_token`         |

Example:

```
mygame://join?host={host}&port={port}&token={token}
```

It is expected that a `mygame://` protocol handler is registered on the user's device, otherwise the game client will not be able to launch.

#### Game Client Download

The web frontend shows a "Download client" button to logged-in users that
downloads the game client archive from `GET /api/download`. For the download
to work you must make the client file available to the `api` container:

1. **Place the client archive in the mounted downloads directory.**
   The `api` service mounts `./data/downloads` into the container at
   `/api/downloads`. Drop your built game client archive there, e.g.:

   ```
   data/downloads/game-client.zip
   ```

2. **Set the file name.**
   `compose.yaml` sets `GAME_CLIENT_FILE: game-client.zip` on the `api`
   service, which must match the file name from step 1. Change it if you name
   the archive differently.

Both `DOWNLOADS_DIR` and `GAME_CLIENT_FILE` must be set on the `api` service.
If either is missing, `GET /api/download` always responds with `404`.

If the file is missing, `GET /api/download` responds with `404`, and users see
a failed download. There is no default file bundled with the stack, so this
setup step is required before the download link works.

### 2. Game Server

#### 2.1 Dockerfile

Open `game-server/Dockerfile`. The build uses a two-stage Dockerfile:

**Builder stage:** Compiles the Go wrapper. This image can remain unchanged unless you modify the Go code.

**Runner stage** — the runtime image that ships the game server binary. Modify the following:

1. **Base image** — Replace the runner image with one that supports your game server runtime (e.g., Ubuntu if your game server requires specific system libraries).
2. **Runtime files** — Place your game server executable and any required assets in the `game-server/runtime/` directory. The Dockerfile copies this folder into the runner image.
3. **CMD argument** — Set the first argument of CMD to the command that starts your actual game server.

#### 2.2 Runtime Contract

The Go wrapper launches your game server as a child process and passes two command-line flags:

- `--match-config <path>` — Path to a JSON file containing the match configuration. By default the `config` field contains the list of matched players with their logins and authorization tokens:
  ```json
  {
    "matchID": 1234,
    "config": {
      "players": [
        {"login": "user1", "matchAuthToken": "a3B2cD1eF4gH5iJ6kL7mN8oP9qR0sT1uV2wX3yZ4="},
        {"login": "user2", "matchAuthToken": "b4C5dE6fG7hI8jK9lM0nO1pQ2rS3tU4vW5xY6zA7="}
      ]
    }
  }
  ```
  You can extend `config` with additional fields when adding players to the waiting queue.
- `--match-result <path>` — Path where your game server **must write** the match result as a JSON file before exiting. Example:
  ```json
  {"winner": "player1", "score": 10}
  ```

Your game server must read `--match-config` at startup, run the match, write results to `--match-result`, and then exit.

### 3. Docker Compose (compose.yaml)

Open `compose.yaml`. The `server-manager` service contains several configuration values that control game server orchestration:

```yaml
server-manager:
  # ...
  environment:
    GAME_SERVER_COUNT: 2
    GAME_SERVER_EXPOSE_PORTS: "7777/udp,8080"
    GAME_SERVER_CLIENT_PORT: "7777/udp"
    PLAYERS_PER_ROOM: 2
```

| Variable | Description |
|---|---|
| `GAME_SERVER_COUNT` | Number of game server containers to run concurrently. Each container hosts a single game at a time. |
| `GAME_SERVER_EXPOSE_PORTS` | Comma-separated list of ports the game server exposes (protocol suffix required for UDP). |
| `GAME_SERVER_CLIENT_PORT` | The port clients connect to. Must be one of the exposed ports. |
| `PLAYERS_PER_ROOM` | Number of players required to start a match. |

Adjust these values according to your game's requirements.

### 4. Start the Stack

```sh
make up
```

This builds all Docker images (game-server, api, server-manager) and starts every service defined in `compose.yaml`. To run in detached mode:

```sh
make up UP_ARGS="-d"
```

To rebuild images without cache:

```sh
make build BUILD_ARGS="--no-cache"
```

## Additional Notes

- Game server containers are managed entirely by the server manager. Do not start or stop them manually while the platform is running.
- Database data is persisted in `data/db/` and NATS data in `data/nats/`.
- The `server-manager` uses labels (`com.github.multiplayer-asset.worker: "true"`) to track running worker containers. Avoid manually creating containers with this label.
