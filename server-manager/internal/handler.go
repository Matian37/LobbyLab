package internal

import (
	"context"
	"errors"
	"log/slog"
)

// ContextErrorHandler wraps a slog.Handler and sets the log level to DEBUG for
// every record whose "error" attribute equals context.Canceled or
// context.DeadlineExceeded. This gets rid of the need to add if statements to
// avoid logging context errors in production modes.
type ContextErrorHandler struct {
	handler slog.Handler
}

// NewContextErrorHandler wraps next in a ContextErrorHandler.
func NewContextErrorHandler(next slog.Handler) ContextErrorHandler {
	return ContextErrorHandler{handler: next}
}

// Handle logs the record, downgrading context-related errors to DEBUG level.
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

// Enabled calls the wrapped handler's Enabled method.
func (h ContextErrorHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

// WithAttrs returns a ContextErrorHandler whose wrapped handler has the
// additional attributes.
func (h ContextErrorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return ContextErrorHandler{handler: h.handler.WithAttrs(attrs)}
}

// WithGroup returns a ContextErrorHandler whose wrapped handler is grouped.
func (h ContextErrorHandler) WithGroup(name string) slog.Handler {
	return ContextErrorHandler{handler: h.handler.WithGroup(name)}
}
