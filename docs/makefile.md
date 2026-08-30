# Makefile Reference

The [`Makefile`](../Makefile) is a convenience wrapper around the underlying
Docker and Docker Compose commands. It centralizes the most common operations.

All targets accept optional arguments passed through as extra arguments to the
underlying command. The values are passed verbatim, so quoting works as it does
in the shell.

## Build

```sh
make build
```

Builds the `game-server:latest` Docker image from the `game-server/`
directory, then builds every service defined in `compose.yaml`.

You can customize the build behavior with `BUILD_ARGS`, which is forwarded to
both the `docker build` and `docker compose build` commands:

```sh
make build BUILD_ARGS="--no-cache"
```

## Up

```sh
make up
```

Starts (and assembles) the whole stack. `up` depends on `build`, so it builds
the images first and then runs `docker compose up`.

You can customize the start behavior with `UP_ARGS`, forwarded to
`docker compose up`. Typical use is detached mode:

```sh
make up UP_ARGS="--detach"
```

Because `up` depends on `build`, you can also pass `BUILD_ARGS` to customize
the build step that runs beforehand.

## Down

```sh
make down
```

Tears down the services by running `docker compose down`.

You can customize the tear down behavior with `DOWN_ARGS`, forwarded to
`docker compose down`. For example, to also remove associated anonymous
volumes:

```sh
make down DOWN_ARGS="--volumes"
```

## Logs

```sh
make logs
```

Streams the logs of the running containers. It collects containers in two
groups and streams them all in parallel:

1. **Worker containers** — game server worker containers, matched by the label
   `com.github.LobbyLab.worker=true`.
2. **Stack containers** — all containers belonging to the compose project,
   matched by the label `com.docker.compose.project=LobbyLab`.

The `docker logs` output is produced concurrently, so the two groups are
streamed side by side.

You can customize the logging behavior with `LOG_ARGS`, forwarded to each
`docker logs` invocation. For example, to follow only the last 100 lines:

```sh
make logs LOG_ARGS="--tail 100 -f"
```

> **Known limitation:** `docker logs` currently does not provide sorting for
> past logs. Because the output merges the compose service logs with the raw
> worker container logs, the lines from the different containers are not
> ordered chronologically relative to one another.

## Arguments Summary

| Variable     | Forwarded to                    | Example                          |
| ------------ | ------------------------------- | -------------------------------- |
| `BUILD_ARGS` | `docker build`, `docker compose build` | `BUILD_ARGS="--no-cache"`  |
| `UP_ARGS`    | `docker compose up`             | `UP_ARGS="--detach"`             |
| `DOWN_ARGS`  | `docker compose down`           | `DOWN_ARGS="--volumes"`          |
| `LOG_ARGS`   | `docker logs`                   | `LOG_ARGS="--tail 100 -f"`       |

For the overall setup and configuration of the platform, see
[setup.md](setup.md).
