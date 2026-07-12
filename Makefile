.PHONY: build up down

build:
	docker build -t game-server:latest ./game-server $(BUILD_ARGS)
	docker compose build $(BUILD_ARGS)

up: build
	docker compose up $(UP_ARGS)

down:
	docker compose down $(DOWN_ARGS)
