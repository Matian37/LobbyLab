# multiplayer-asset

Multiplayer game platform used for running matchmaking with dedicated game servers.
Designed to run any user-provided game server on a single VPS.

## Table of Contents

- [Features](#features)
- [Structure](#structure)
- [Installation](#installation)
- [Setup](#setup)
- [Deployment](#deployment)
- [Authors](#authors)

## Features

- Matchmaking for any dedicated game server
- Website with authentication, matchmaking, and match history
- Game client downloads for registered users
- Adjustable matchmaking configuration

## Structure

The project consists of three main Docker services:

- **API (SvelteKit)**
  - Serves the website and exposes the REST API
  - Provides matchmaking, authentication, and match history
- **Server-Manager (Go)**
  - Orchestrates the lifecycle of game servers
  - Runs actual matchmaking and assigns matches to game servers
- **Game-Server (Go)**
  - Wraps your actual game server and manages its lifecycle
  - Listens for match assignments and starts them

Docker also runs a PostgreSQL database and a NATS message broker for the services to use.
For more details see [architecture.md](docs/architecture.md).

## Installation

Clone the repository:

```bash
git clone https://github.com/Matian37/multiplayer-asset
cd ./multiplayer-asset
```

Install required dependencies:

- [Docker](https://docs.docker.com/engine/install/)
- [Docker Compose](https://docs.docker.com/compose/install/)
- [Make](https://www.gnu.org/software/make/)

## Setup

For a quick start you can do:

```bash
cp .env.example .env
```

However, for a production-ready setup you should follow the instructions in [setup.md](docs/setup.md).

## Deployment

To deploy the services, run:

```bash
make up
```

You can specify additional docker compose up arguments in `UP_ARGS`.

```bash
make up UP_ARGS="-d"
```

This runs compose in detached mode.

To teardown the services, run:

```bash
make down
```
You can also specify additional docker compose down arguments in `DOWN_ARGS` the same way as `UP_ARGS`.

To display compose logs and game-servers logs run:

```bash
make logs
```

You can also specify additional docker compose logs arguments in `LOG_ARGS`.

For the full reference of the available Make targets and their arguments, see [makefile.md](docs/makefile.md).

## Authors

* Mateusz Pietrowcow ([Matian37](https://github.com/Matian37))
  * Server-Manager and Game-Server code maintainer

* Kacper Wojtkielewicz ([PASJANSS](https://github.com/PASJANSS))
  * API code maintainer

For authors' contribution details see [AUTHORS](AUTHORS).
