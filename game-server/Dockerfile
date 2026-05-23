FROM golang:1.26-alpine

WORKDIR /game-server

COPY runtime ./runtime
RUN chmod +x runtime/example.ash

COPY host/go.mod host/go.sum ./host/
RUN cd host && go mod download

COPY host ./host
RUN cd host && go build -o ../server .

CMD ["./server", "./runtime/example.ash"]