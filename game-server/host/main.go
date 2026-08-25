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
