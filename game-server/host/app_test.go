package main

import (
	"context"
	"errors"
	"server/internal"
	"server/internal/mocks"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func newMockApp(t *testing.T) (*mocks.MockBrokerConnection, *mocks.MockServer, *App) {
	t.Helper()
	ctrl := gomock.NewController(t)
	mockConn := mocks.NewMockBrokerConnection(ctrl)
	mockSrv := mocks.NewMockServer(ctrl)
	app := &App{
		conn:              mockConn,
		server:            mockSrv,
		cmdArgs:           []string{"./game"},
		initTimeout:       150 * time.Millisecond,
		serverStopTimeout: 150 * time.Millisecond,
		sendResultTimeout: 150 * time.Millisecond,
		sendCancelTimeout: 150 * time.Millisecond,
	}
	return mockConn, mockSrv, app
}

func newMockAppWithInit(t *testing.T) (*mocks.MockBrokerConnection, *mocks.MockServer, *App) {
	mockConn, mockSrv, app := newMockApp(t)
	app.initialized = true
	return mockConn, mockSrv, app
}

// newUniqueCtx returns a cancellable context distinct from background/todo,
// useful as a gomock matcher to verify the exact context flows through.
func newUniqueCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}

func TestNewApp(t *testing.T) {
	cmdArgs := []string{"./game", "--arg1"}
	brokerURI := "nats://localhost:4222"
	workerID := "worker-1"

	app := NewApp(brokerURI, workerID, cmdArgs)

	assert.NotNil(t, app.conn)
	assert.NotNil(t, app.server)
	assert.Equal(t, cmdArgs, app.cmdArgs)
	assert.False(t, app.initialized)
	assert.Equal(t, 5*time.Second, app.serverStopTimeout)
	assert.Equal(t, 15*time.Second, app.sendResultTimeout)
	assert.Equal(t, 5*time.Second, app.sendCancelTimeout)

	nc, ok := app.conn.(*NATSConnection)
	assert.True(t, ok, "NewApp should create a NATSConnection")
	assert.Equal(t, brokerURI, nc.brokerURI)
	assert.Equal(t, workerID, nc.workerID)
}

func TestApp_Init(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockConn, _, app := newMockApp(t)
		mockConn.EXPECT().Open(app.initTimeout).Return(nil)

		err := app.Init()
		assert.NoError(t, err)
		assert.True(t, app.initialized)
	})

	t.Run("already initialized", func(t *testing.T) {
		app := App{initialized: true}
		err := app.Init()
		assert.ErrorIs(t, err, ErrAppAlreadyInitialized)
	})

	t.Run("connection error", func(t *testing.T) {
		mockConn, _, app := newMockApp(t)
		expectedErr := errors.New("")
		mockConn.EXPECT().Open(app.initTimeout).Return(expectedErr)

		err := app.Init()
		assert.ErrorIs(t, err, expectedErr)
		assert.False(t, app.initialized)
	})
}

func TestApp_sendResult(t *testing.T) {
	result := []byte(`{"result":345}`)
	matchID := 1

	t.Run("success", func(t *testing.T) {
		mockConn, _, app := newMockApp(t)
		mockConn.EXPECT().SendResult(gomock.Any(), matchID, result).Return(nil)

		err := app.sendResult(context.Background(), matchID, result)
		assert.NoError(t, err)
	})

	t.Run("timeout", func(t *testing.T) {
		mockConn, _, app := newMockApp(t)
		app.sendResultTimeout = 5 * time.Millisecond
		mockConn.EXPECT().
			SendResult(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, _ int, _ []byte) error {
				deadline, ok := ctx.Deadline()
				assert.True(t, ok)
				assert.WithinDuration(t, time.Now().Add(5*time.Millisecond), deadline, 50*time.Millisecond)

				<-ctx.Done()
				return ctx.Err()
			})

		err := app.sendResult(context.Background(), matchID, result)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("canceled by parent context", func(t *testing.T) {
		mockConn, _, app := newMockApp(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		mockConn.EXPECT().
			SendResult(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, _ int, _ []byte) error {
				return ctx.Err()
			})

		err := app.sendResult(ctx, matchID, result)
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestApp_sendCancel(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockConn, _, app := newMockApp(t)
		matchID := 1
		mockConn.EXPECT().SendCancel(gomock.Any(), matchID).Return(nil)

		app.sendCancel(matchID)
	})

	t.Run("timeout", func(t *testing.T) {
		mockConn, _, app := newMockApp(t)
		app.sendCancelTimeout = 5 * time.Millisecond
		mockConn.EXPECT().SendCancel(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ int) error {
			deadline, ok := ctx.Deadline()
			assert.True(t, ok)
			assert.WithinDuration(t, time.Now().Add(5*time.Millisecond), deadline, 50*time.Millisecond)

			<-ctx.Done()
			return ctx.Err()
		})

		app.sendCancel(1)
	})
}

func TestApp_stopServer(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		_, mockSrv, app := newMockApp(t)
		mockSrv.EXPECT().Stop(gomock.Any()).Return(nil)

		app.stopServer(context.Background())
	})

	t.Run("timeout", func(t *testing.T) {
		_, mockSrv, app := newMockApp(t)
		app.serverStopTimeout = 5 * time.Millisecond
		mockSrv.EXPECT().Stop(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
			deadline, ok := ctx.Deadline()
			assert.True(t, ok)
			assert.WithinDuration(t, time.Now().Add(5*time.Millisecond), deadline, 50*time.Millisecond)

			<-ctx.Done()
			return ctx.Err()
		})

		app.stopServer(context.Background())
	})

	t.Run("canceled by parent context", func(t *testing.T) {
		_, mockSrv, app := newMockApp(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		mockSrv.EXPECT().Stop(gomock.Any()).Return(context.Canceled)

		app.stopServer(ctx)
	})
}

func TestApp_runServer(t *testing.T) {
	config := `{"config":123}`

	t.Run("success", func(t *testing.T) {
		_, mockSrv, app := newMockApp(t)
		ctx := newUniqueCtx(t)
		result := []byte(`{"result":345}`)

		mockSrv.EXPECT().Start(config, app.cmdArgs).Return(nil)
		mockSrv.EXPECT().GetResult(ctx).Return(result, nil)
		mockSrv.EXPECT().Stop(gomock.Any()).Return(nil)

		res, err := app.runServer(ctx, config)
		assert.NoError(t, err)
		assert.Equal(t, result, res)
	})

	t.Run("start error", func(t *testing.T) {
		_, mockSrv, app := newMockApp(t)
		mockSrv.EXPECT().Start(config, app.cmdArgs).Return(errors.New(""))

		res, err := app.runServer(context.Background(), config)
		assert.Error(t, err)
		assert.Nil(t, res)
	})

	t.Run("GetResult error", func(t *testing.T) {
		_, mockSrv, app := newMockApp(t)
		ctx := newUniqueCtx(t)

		mockSrv.EXPECT().Start(config, app.cmdArgs).Return(nil)
		mockSrv.EXPECT().GetResult(ctx).Return(nil, errors.New(""))
		mockSrv.EXPECT().Stop(gomock.Any()).Return(nil)

		res, err := app.runServer(ctx, config)
		assert.Error(t, err)
		assert.Nil(t, res)
	})
}

func TestApp_runMatch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockConn, mockSrv, app := newMockApp(t)
		ctx := newUniqueCtx(t)
		config := internal.MatchConfig{MatchID: 1, Config: []byte(`{"config":123}`)}
		result := []byte(`{"result":345}`)

		mockConn.EXPECT().GetMatchConfig(ctx).Return(config, nil)
		mockSrv.EXPECT().Start(string(config.Config), app.cmdArgs).Return(nil)
		mockSrv.EXPECT().GetResult(ctx).Return(result, nil)
		mockSrv.EXPECT().Stop(gomock.Any()).Return(nil)
		mockConn.EXPECT().SendResult(gomock.Any(), config.MatchID, result).Return(nil)

		err := app.runMatch(ctx)
		assert.NoError(t, err)
	})

	t.Run("GetMatchConfig failed", func(t *testing.T) {
		mockConn, _, app := newMockApp(t)
		ctx := newUniqueCtx(t)
		mockConn.EXPECT().GetMatchConfig(ctx).Return(internal.MatchConfig{}, errors.New(""))

		err := app.runMatch(ctx)
		assert.Error(t, err)
	})

	t.Run("runServer failed", func(t *testing.T) {
		mockConn, mockSrv, app := newMockApp(t)
		ctx := newUniqueCtx(t)

		mockConn.EXPECT().GetMatchConfig(ctx).Return(internal.MatchConfig{}, nil)
		mockSrv.EXPECT().Start(gomock.Any(), app.cmdArgs).Return(errors.New(""))
		mockConn.EXPECT().SendCancel(gomock.Any(), gomock.Any()).Return(nil)

		err := app.runMatch(ctx)
		assert.Error(t, err)
	})

	t.Run("sendResult failed", func(t *testing.T) {
		mockConn, mockSrv, app := newMockApp(t)

		mockConn.EXPECT().GetMatchConfig(gomock.Any()).Return(internal.MatchConfig{}, nil)
		mockSrv.EXPECT().Start(gomock.Any(), app.cmdArgs).Return(nil)
		mockSrv.EXPECT().GetResult(gomock.Any()).Return([]byte{}, nil)
		mockSrv.EXPECT().Stop(gomock.Any()).Return(nil)
		mockConn.EXPECT().SendResult(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New(""))
		mockConn.EXPECT().SendCancel(gomock.Any(), gomock.Any()).Return(nil)

		err := app.runMatch(context.Background())
		assert.Error(t, err)
	})
}

func TestApp_Run(t *testing.T) {
	t.Run("not initialized", func(t *testing.T) {
		app := App{}
		err := app.Run(context.Background())
		assert.ErrorIs(t, err, ErrAppNotInitialized)
	})

	t.Run("context canceled", func(t *testing.T) {
		_, _, app := newMockAppWithInit(t)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := app.Run(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("successful iterations", func(t *testing.T) {
		mockConn, mockSrv, app := newMockAppWithInit(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mockConn.EXPECT().GetMatchConfig(ctx).Return(internal.MatchConfig{}, nil)
		mockSrv.EXPECT().Start(gomock.Any(), app.cmdArgs).Return(nil)
		mockSrv.EXPECT().GetResult(ctx).Return([]byte{}, nil)
		mockSrv.EXPECT().Stop(gomock.Any()).Return(nil)
		mockConn.EXPECT().SendResult(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

		mockConn.EXPECT().GetMatchConfig(gomock.Any()).
			DoAndReturn(func(_ context.Context) (internal.MatchConfig, error) {
				cancel()
				return internal.MatchConfig{}, context.Canceled
			})

		err := app.Run(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("continue after error", func(t *testing.T) {
		mockConn, _, app := newMockAppWithInit(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mockConn.EXPECT().GetMatchConfig(gomock.Any()).
			Return(internal.MatchConfig{}, errors.New(""))
		mockConn.EXPECT().GetMatchConfig(gomock.Any()).
			DoAndReturn(func(_ context.Context) (internal.MatchConfig, error) {
				cancel()
				return internal.MatchConfig{}, context.Canceled
			})

		err := app.Run(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})
}
