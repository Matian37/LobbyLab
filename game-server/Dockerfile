FROM golang:1.27-alpine AS builder

WORKDIR /game-server

COPY host/go.mod host/go.sum ./host/
RUN cd host && go mod download

COPY host ./host
RUN cd host && go build -o /game-server-bin .

# alpine image is used only as example
# change it according to your needs
FROM alpine:3.24 AS runner

COPY --from=builder /game-server-bin /usr/local/bin/game-server

WORKDIR /game-server
COPY runtime ./runtime

# here you can setup your actual game-server
# runtime folder is meant for holding game-server files
RUN chmod +x runtime/example.ash

# first argument in CMD is the command which starts your actual game-server
# modify it to your needs
CMD ["game-server", "./runtime/example.ash"]
