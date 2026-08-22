package app

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v6"
)

func HandleBackoff(ctx context.Context, b backoff.BackOff, err error) {
	if err == nil {
		b.Reset()
		return
	}
	select {
	case <-ctx.Done():
	case <-time.After(b.NextBackOff()):
	}
}
