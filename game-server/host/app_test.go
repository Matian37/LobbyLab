package main

import (
	"context"
	"errors"
	"server/internal/mocks"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

//go:generate go run go.uber.org/mock/mockgen -source=internal/domain/interfaces.go -destination=internal/mocks/mocks.go -package=mocks

func TestNewApp(t *testing.T) {
	brokerUri := "amqp://user:pass@localhost:5672/"
	cmdArgs := []string{"./game", "--arg1"}
	app := NewApp(brokerUri, cmdArgs)

	assert.NotNil(t, app.conn)
	assert.NotNil(t, app.server)
	assert.Equal(t, cmdArgs, app.cmdArgs)
	assert.False(t, app.initialized)
}

func TestApp_Init(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		mockConn := mocks.NewMockBrokerConnection(ctrl)
		app := App{conn: mockConn}
		ctx := context.Background()

		mockConn.EXPECT().Connect(ctx).Return(nil)

		err := app.Init(ctx)
		assert.NoError(t, err)
		assert.True(t, app.initialized)
	})

	t.Run("app already initialized", func(t *testing.T) {
		app := App{initialized: true}
		err := app.Init(context.Background())
		assert.ErrorIs(t, err, ErrAppAlreadyInitialized)
	})

	t.Run("connection failure", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		mockConn := mocks.NewMockBrokerConnection(ctrl)
		app := App{conn: mockConn}
		ctx := context.Background()
		expectedErr := errors.New("connection failed")

		mockConn.EXPECT().Connect(ctx).Return(expectedErr)

		err := app.Init(ctx)
		assert.ErrorIs(t, err, expectedErr)
		assert.False(t, app.initialized)
	})

	t.Run("context cancel test", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		mockConn := mocks.NewMockBrokerConnection(ctrl)
		app := App{conn: mockConn}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		mockConn.EXPECT().Connect(ctx).Return(context.Canceled)

		err := app.Init(ctx)
		assert.ErrorIs(t, err, context.Canceled)
		assert.False(t, app.initialized)
	})
}

func TestApp_Run(t *testing.T) {
	t.Run("app not initialized", func(t *testing.T) {
		app := App{}
		err := app.Run(context.Background())
		assert.ErrorIs(t, err, ErrAppNotInitialized)
	})

	t.Run("context cancel test", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		app := App{conn: mocks.NewMockBrokerConnection(ctrl), initialized: true}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := app.Run(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("GetStartRequest failure", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		mockConn := mocks.NewMockBrokerConnection(ctrl)
		app := App{conn: mockConn, initialized: true}
		ctx := context.Background()
		expectedErr := errors.New("get request failed")

		mockConn.EXPECT().GetStartRequest(ctx).Return([]byte{}, expectedErr)

		err := app.Run(ctx)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("server start failure", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		mockConn := mocks.NewMockBrokerConnection(ctrl)
		mockServer := mocks.NewMockServer(ctrl)
		app := App{
			conn:        mockConn,
			server:      mockServer,
			initialized: true,
		}
		ctx, cancel := context.WithCancel(context.Background())

		config := "{}"
		mockConn.EXPECT().GetStartRequest(ctx).Return([]byte(config), nil)
		mockServer.EXPECT().Start(config, app.cmdArgs).Return(errors.New("start failed"))
		mockServer.EXPECT().Stop(ctx).Do(func(ctx context.Context) { cancel() }).Return(nil)

		err := app.Run(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("GetResult failure", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		mockConn := mocks.NewMockBrokerConnection(ctrl)
		mockServer := mocks.NewMockServer(ctrl)
		app := App{
			conn:        mockConn,
			server:      mockServer,
			initialized: true,
		}
		ctx, cancel := context.WithCancel(context.Background())

		config := "{}"
		mockConn.EXPECT().GetStartRequest(ctx).Return([]byte(config), nil)
		mockServer.EXPECT().Start(config, app.cmdArgs).Return(nil)
		mockServer.EXPECT().
			GetResult(ctx).
			Do(func(ctx context.Context) { cancel() }).
			Return([]byte{}, errors.New("get result failed"))
		mockServer.EXPECT().Stop(ctx).Return(nil)

		err := app.Run(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("send result failure", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		mockConn := mocks.NewMockBrokerConnection(ctrl)
		mockServer := mocks.NewMockServer(ctrl)
		app := App{
			conn:        mockConn,
			server:      mockServer,
			initialized: true,
		}
		ctx, cancel := context.WithCancel(context.Background())

		config := "config"
		result := []byte("result")

		mockConn.EXPECT().GetStartRequest(ctx).Return([]byte(config), nil)
		mockServer.EXPECT().Start(config, app.cmdArgs).Return(nil)
		mockServer.EXPECT().GetResult(ctx).Return(result, nil)
		mockServer.EXPECT().Stop(ctx).Return(nil)
		mockConn.EXPECT().SendMatchResult(ctx, result).
			Do(func(ctx context.Context, result []byte) { cancel() }).
			Return(errors.New("send failed"))

		err := app.Run(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		mockConn := mocks.NewMockBrokerConnection(ctrl)
		mockServer := mocks.NewMockServer(ctrl)
		app := App{
			conn:        mockConn,
			server:      mockServer,
			initialized: true,
		}
		ctx, cancel := context.WithCancel(context.Background())

		payload := "config"
		result := []byte("result")

		mockConn.EXPECT().GetStartRequest(ctx).Return([]byte(payload), nil)
		mockServer.EXPECT().Start(payload, app.cmdArgs).Return(nil)
		mockServer.EXPECT().GetResult(ctx).Return(result, nil)
		mockServer.EXPECT().Stop(ctx).Return(nil)
		mockConn.EXPECT().SendMatchResult(ctx, result).Return(nil)
		mockConn.EXPECT().GetStartRequest(ctx).
			Do(func(ctx context.Context) { cancel() }).
			Return([]byte{}, context.Canceled)

		err := app.Run(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})
}
