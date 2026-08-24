package app

import (
	"context"
	"errors"
	"github.com/Matian37/multiplayer-asset/game-server/internal"
	"github.com/Matian37/multiplayer-asset/game-server/internal/mocks"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func newMockServer(t *testing.T) (*mocks.MockBrokerConnection, *mocks.MockExecutor, *Server) {
	t.Helper()
	ctrl := gomock.NewController(t)
	mockConn := mocks.NewMockBrokerConnection(ctrl)
	mockSrv := mocks.NewMockExecutor(ctrl)
	server := &Server{
		broker:            mockConn,
		executor:          mockSrv,
		cmdArgs:           []string{"./game"},
		logger:            slog.New(slog.DiscardHandler),
		initTimeout:       150 * time.Millisecond,
		serverStopTimeout: 150 * time.Millisecond,
		sendResultTimeout: 150 * time.Millisecond,
		sendCancelTimeout: 150 * time.Millisecond,
	}
	return mockConn, mockSrv, server
}

func newMockServerWithOpen(t *testing.T) (*mocks.MockBrokerConnection, *mocks.MockExecutor, *Server) {
	mockConn, mockSrv, server := newMockServer(t)
	server.opened = true
	return mockConn, mockSrv, server
}

// newUniqueCtx returns a cancellable context distinct from background/todo,
// useful as a gomock matcher to verify the exact context flows through.
func newUniqueCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}

func TestNewServer(t *testing.T) {
	cmdArgs := []string{"./game", "--arg1"}
	brokerURI := "nats://localhost:4222"
	workerID := "worker-1"

	server := NewServer(brokerURI, workerID, cmdArgs, slog.New(slog.DiscardHandler))

	assert.NotNil(t, server.broker)
	assert.NotNil(t, server.executor)
	assert.Equal(t, cmdArgs, server.cmdArgs)
	assert.False(t, server.opened)
	assert.False(t, server.closed)
	assert.Equal(t, 5*time.Second, server.serverStopTimeout)
	assert.Equal(t, 15*time.Second, server.sendResultTimeout)
	assert.Equal(t, 5*time.Second, server.sendCancelTimeout)
	assert.NotNil(t, server.broker)
}

func TestServer_Open(t *testing.T) {
	t.Run("already opened", func(t *testing.T) {
		server := Server{opened: true}
		err := server.Open()
		assert.ErrorIs(t, err, ErrServerAlreadyOpened)
	})

	t.Run("already closed", func(t *testing.T) {
		server := Server{closed: true}
		err := server.Open()
		assert.ErrorIs(t, err, ErrServerAlreadyClosed)
	})

	t.Run("success", func(t *testing.T) {
		mockConn, _, server := newMockServer(t)
		mockConn.EXPECT().Open(server.initTimeout).Return(nil)

		err := server.Open()
		assert.NoError(t, err)
		assert.True(t, server.opened)
	})

	t.Run("connection error", func(t *testing.T) {
		mockConn, _, server := newMockServer(t)
		expectedErr := errors.New("")
		mockConn.EXPECT().Open(server.initTimeout).Return(expectedErr)

		err := server.Open()
		assert.ErrorIs(t, err, expectedErr)
		assert.False(t, server.opened)
	})
}

func TestServer_Close(t *testing.T) {
	t.Run("not opened", func(t *testing.T) {
		server := Server{}
		err := server.Close()
		assert.ErrorIs(t, err, ErrServerNotOpened)
	})

	t.Run("already closed", func(t *testing.T) {
		server := Server{opened: true, closed: true}
		err := server.Close()
		assert.ErrorIs(t, err, ErrServerAlreadyClosed)
	})

	t.Run("success", func(t *testing.T) {
		mockConn, mockSrv, server := newMockServerWithOpen(t)
		mockSrv.EXPECT().Stop(gomock.Any()).Return(nil)
		mockConn.EXPECT().Close().Return(nil)

		err := server.Close()
		assert.NoError(t, err)
		assert.True(t, server.closed)
	})

	t.Run("close error", func(t *testing.T) {
		mockConn, mockSrv, server := newMockServerWithOpen(t)
		mockSrv.EXPECT().Stop(gomock.Any()).Return(errors.New(""))
		mockConn.EXPECT().Close().Return(errors.New(""))

		err := server.Close()
		assert.Error(t, err)
		assert.True(t, server.closed)
	})
}

func TestServer_sendResult(t *testing.T) {
	result := []byte(`{"result":345}`)
	matchID := 1

	t.Run("success", func(t *testing.T) {
		mockConn, _, server := newMockServer(t)
		mockConn.EXPECT().SendResult(gomock.Any(), matchID, result).Return(nil)

		err := server.sendResult(context.Background(), matchID, result)
		assert.NoError(t, err)
	})

	t.Run("timeout", func(t *testing.T) {
		mockConn, _, server := newMockServer(t)
		server.sendResultTimeout = 5 * time.Millisecond
		mockConn.EXPECT().
			SendResult(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, _ int, _ []byte) error {
				deadline, ok := ctx.Deadline()
				assert.True(t, ok)
				assert.WithinDuration(t, time.Now().Add(5*time.Millisecond), deadline, 50*time.Millisecond)

				<-ctx.Done()
				return ctx.Err()
			})

		err := server.sendResult(context.Background(), matchID, result)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("canceled by parent context", func(t *testing.T) {
		mockConn, _, server := newMockServer(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		mockConn.EXPECT().
			SendResult(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, _ int, _ []byte) error {
				return ctx.Err()
			})

		err := server.sendResult(ctx, matchID, result)
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestServer_sendCancel(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockConn, _, server := newMockServer(t)
		matchID := 1
		mockConn.EXPECT().SendCancel(gomock.Any(), matchID).Return(nil)

		server.sendCancel(matchID)
	})

	t.Run("timeout", func(t *testing.T) {
		mockConn, _, server := newMockServer(t)
		server.sendCancelTimeout = 5 * time.Millisecond
		mockConn.EXPECT().SendCancel(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ int) error {
			deadline, ok := ctx.Deadline()
			assert.True(t, ok)
			assert.WithinDuration(t, time.Now().Add(5*time.Millisecond), deadline, 50*time.Millisecond)

			<-ctx.Done()
			return ctx.Err()
		})

		server.sendCancel(1)
	})
}

func TestServer_stopServer(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		_, mockSrv, server := newMockServer(t)
		mockSrv.EXPECT().Stop(gomock.Any()).Return(nil)

		server.stopServer(context.Background())
	})

	t.Run("timeout", func(t *testing.T) {
		_, mockSrv, server := newMockServer(t)
		server.serverStopTimeout = 5 * time.Millisecond
		mockSrv.EXPECT().Stop(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
			deadline, ok := ctx.Deadline()
			assert.True(t, ok)
			assert.WithinDuration(t, time.Now().Add(5*time.Millisecond), deadline, 50*time.Millisecond)

			<-ctx.Done()
			return ctx.Err()
		})

		server.stopServer(context.Background())
	})

	t.Run("canceled by parent context", func(t *testing.T) {
		_, mockSrv, server := newMockServer(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		mockSrv.EXPECT().Stop(gomock.Any()).Return(context.Canceled)

		server.stopServer(ctx)
	})
}

func TestServer_runServer(t *testing.T) {
	config := `{"config":123}`

	t.Run("success", func(t *testing.T) {
		_, mockSrv, server := newMockServer(t)
		ctx := newUniqueCtx(t)
		result := []byte(`{"result":345}`)

		mockSrv.EXPECT().Start(config).Return(nil)
		mockSrv.EXPECT().GetResult(ctx).Return(result, nil)
		mockSrv.EXPECT().Stop(gomock.Any()).Return(nil)

		res, err := server.runServer(ctx, config)
		assert.NoError(t, err)
		assert.Equal(t, result, res)
	})

	t.Run("start error", func(t *testing.T) {
		_, mockSrv, server := newMockServer(t)
		mockSrv.EXPECT().Start(config).Return(errors.New(""))

		res, err := server.runServer(context.Background(), config)
		assert.Error(t, err)
		assert.Nil(t, res)
	})

	t.Run("GetResult error", func(t *testing.T) {
		_, mockSrv, server := newMockServer(t)
		ctx := newUniqueCtx(t)

		mockSrv.EXPECT().Start(config).Return(nil)
		mockSrv.EXPECT().GetResult(ctx).Return(nil, errors.New(""))
		mockSrv.EXPECT().Stop(gomock.Any()).Return(nil)

		res, err := server.runServer(ctx, config)
		assert.Error(t, err)
		assert.Nil(t, res)
	})
}

func TestServer_runMatch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockConn, mockSrv, server := newMockServer(t)
		ctx := newUniqueCtx(t)
		config := internal.MatchConfig{MatchID: 1, Config: []byte(`{"config":123}`)}
		result := []byte(`{"result":345}`)

		mockConn.EXPECT().GetMatchConfig(ctx).Return(config, nil)
		mockSrv.EXPECT().Start(string(config.Config)).Return(nil)
		mockSrv.EXPECT().GetResult(ctx).Return(result, nil)
		mockSrv.EXPECT().Stop(gomock.Any()).Return(nil)
		mockConn.EXPECT().SendResult(gomock.Any(), config.MatchID, result).Return(nil)

		err := server.runMatch(ctx)
		assert.NoError(t, err)
	})

	t.Run("GetMatchConfig failed", func(t *testing.T) {
		mockConn, _, server := newMockServer(t)
		ctx := newUniqueCtx(t)
		mockConn.EXPECT().GetMatchConfig(ctx).Return(internal.MatchConfig{}, errors.New(""))

		err := server.runMatch(ctx)
		assert.Error(t, err)
	})

	t.Run("runServer failed", func(t *testing.T) {
		mockConn, mockSrv, server := newMockServer(t)
		ctx := newUniqueCtx(t)

		mockConn.EXPECT().GetMatchConfig(ctx).Return(internal.MatchConfig{}, nil)
		mockSrv.EXPECT().Start(gomock.Any()).Return(errors.New(""))
		mockConn.EXPECT().SendCancel(gomock.Any(), gomock.Any()).Return(nil)

		err := server.runMatch(ctx)
		assert.Error(t, err)
	})

	t.Run("sendResult failed", func(t *testing.T) {
		mockConn, mockSrv, server := newMockServer(t)

		mockConn.EXPECT().GetMatchConfig(gomock.Any()).Return(internal.MatchConfig{}, nil)
		mockSrv.EXPECT().Start(gomock.Any()).Return(nil)
		mockSrv.EXPECT().GetResult(gomock.Any()).Return([]byte{}, nil)
		mockSrv.EXPECT().Stop(gomock.Any()).Return(nil)
		mockConn.EXPECT().SendResult(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New(""))
		mockConn.EXPECT().SendCancel(gomock.Any(), gomock.Any()).Return(nil)

		err := server.runMatch(context.Background())
		assert.Error(t, err)
	})
}

func TestServer_Run(t *testing.T) {
	t.Run("not opened", func(t *testing.T) {
		server := Server{}
		err := server.Run(context.Background())
		assert.ErrorIs(t, err, ErrServerNotOpened)
	})

	t.Run("already closed", func(t *testing.T) {
		server := Server{opened: true, closed: true}
		err := server.Run(context.Background())
		assert.ErrorIs(t, err, ErrServerAlreadyClosed)
	})

	t.Run("context canceled", func(t *testing.T) {
		_, _, server := newMockServerWithOpen(t)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := server.Run(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("successful iterations", func(t *testing.T) {
		mockConn, mockSrv, server := newMockServerWithOpen(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mockConn.EXPECT().GetMatchConfig(ctx).Return(internal.MatchConfig{}, nil)
		mockSrv.EXPECT().Start(gomock.Any()).Return(nil)
		mockSrv.EXPECT().GetResult(ctx).Return([]byte{}, nil)
		mockSrv.EXPECT().Stop(gomock.Any()).Return(nil)
		mockConn.EXPECT().SendResult(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

		mockConn.EXPECT().GetMatchConfig(gomock.Any()).
			DoAndReturn(func(_ context.Context) (internal.MatchConfig, error) {
				cancel()
				return internal.MatchConfig{}, context.Canceled
			})

		err := server.Run(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("continue after error", func(t *testing.T) {
		mockConn, _, server := newMockServerWithOpen(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mockConn.EXPECT().GetMatchConfig(gomock.Any()).
			Return(internal.MatchConfig{}, errors.New(""))
		mockConn.EXPECT().GetMatchConfig(gomock.Any()).
			DoAndReturn(func(_ context.Context) (internal.MatchConfig, error) {
				cancel()
				return internal.MatchConfig{}, context.Canceled
			})

		err := server.Run(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})
}
