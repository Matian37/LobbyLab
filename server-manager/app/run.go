package app

import (
	"context"
	"log/slog"

	"github.com/Matian37/LobbyLab/server-manager/internal"
)

// Run runs the WorkerManager and Matchmaker, closing them when ctx is done.
func Run(ctx context.Context, config *internal.EnvConfig, logger *slog.Logger) error {
	logger.Debug("starting worker manager")

	wm := NewWorkerManager(config, logger)
	defer wm.Shutdown()
	if err := wm.Start(ctx); err != nil {
		return err
	}

	logger.Debug("worker manager started; starting matchmaker")

	matchmaker := NewMatchmaker(wm, config, logger)
	defer matchmaker.Shutdown()
	if err := matchmaker.Start(ctx); err != nil {
		return err
	}

	logger.Info("started")

	<-ctx.Done()

	logger.Info("shuting down")

	return nil
}
