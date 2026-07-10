package internal

import (
	"context"
	"errors"
	"log/slog"
)

type ContextErrorHandler struct {
	handler slog.Handler
}

func NewContextErrorHandler(next slog.Handler) ContextErrorHandler {
	return ContextErrorHandler{handler: next}
}

func (h ContextErrorHandler) Handle(ctx context.Context, r slog.Record) error {
	var isContextErr bool

	r.Attrs(func(a slog.Attr) bool {
		if a.Key != "error" {
			return true
		}

		err, ok := a.Value.Any().(error)
		if !ok {
			return true
		}

		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			isContextErr = true
			return false
		}
		return true
	})

	if isContextErr {
		r.Level = slog.LevelDebug
		if !h.handler.Enabled(ctx, slog.LevelDebug) {
			return nil
		}
	}

	return h.handler.Handle(ctx, r)
}

func (h ContextErrorHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h ContextErrorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return ContextErrorHandler{handler: h.handler.WithAttrs(attrs)}
}

func (h ContextErrorHandler) WithGroup(name string) slog.Handler {
	return ContextErrorHandler{handler: h.handler.WithGroup(name)}
}
