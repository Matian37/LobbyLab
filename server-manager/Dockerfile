FROM golang:1.27.1-alpine

WORKDIR /server-manager

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o server-manager

CMD ["./server-manager"]
