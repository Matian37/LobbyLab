package main

import (
	"context"
	"errors"
	"fmt"

	rmq "github.com/rabbitmq/rabbitmq-amqp-go-client/pkg/rabbitmqamqp"
)

//go:generate mockgen -source=internal/broker/broker.go -destination=mocks/conn.go -package=mock

const statusQueueName = "server.status"
const resultQueueName = "server.result"

var (
	ErrConnectionNotInitialized = errors.New("conection not initialized")
	ErrSecondConnectAttempt     = errors.New("cannot connect twice")
	ErrConnectionCreate         = errors.New("failed to create a new connection")
	ErrQueueDeclare             = errors.New("failed to declare a queue")
	ErrConsumerCreate           = errors.New("failed to create a consumer")
	ErrPublisherCreate          = errors.New("failed to create a publisher")
	ErrMessageReceiveFailure    = errors.New("Failed to receive a message:")
)

// NOTE: Connection must be used only once to connect to rabbitmq
type RMQConnection struct {
	brokerUri       string
	env             *rmq.Environment
	conn            *rmq.AmqpConnection
	statusConsumer  *rmq.Consumer
	resultPublisher *rmq.Publisher
}

func GenBrokerUri(user string, pass string) string {
	return fmt.Sprintf("amqp://%s:%s@rabbitmq:5672/", user, pass)
}

func NewConnection(brokerUri string) *RMQConnection {
	return &RMQConnection{
		brokerUri: brokerUri,
		env:       rmq.NewEnvironment(brokerUri, nil),
	}
}

func (c *RMQConnection) Connect(ctx context.Context) error {
	if c.conn != nil {
		return ErrSecondConnectAttempt
	}

	conn, err := c.env.NewConnection(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrConnectionCreate, err)
	}
	c.conn = conn

	_, err = c.conn.Management().DeclareQueue(
		ctx,
		&rmq.QuorumQueueSpecification{Name: statusQueueName},
	)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrQueueDeclare, err)
	}

	consumer, err := c.conn.NewConsumer(
		ctx,
		statusQueueName,
		&rmq.ConsumerOptions{InitialCredits: 1},
	)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrConsumerCreate, err)
	}
	c.statusConsumer = consumer

	_, err = c.conn.Management().DeclareQueue(
		ctx,
		&rmq.QuorumQueueSpecification{Name: resultQueueName},
	)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrQueueDeclare, err)
	}

	publisher, err := c.conn.NewPublisher(ctx, &rmq.QueueAddress{Queue: resultQueueName}, nil)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPublisherCreate, err)
	}
	c.resultPublisher = publisher

	return nil
}

func (c *RMQConnection) Close(ctx context.Context) error {
	if c.conn == nil {
		return ErrConnectionNotInitialized
	}
	return c.env.CloseConnections(ctx)
}

func (c *RMQConnection) GetStartRequest(ctx context.Context) ([]byte, error) {
	if c.statusConsumer == nil {
		return []byte{}, ErrConnectionNotInitialized
	}

	delivery, err := c.statusConsumer.Receive(ctx)
	if err != nil {
		return []byte{}, fmt.Errorf("%w: %w", ErrMessageReceiveFailure, err)
	}

	msg := delivery.Message()

	var payload []byte
	if len(msg.Data) > 0 {
		payload = msg.Data[0]
	}

	// NOTE: can cause issues if something breaks before server is alive
	if err = delivery.Accept(ctx); err != nil {
		return []byte{}, err
	}
	return payload, nil
}

func (c *RMQConnection) SendMatchResult(ctx context.Context, payload []byte) error {
	if c.resultPublisher == nil {
		return ErrConnectionNotInitialized
	}
	_, err := c.resultPublisher.Publish(ctx, rmq.NewMessage(payload))
	return err
}
