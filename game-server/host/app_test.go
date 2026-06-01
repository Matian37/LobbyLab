package main

import (
	"context"
	"errors"
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
		conn:    mockConn,
		server:  mockSrv,
		cmdArgs: []string{"./game"},
	}
	return mockConn, mockSrv, app
}

func newMockAppWithInit(t *testing.T) (*mocks.MockBrokerConnection, *mocks.MockServer, *App) {
	mockConn, mockSrv, app := newMockApp(t)
	app.initialized = true
	return mockConn, mockSrv, app
}

func TestNewApp(t *testing.T) {
	cmdArgs := []string{"./game", "--arg1"}
	brokerUri := "nats://localhost:4222"
	containerId := "worker-1"

	app := NewApp(brokerUri, containerId, cmdArgs)

	assert.NotNil(t, app.conn)
	assert.NotNil(t, app.server)
	assert.Equal(t, cmdArgs, app.cmdArgs)
	assert.False(t, app.initialized)

	nc, ok := app.conn.(*NATSConnection)
	assert.True(t, ok, "NewApp should create a NATSConnection")
	assert.Equal(t, brokerUri, nc.brokerUri)
	assert.Equal(t, containerId, nc.containerId)
}

func TestApp_Init(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockConn, _, app := newMockApp(t)
		mockConn.EXPECT().Connect(gomock.Any()).Return(nil)

		err := app.Init(150 * time.Millisecond)
		assert.NoError(t, err)
		assert.True(t, app.initialized)
	})

	t.Run("already initialized", func(t *testing.T) {
		app := App{initialized: true}
		err := app.Init(150 * time.Millisecond)
		assert.ErrorIs(t, err, ErrAppAlreadyInitialized)
	})

	t.Run("connection error", func(t *testing.T) {
		mockConn, _, app := newMockApp(t)
		expectedErr := errors.New("nats connection failed")
		mockConn.EXPECT().Connect(gomock.Any()).Return(expectedErr)

		err := app.Init(150 * time.Millisecond)
		assert.ErrorIs(t, err, expectedErr)
		assert.False(t, app.initialized)
	})
}

func TestApp_Run(t *testing.T) {
	t.Run("not initialized", func(t *testing.T) {
		app := App{}
		err := app.Run(context.Background())
		assert.ErrorIs(t, err, ErrAppNotInitialized)
	})

	t.Run("context cancelled", func(t *testing.T) {
		_, _, app := newMockAppWithInit(t)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := app.Run(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("successful match cycle", func(t *testing.T) {
		mockConn, mockSrv, app := newMockAppWithInit(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		config := `{"config":123}`
		result := []byte(`{"result":345}`)

		mockConn.EXPECT().GetMatchConfig(ctx).Return(config, nil)
		mockSrv.EXPECT().Start(config, app.cmdArgs).Return(nil)
		mockSrv.EXPECT().GetResult(ctx).Return(result, nil)
		mockConn.EXPECT().SendResult(result).Return(nil)

		mockConn.EXPECT().GetMatchConfig(ctx).
			DoAndReturn(func(_ context.Context) (string, error) {
				cancel()
				return "", context.Canceled
			})

		err := app.Run(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("GetMatchConfig error skip request", func(t *testing.T) {
		mockConn, _, app := newMockAppWithInit(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mockConn.EXPECT().GetMatchConfig(gomock.Any()).
			DoAndReturn(func(_ context.Context) (string, error) {
				cancel()
				return "", context.Canceled
			})

		err := app.Run(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("server Start error skip request", func(t *testing.T) {
		mockConn, mockSrv, app := newMockAppWithInit(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		config := `{"config":123}`

		mockConn.EXPECT().GetMatchConfig(gomock.Any()).Return(config, nil)
		mockSrv.EXPECT().Start(config, app.cmdArgs).Return(errors.New("binary not found"))
		mockConn.EXPECT().SendCancel().Return(nil)

		mockConn.EXPECT().GetMatchConfig(gomock.Any()).
			DoAndReturn(func(_ context.Context) (string, error) {
				cancel()
				return "", context.Canceled
			})

		err := app.Run(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("server.GetResult error skip request", func(t *testing.T) {
		mockConn, mockSrv, app := newMockAppWithInit(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		config := `{"config":123}`

		mockConn.EXPECT().GetMatchConfig(gomock.Any()).Return(config, nil)
		mockSrv.EXPECT().Start(config, app.cmdArgs).Return(nil)
		mockSrv.EXPECT().GetResult(ctx).Return(nil, errors.New("server crash"))
		mockConn.EXPECT().SendCancel().Return(nil)

		mockConn.EXPECT().GetMatchConfig(gomock.Any()).
			DoAndReturn(func(_ context.Context) (string, error) {
				cancel()
				return "", context.Canceled
			})

		err := app.Run(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})
}
