package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"server/internal"
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

type NATSConnection struct {
	brokerURI   string
	containerID string

	conn *nats.Conn
	js   jetstream.JetStream

	healthSub  *nats.Subscription
	requestSub *nats.Subscription

	opened bool
	closed bool
}

func NewConnection(brokerURI string, containerID string) *NATSConnection {
	return &NATSConnection{
		brokerURI:   brokerURI,
		containerID: containerID,
	}
}

func (c *NATSConnection) Open(timeout time.Duration) error {
	if c.opened {
		return ErrConnectionNotReopenable
	}

	// check whether the result stream exists
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := nats.Connect(c.brokerURI, nats.Timeout(timeout))
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

func (c *NATSConnection) GetMatchConfig(ctx context.Context) (internal.MatchConfig, error) {
	if !c.opened {
		return internal.MatchConfig{}, ErrConnectionNotOpen
	}
	if c.closed {
		return internal.MatchConfig{}, ErrConnectionClosed
	}

	msg, err := c.requestSub.NextMsgWithContext(ctx)
	if err != nil {
		return internal.MatchConfig{}, err
	}

	var matchConfig internal.MatchConfig
	if err := json.Unmarshal(msg.Data, &matchConfig); err != nil {
		return internal.MatchConfig{}, err
	}

	// acknowledge request
	if err := c.conn.Publish(msg.Reply, []byte{}); err != nil {
		return internal.MatchConfig{}, err
	}
	return matchConfig, nil
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
	sub, err := c.conn.SubscribeSync(assignSubject + "." + c.containerID)
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

		if pongErr := c.conn.Publish(msg.Reply, []byte(c.containerID)); pongErr != nil {
			slog.Error("failed to publish health response", "error", pongErr)
		} else {
			slog.Debug("pong sent successfuly")
		}
	})
	if err != nil {
		return err
	}

	if err := sub.SetPendingLimits(1, -1); err != nil {
		return err
	}
	c.healthSub = sub

	return nil
}
