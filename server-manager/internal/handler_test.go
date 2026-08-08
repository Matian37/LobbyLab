package internal

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_Handle(t *testing.T) {
	t.Run("suppress context errors", func(t *testing.T) {
		tests := []struct {
			name   string
			attrs  []slog.Attr
			writes bool
		}{
			{
				name:   "context canceled",
				attrs:  []slog.Attr{slog.Any("error", context.Canceled)},
				writes: false,
			},
			{
				name:   "deadline exceeded",
				attrs:  []slog.Attr{slog.Any("error", context.DeadlineExceeded)},
				writes: false,
			},
			{
				name:   "regular error",
				attrs:  []slog.Attr{slog.Any("error", errors.New("oops"))},
				writes: true,
			},
			{
				name:   "no error key",
				attrs:  []slog.Attr{slog.String("foo", "bar")},
				writes: true,
			},
			{
				name:   "error key string value",
				attrs:  []slog.Attr{slog.String("error", "not-an-error")},
				writes: true,
			},
			{
				name:   "nil error value",
				attrs:  []slog.Attr{slog.Any("error", nil)},
				writes: true,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				var buf bytes.Buffer
				h := NewContextErrorHandler(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

				r := slog.NewRecord(time.Time{}, slog.LevelError, "msg", 0)
				r.AddAttrs(tc.attrs...)
				assert.NoError(t, h.Handle(context.Background(), r))

				if tc.writes {
					assert.NotEmpty(t, buf.String())
				} else {
					assert.Empty(t, buf.String())
				}
			})
		}
	})

	t.Run("downgrade to debug", func(t *testing.T) {
		var buf bytes.Buffer
		h := NewContextErrorHandler(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

		r := slog.NewRecord(time.Time{}, slog.LevelError, "msg", 0)
		r.AddAttrs(slog.Any("error", context.Canceled))
		assert.NoError(t, h.Handle(context.Background(), r))
		assert.Contains(t, buf.String(), "level=DEBUG")
	})
}
