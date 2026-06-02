package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	healthSubject = "workers.health"
	assignSubject = "workers.assign"
	resultSubject = "workers.results"
)

var (
	ErrConnectionNotOpen       = errors.New("conection not initialized")
	ErrConnectionNotReopenable = errors.New("connection cannot be reopened")
	ErrConnectionClosed        = errors.New("connection already closed")
)

// Note: closed connection cannot be reopened
type NATSConnection struct {
	brokerUri   string
	containerId string

	conn *nats.Conn
	js   jetstream.JetStream

	healthSub  *nats.Subscription
	requestSub *nats.Subscription

	opened bool
	closed bool
}

func NewConnection(brokerUri string, containerId string) *NATSConnection {
	return &NATSConnection{
		brokerUri:   brokerUri,
		containerId: containerId,
	}
}

func (c *NATSConnection) Open(timeout time.Duration) error {
	if c.opened {
		return ErrConnectionNotReopenable
	}

	conn, err := nats.Connect(c.brokerUri, nats.Timeout(timeout))
	if err != nil {
		return err
	}
	c.conn = conn

	js, err := jetstream.New(conn)
	if err != nil {
		c.Close()
		return err
	}
	c.js = js

	// check whether the result stream exists
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if _, err := c.js.StreamNameBySubject(ctx, resultSubject); err != nil {
		c.Close()
		return err
	}

	if err := c.subscribeAssign(); err != nil {
		c.Close()
		return err
	}
	if err := c.subscribeHealth(); err != nil {
		c.Close()
		return err
	}
	c.opened = true

	return nil
}

func (c *NATSConnection) Close() error {
	// c.conn == nil allows partialy opened NATSConn to be closed
	if !c.opened && c.conn == nil {
		return ErrConnectionNotOpen
	}
	if c.closed {
		return ErrConnectionClosed
	}
	c.closed = true
	return c.conn.Drain()
}

func (c *NATSConnection) GetMatchConfig(ctx context.Context) (string, error) {
	if !c.opened {
		return "", ErrConnectionNotOpen
	}
	if c.closed {
		return "", ErrConnectionClosed
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

func (c *NATSConnection) SendCancel(ctx context.Context) error {
	if !c.opened {
		return ErrConnectionNotOpen
	}
	if c.closed {
		return ErrConnectionClosed
	}
	_, err := c.js.Publish(
		ctx,
		resultSubject,
		[]byte(`{"success": false, "details":{}}`),
	)
	return err
}

func (c *NATSConnection) SendResult(ctx context.Context, result []byte) error {
	if !c.opened {
		return ErrConnectionNotOpen
	}
	if c.closed {
		return ErrConnectionClosed
	}

	payload, err := json.Marshal(Result{
		Success: true,
		Details: result,
	})
	if err != nil {
		return fmt.Errorf("the result is not valid JSON: %w", err)
	}

	_, err = c.js.Publish(ctx, resultSubject, payload)
	return err
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
