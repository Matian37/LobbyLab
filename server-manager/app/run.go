package app

import (
	"context"
	"server-manager/internal"
)

func Run(ctx context.Context, config *internal.EnvConfig) error {
	wm := NewWorkerManager(config)
	defer wm.Shutdown()
	if err := wm.Start(ctx); err != nil {
		return err
	}

	matchmaker := NewMatchmaker(wm, config)
	defer matchmaker.Shutdown()
	if err := matchmaker.Start(ctx); err != nil {
		return err
	}

	<-ctx.Done()

	return nil
}
