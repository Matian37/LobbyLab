package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/rabbitmq"

	rmq "github.com/rabbitmq/rabbitmq-amqp-go-client/pkg/rabbitmqamqp"
)

const containerImageName = "rabbitmq:4.3-alpine"

var container *rabbitmq.RabbitMQContainer
var brokerUri string

// TODO: tests for contexts

func RestartBroker() {
	if container != nil {
		container.Exec(context.Background(), []string{"rabbitmqctl", "stop_app"})
		container.Exec(context.Background(), []string{"rabbitmqctl", "reset"})
		container.Exec(context.Background(), []string{"rabbitmqctl", "start_app"})
		return
	}

	fmt.Println("Starting container for conn.go...")

	ctx := context.Background()

	cont, err := rabbitmq.Run(ctx, containerImageName)
	if err != nil {
		panic(fmt.Errorf("rabbitmq start failed: %w", err))
	}
	defer func() {
		if err := testcontainers.TerminateContainer(cont); err != nil {
			panic(fmt.Errorf("terminate container failed: %w", err))
		}
	}()
	container = cont

	url, err := container.AmqpURL(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to  get amqp url: %w", err))
	}
	brokerUri = url
}

func TestGenBrokerURI(t *testing.T) {
	assert.Equal(t, "amqp://X:Y@rabbitmq:5672/", GenBrokerUri("X", "Y"))
}

func TestNewConnection(t *testing.T) {
	conn := NewConnection("X")
	assert.Equal(t, "X", conn.brokerUri)
}

func TestConnect(t *testing.T) {
	t.Run("no connection", func(t *testing.T) {
		conn := NewConnection("amqp://X:Y@rabbitmq:5672/")
		err := conn.Connect(context.Background())
		assert.ErrorIs(t, err, ErrConnectionCreate)
	})

	t.Run("success", func(t *testing.T) {
		RestartBroker()

		conn := NewConnection(brokerUri)
		assert.Nil(t, conn.conn)

		assert.NoError(t, conn.Connect(context.Background()))

		assert.NotNil(t, conn.conn)
		assert.NotNil(t, conn.statusConsumer)
		assert.NotNil(t, conn.resultPublisher)

		assert.IsType(t, &rmq.StateOpen{}, conn.conn.State())
	})

	t.Run("second connect attempt", func(t *testing.T) {
		RestartBroker()

		conn := NewConnection(brokerUri)
		assert.NoError(t, conn.Connect(context.Background()))
		assert.ErrorIs(t, conn.Connect(context.Background()), ErrSecondConnectAttempt)
	})
}

func TestClose_NotInitialized(t *testing.T) {
	conn := NewConnection("")
	err := conn.Close(context.Background())
	assert.ErrorIs(t, err, ErrConnectionNotInitialized)
}

func TestClose_Success(t *testing.T) {
	RestartBroker()

	conn := NewConnection(brokerUri)
	err := conn.Connect(context.Background())
	assert.NoError(t, err)

	cha := make(chan *rmq.StateChanged, 1)
	conn.conn.NotifyStatusChange(cha)

	assert.IsType(t, &rmq.StateOpen{}, conn.conn.State())
	conn.Close(context.Background())

	select {
	case change := <-cha:
		assert.IsType(t, &rmq.StateClosed{}, change.To)
	case <-time.After(1 * time.Second):
		assert.Fail(t, "timeout")
	}

}

func TestGetStartRequest_NotInitialized(t *testing.T) {
	conn := NewConnection("")
	_, err := conn.GetStartRequest(context.Background())
	assert.ErrorIs(t, err, ErrConnectionNotInitialized)
}

func TestGetStartRequest(t *testing.T) {
	RestartBroker()

	conn := NewConnection(brokerUri)
	err := conn.Connect(context.Background())
	assert.NoError(t, err)

	tests := []struct {
		name string
		msg  []byte
	}{
		{name: "example", msg: []byte{0, 1, 2}},
		{name: "empty", msg: []byte{}},
	}

	func() {
		ctx := context.Background()

		env := rmq.NewEnvironment(brokerUri, nil)
		conn, err := env.NewConnection(ctx)
		require.NoError(t, err)

		publisher, err := conn.NewPublisher(
			ctx,
			&rmq.QueueAddress{Queue: statusQueueName},
			nil,
		)
		require.NoError(t, err)

		for _, test := range tests {
			_, err = publisher.Publish(ctx, rmq.NewMessage(test.msg))
			require.NoError(t, err)
		}
	}()

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			res, err := conn.GetStartRequest(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, test.msg, res)
		})
	}
}

func TestSendMatchResult_NotInitialized(t *testing.T) {
	conn := NewConnection("")
	err := conn.SendMatchResult(context.Background(), []byte{})
	assert.ErrorIs(t, err, ErrConnectionNotInitialized)
}

func TestSendMatchResult(t *testing.T) {
	RestartBroker()

	conn := NewConnection(brokerUri)
	err := conn.Connect(context.Background())
	require.NoError(t, err)

	err = conn.SendMatchResult(context.Background(), []byte{0, 1, 2})
	assert.NoError(t, err)
}
