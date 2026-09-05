# Setup Guide

## Prerequisites

- [Docker](https://docs.docker.com/engine/install/)
- [Docker Compose](https://docs.docker.com/compose/install/)
- [Make](https://www.gnu.org/software/make/)

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
| `GAME_SERVER_COUNT` | Number of game server containers to run concurrently |
| `GAME_SERVER_EXPOSE_PORTS` | Comma-separated ports the game server exposes (protocol suffix required for UDP) |
| `GAME_SERVER_CLIENT_PORT` | Game server's internal player-facing port, published to a random host port. Must be one of the exposed ports. |
| `PLAYERS_PER_ROOM` | Number of players required to start a match, two is the minimum value |
| `DOWNLOADS_DIR` | Host path to the directory holding the downloadable game client archive |
| `GAME_CLIENT_FILE` | File name of the game client archive in the downloads directory |

The defaults are suitable for local development. In production, set `PUBLIC_HOST` to your server's public IP or domain name so game clients can connect to game server containers.

Values are defined in `.env` and referenced by `compose.yaml`. After changing any value, restart the stack for it to take effect.

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
downloads the game client archive from `GET /api/download` endpoint. 
For the download to work you must make the client file available in `.env`:

1. **Set the download directory.**
   Set `DOWNLOADS_DIR` to the folder where your game client archive is stored.

2. **Set the filename.**
   Set `GAME_CLIENT_FILE` to the name of the archive in that folder. It
   defaults to `game-client.zip`.

Both `DOWNLOADS_DIR` and `GAME_CLIENT_FILE` must be set (in `.env`). If either
is missing, `GET /api/download` always responds with `404`.

### 2. Game Server

#### 2.1 Dockerfile

Open `game-server/Dockerfile`. The build uses a two-stage Dockerfile:

**Builder stage:** Compiles the Go wrapper. This image can remain unchanged unless you modify the Go code.

**Runner stage** — the runtime image that ships the game server binary. Modify the following:

1. **Base image** — Replace the runner image with one that supports your game server runtime (e.g., Ubuntu if your game server requires specific system libraries).
2. **Runtime files** — Place your game server executable and any required assets in the `game-server/runtime/` directory. The Dockerfile copies this folder into the runner image.
3. **Runtime setup** — Compile and setup anything you need for your game server runtime.
4. **CMD argument** — Set the first argument of CMD to the command that starts your actual game server.

#### 2.2 Runtime Contract

The Go wrapper launches your game server as a child process and passes two command-line flags:

- `--match-config <path>` — Path to a JSON file containing the match configuration. By default the `players` field contains the list of matched players with their logins and authorization tokens:
  ```json
  {
    "players": [
      {"login": "user1", "matchAuthToken": "a3B2cD1eF4gH5iJ6kL7mN8oP9qR0sT1uV2wX3yZ4="},
      {"login": "user2", "matchAuthToken": "b4C5dE6fG7hI8jK9lM0nO1pQ2rS3tU4vW5xY6zA7="}
    ]
  }
  ```
  You can add additional fields, but this requires you to modify code which builds the match configuration in server-manager.

- `--match-result <path>` — Path where your game server **must write** the match result as a JSON file before exiting. Example:
  ```json
  {"winner": "player1", "players": ["player1", "player2"]}
  ```
  Currently API expects details in above format; to use your own you must modify part of code where the API displays details on page.

Your game server must read `--match-config` at startup, run the match, write results to `--match-result`, and then exit with zero status code. However, on failure, exit with a nonzero status code. The server manager will then mark the match as canceled.

Game server also needs to handle the following responsibilities:
1. Authenticate each user using the `matchAuthToken` provided in the match configuration.
2. Handle players attempting to reconnect.
3. Wait for players, and exit with a nonzero status code if not enough join within a set timeout.

### 3. Docker Compose (compose.yaml)

The `server-manager` service reads several values that control game server orchestration from your `.env` file (see the Configuration table in step 1):

| Variable | Description |
|---|---|
| `GAME_SERVER_COUNT` | Number of game server containers to run concurrently. Each container hosts a single game at a time. |
| `GAME_SERVER_EXPOSE_PORTS` | Comma-separated list of ports the game server exposes (protocol suffix required for UDP). |
| `GAME_SERVER_CLIENT_PORT` | Game server's internal player-facing port, published to a random host port. Must be one of the exposed ports. |
| `PLAYERS_PER_ROOM` | Number of players required to start a match. |

Adjust these values in `.env` according to your game's requirements.

### 4. Control the Platform

All commands below are provided by the project `Makefile`. For the full
reference of every target and its arguments see [makefile.md](makefile.md).

1. **Start services**:
```sh
make up
```
You can add `UP_ARGS` to customize the start behavior (e.g., `UP_ARGS="--detach"`).

2. **Build images**:
```sh
make build
```
You can add `BUILD_ARGS` to customize the build behavior (e.g., `BUILD_ARGS="--no-cache"`).

3. **Tear down services**:
```sh
make down
```
You can add `DOWN_ARGS` to customize the tear down behavior (e.g., `DOWN_ARGS="--volumes"`).

### 5. API Integration

Project allows you to implement your own matchmaking client.

Each router is documented in the [API Reference](api.md) which provides details on the available endpoints and responses.


### 6. Website Setup

Project provides a web interface for users. 
It covers auth, downloading the game client, matchmaking and viewing match results.

Only thing which you need setting up is protocol for `PUBLIC_GAME_LAUNCH_URL`.
Protocol allow browsers to open the game client directly from the web interface.
They require them to be registered on machine beforehand, so your game client should register the protocol during installation.
In absent of the protocol, user will not be able to join matches using the web interface.


## Additional Notes

- Game server containers are managed entirely by the server manager. Do not start or stop them manually while the platform is running.
- Database data is persisted in `data/db/` and NATS data in `data/nats/`.
- The `server-manager` uses labels (`com.github.Matian37.LobbyLab.service: "game-server"`) to track running worker containers. Avoid manually creating containers with this label.
