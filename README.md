# multiplayer-asset

Multiplayer game server orchestration platform.

## Prerequisites

* [Docker](https://docs.docker.com/engine/install/)
* [Docker Compose](https://docs.docker.com/compose/install/)

## Quick start

```sh
cp .env.example .env
make up
```

## Development

```sh
# Build images without starting
make build BUILD_ARGS="--no-cache"

# Start stack with build options
make up UP_ARGS="--build"

# Start stack in background
make up UP_ARGS="-d"

# Stop stack immediately
make down DOWN_ARGS="-t 0"
```
