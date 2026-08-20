package app

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestRun(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	t.Run("success on ctx cancel", func(t *testing.T) {
		mockConn, mockSrv, server := newMockServer(t)
		mockConn.EXPECT().Open(server.initTimeout).Return(nil)
		mockSrv.EXPECT().Stop(gomock.Any()).Return(nil)
		mockConn.EXPECT().Close().Return(nil)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		assert.NoError(t, run(ctx, server, logger))
	})

	t.Run("open error", func(t *testing.T) {
		mockConn, _, server := newMockServer(t)
		expectedErr := errors.New("")
		mockConn.EXPECT().Open(server.initTimeout).Return(expectedErr)

		assert.ErrorIs(t, run(context.Background(), server, logger), expectedErr)
	})

	t.Run("run error", func(t *testing.T) {
		mockConn, mockSrv, server := newMockServer(t)
		mockConn.EXPECT().Open(server.initTimeout).Return(nil)
		mockSrv.EXPECT().Stop(gomock.Any()).Return(nil)
		mockConn.EXPECT().Close().Return(nil)

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		defer cancel()

		assert.ErrorIs(t, run(ctx, server, logger), context.DeadlineExceeded)
	})
}
