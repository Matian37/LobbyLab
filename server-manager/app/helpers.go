package app

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v6"
)

// HandleBackoff resets the backoff if the error is nil; otherwise it waits for
// the value of the next backoff interval.
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
