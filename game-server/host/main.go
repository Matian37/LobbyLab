// The game-server is a Go wrapper that runs a user-provided game server
// process on demand for specific match configurations. It uses a message broker
// to listen for requests. After running match it produces a result,
// and publishes it back to the broker. Game-server also answers health pings so
// the server-manager knows whether the worker is alive.
//
// The binary is launched by the Dockerfile with the game-server command given
// as a single positional argument, for example:
//
//	game-server "./my-game-server.bin --port 7777 --myflag myvalue"
//
// The directory named runtime is meant to hold the user's actual game-server files.
// The example.ash script is a reference usage of the game-server wrapper.
//
// Package main is the entry point of the game-server worker.
//
// Shutdown is either triggered by signal (SIGTERM, SIGINT) or by internal error.
// When shutdown is triggered, the application stops accepting new work,
// kills any running game server process, and exits.
// Application error is logged and reflected in the process exit code.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Matian37/multiplayer-asset/game-server/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	if err := app.Run(ctx); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}
