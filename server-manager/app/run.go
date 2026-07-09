package app

import (
	"context"
	"server-manager/internal"
)

func Run(ctx context.Context, config *internal.EnvConfig) error {
	wm := NewWorkerManager(config)
	defer func() { _ = wm.Shutdown() }()
	if err := wm.Start(ctx); err != nil {
		return err
	}

	matchmaker := NewMatchmaker(wm, config)
	defer func() { _ = matchmaker.Shutdown() }()
	if err := matchmaker.Start(ctx); err != nil {
		return err
	}

	<-ctx.Done()

	return nil
}
