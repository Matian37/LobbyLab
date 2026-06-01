package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	healthSubject = "workers.health"
	assignSubject = "workers.assign"
	resultSubject = "workers.results"
)

var (
	ErrConnectionNotInitialized     = errors.New("conection not initialized")
	ErrConnectionAlreadyInitialized = errors.New("connection already initialized")
	ErrConnectionAlreadyClosed      = errors.New("connection already closed")
	ErrPingJSONEncodingFailed       = errors.New("ping json enconding failed")
	ErrFailedToConnect              = errors.New("failed to connect")
	ErrChannelError                 = errors.New("channel")
)

// TODO: change connect to open and initialized to opened
// Note: closed connection cannot be reconnected
type NATSConnection struct {
	brokerUri   string
	containerId string
	conn        *nats.Conn

	healthSub  *nats.Subscription
	requestSub *nats.Subscription

	initialized bool
	closed      bool
}

func NewConnection(brokerUri string, containerId string) *NATSConnection {
	return &NATSConnection{
		brokerUri:   brokerUri,
		containerId: containerId,
	}
}

func (c *NATSConnection) Connect(timeout time.Duration) error {
	if c.initialized {
		// TODO: rename this error to cannot be reopened
		return ErrConnectionAlreadyInitialized
	}

	conn, err := nats.Connect(c.brokerUri, nats.Timeout(timeout))
	if err != nil {
		return err
	}
	c.conn = conn

	if err := c.subscribeAssign(); err != nil {
		c.Close()
		return err
	}
	if err := c.subscribeHealth(); err != nil {
		c.Close()
		return err
	}
	c.initialized = true

	return nil
}

func (c *NATSConnection) Close() error {
	if !c.initialized {
		return ErrConnectionNotInitialized
	}
	if c.closed {
		return ErrConnectionAlreadyClosed
	}
	c.closed = true
	return c.conn.Drain()
}

func (c *NATSConnection) GetMatchConfig(ctx context.Context) (string, error) {
	if !c.initialized {
		return "", ErrConnectionNotInitialized
	}
	if c.closed {
		return "", ErrConnectionAlreadyClosed
	}

	msg, err := c.requestSub.NextMsgWithContext(ctx)
	if err != nil {
		return "", err
	}

	// acknowledge request
	if err := c.conn.Publish(msg.Reply, []byte{}); err != nil {
		return "", err
	}
	return string(msg.Data), nil
}

type Result struct {
	Success bool            `json:"success"`
	Details json.RawMessage `json:"details"`
}

func (c *NATSConnection) SendCancel() error {
	if !c.initialized {
		return ErrConnectionNotInitialized
	}
	if c.closed {
		return ErrConnectionAlreadyClosed
	}
	return c.conn.Publish(
		resultSubject,
		[]byte(`{"success": false, "details":{}}`),
	)
}

// TODO: use jetstream here
func (c *NATSConnection) SendResult(result []byte) error {
	if !c.initialized {
		return ErrConnectionNotInitialized
	}
	if c.closed {
		return ErrConnectionAlreadyClosed
	}

	payload, err := json.Marshal(Result{
		Success: true,
		Details: result,
	})
	if err != nil {
		return fmt.Errorf("the result is not valid JSON")
	}

	return c.conn.Publish(resultSubject, payload)
}

func (c *NATSConnection) subscribeAssign() error {
	sub, err := c.conn.SubscribeSync(assignSubject + "." + c.containerId)
	if err != nil {
		c.Close()
		return err
	}
	if err := sub.SetPendingLimits(1, -1); err != nil {
		c.Close()
		return err
	}
	c.requestSub = sub

	return nil
}

func (c *NATSConnection) subscribeHealth() error {
	sub, err := c.conn.Subscribe(healthSubject, func(msg *nats.Msg) {
		slog.Debug("received ping, sending pong...")

		responseErr := c.conn.Publish(healthSubject+"."+c.containerId, []byte{})
		if responseErr != nil {
			slog.Error("failed to publish health response", "error", responseErr)
		}

		slog.Debug("pong sent successfuly...")
	})
	if err != nil {
		return err
	}
	if err := sub.SetPendingLimits(1, -1); err != nil {
		return err
	}
	c.healthSub = sub

	// it doesnt make sense to store more than one ping msg
	return nil
}
