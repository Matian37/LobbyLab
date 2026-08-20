package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"server/internal"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSetupLogger(t *testing.T) {
	cfg := internal.Config{Hostname: "test-host", LogLevel: slog.LevelInfo}

	originalLogger := slog.Default()
	defer slog.SetDefault(originalLogger)

	var buf bytes.Buffer
	setupLoggerWithWriter(&buf, cfg)

	assert.NotSame(t, originalLogger, slog.Default())

	slog.Info("test message", "extra_key", "extra_val")

	var parsed map[string]any
	err := json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)

	assert.Equal(t, "test-host", parsed["hostname"])
	assert.Equal(t, "test message", parsed["msg"])
	assert.Equal(t, "extra_val", parsed["extra_key"])
}

func TestRun(t *testing.T) {
	t.Run("success on ctx cancel", func(t *testing.T) {
		mockConn, mockSrv, server := newMockServer(t)
		mockConn.EXPECT().Open(server.initTimeout).Return(nil)
		mockSrv.EXPECT().Stop(gomock.Any()).Return(nil)
		mockConn.EXPECT().Close().Return(nil)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		assert.NoError(t, run(ctx, server))
	})

	t.Run("open error", func(t *testing.T) {
		mockConn, _, server := newMockServer(t)
		expectedErr := errors.New("")
		mockConn.EXPECT().Open(server.initTimeout).Return(expectedErr)

		assert.ErrorIs(t, run(context.Background(), server), expectedErr)
	})

	t.Run("run error", func(t *testing.T) {
		mockConn, mockSrv, server := newMockServer(t)
		mockConn.EXPECT().Open(server.initTimeout).Return(nil)
		mockSrv.EXPECT().Stop(gomock.Any()).Return(nil)
		mockConn.EXPECT().Close().Return(nil)

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		defer cancel()

		assert.ErrorIs(t, run(ctx, server), context.DeadlineExceeded)
	})
}
