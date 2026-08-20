package adapters

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
	ErrConnectionAlreadyOpen   = errors.New("connection already opened")
	ErrConnectionAlreadyClosed = errors.New("connection already closed")
)

type NATSConnection struct {
	brokerURI   string
	containerID string
	logger      *slog.Logger

	conn *nats.Conn
	js   jetstream.JetStream

	requestSub *nats.Subscription

	opened bool
	closed bool
}

func NewConnection(brokerURI string, containerID string, logger *slog.Logger) *NATSConnection {
	return &NATSConnection{
		brokerURI:   brokerURI,
		containerID: containerID,
		logger:      logger.With("component", "broker"),
	}
}

func (c *NATSConnection) Open(timeout time.Duration) error {
	if c.closed {
		return ErrConnectionAlreadyClosed
	}
	if c.opened {
		return ErrConnectionAlreadyOpen
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := nats.Connect(c.brokerURI, nats.Timeout(timeout))
	if err != nil {
		return err
	}
	c.conn = conn

	js, err := jetstream.New(conn)
	if err != nil {
		return err
	}
	c.js = js

	// checks if the result subject stream exists
	if _, err := c.js.StreamNameBySubject(ctx, resultSubject); err != nil {
		return err
	}

	if err := c.subscribeAssign(); err != nil {
		return err
	}
	if err := c.subscribeHealth(); err != nil {
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
		return ErrConnectionAlreadyClosed
	}
	c.closed = true
	return c.conn.Drain()
}

func (c *NATSConnection) GetMatchConfig(ctx context.Context) (internal.MatchConfig, error) {
	if !c.opened {
		return internal.MatchConfig{}, ErrConnectionNotOpen
	}
	if c.closed {
		return internal.MatchConfig{}, ErrConnectionAlreadyClosed
	}

	msg, err := c.requestSub.NextMsgWithContext(ctx)
	if err != nil {
		return internal.MatchConfig{}, err
	}

	var matchConfig internal.MatchConfig
	if err := json.Unmarshal(msg.Data, &matchConfig); err != nil {
		return internal.MatchConfig{}, err
	}

	// send request acknowledge
	if err := c.conn.Publish(msg.Reply, []byte{}); err != nil {
		return internal.MatchConfig{}, err
	}
	return matchConfig, nil
}

func (c *NATSConnection) SendCancel(ctx context.Context, matchID int) error {
	if !c.opened {
		return ErrConnectionNotOpen
	}
	if c.closed {
		return ErrConnectionAlreadyClosed
	}

	payload, err := json.Marshal(internal.Result{
		Success: false,
		MatchID: matchID,
		Details: []byte(`{}`),
	})
	if err != nil {
		return errors.New("failed to marshal cancel result")
	}

	_, err = c.js.Publish(ctx, resultSubject, payload)
	return err
}

func (c *NATSConnection) SendResult(ctx context.Context, matchID int, result []byte) error {
	if !c.opened {
		return ErrConnectionNotOpen
	}
	if c.closed {
		return ErrConnectionAlreadyClosed
	}

	payload, err := json.Marshal(internal.Result{
		Success: true,
		MatchID: matchID,
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
		return err
	}
	if err := sub.SetPendingLimits(1, -1); err != nil {
		return err
	}
	c.requestSub = sub

	return nil
}

func (c *NATSConnection) subscribeHealth() error {
	sub, err := c.conn.Subscribe(healthSubject, func(msg *nats.Msg) {
		c.logger.Debug("received ping, sending pong...")

		if err := c.conn.Publish(msg.Reply, []byte(c.containerID)); err != nil {
			c.logger.Error("failed to publish pong", "error", err)
		} else {
			c.logger.Debug("pong sent successfuly")
		}
	})
	if err != nil {
		return err
	}
	return sub.SetPendingLimits(1, -1)
}
