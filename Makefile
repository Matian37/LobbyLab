.PHONY: build up down logs

build:
	docker build -t game-server:latest ./game-server $(BUILD_ARGS)
	docker compose build $(BUILD_ARGS)

up: build
	docker compose up $(UP_ARGS)

down:
	docker compose down $(DOWN_ARGS)

logs:
	# limitation: past game-server logs are not sorted with past compose logs
	@{ docker compose -p multiplayer-asset ps -aq; \
	   docker ps -aq --filter 'label=com.github.multiplayer-asset.worker=true'; \
	 } | sort -u | xargs -r -P 0 -I{} docker logs --timestamps $(LOG_ARGS) {}
