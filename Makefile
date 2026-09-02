.PHONY: build up down logs

build:
	docker build -t game-server:latest ./game-server $(BUILD_ARGS)
	docker compose build $(BUILD_ARGS)

up: build
	docker compose up $(UP_ARGS)

down:
	docker compose down $(DOWN_ARGS)

logs:
	docker ps -aq \
	    --filter 'label=com.github.Matian37.LobbyLab.service=game-server' \
	&& docker ps -aq \
	    --filter 'label=com.docker.compose.project=LobbyLab' \
	| xargs -r -n 1 -P 0 docker logs $(LOG_ARGS)
