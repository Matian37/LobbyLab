package main

import (
	"context"
	"errors"
	"server-manager/internal"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	healthSubject = "workers.health"
	assignSubject = "workers.assign"
	resultSubject = "workers.results"
	finishSubject = "workers.finish"
	resultStream  = "RESULT"
)
const pongBufferSize = 4096

var (
	ErrConnCannotBeReopened = errors.New("connection cannot be reopened")
	ErrConnNotOpen          = errors.New("connection not open")
	ErrConnClosed           = errors.New("connection closed")
	ErrConnAlreadyOpen      = errors.New("connection already open")
	ErrConnAlreadyClosed    = errors.New("connection already closed")
)

type NATSConnection struct {
	conn *nats.Conn
	js   *jetstream.JetStream

	finishSub      *nats.Subscription
	resultConsumer jetstream.Consumer

	openTimeout      time.Duration
	pongTimeout      time.Duration
	assignJobTimeout time.Duration

	opened bool
	closed bool
}

func NewNATSConnection() *NATSConnection {
	return &NATSConnection{
		assignJobTimeout: 5 * time.Second,
		pongTimeout:      3 * time.Second,
		openTimeout:      5 * time.Second,
	}
}

func (nc *NATSConnection) Open(ctx context.Context, config *internal.EnvConfig) error {
	if nc.closed {
		return ErrConnCannotBeReopened
	}
	if nc.opened {
		return ErrConnAlreadyOpen
	}

	conn, err := nats.Connect(config.BrokerURI, nats.Timeout(nc.openTimeout))
	if err != nil {
		return err
	}
	nc.conn = conn

	js, err := jetstream.New(nc.conn)
	if err != nil {
		return err
	}
	nc.js = &js

	stream, err := js.CreateOrUpdateStream(
		ctx,
		jetstream.StreamConfig{
			Name:         resultStream,
			Subjects:     []string{resultSubject},
			Retention:    jetstream.LimitsPolicy,
			MaxConsumers: 1,
			Storage:      jetstream.FileStorage,
			Replicas:     1,
			Compression:  jetstream.S2Compression,
		},
	)
	if err != nil {
		return err
	}

	consumer, err := stream.CreateConsumer(ctx, jetstream.ConsumerConfig{
		DeliverPolicy: jetstream.DeliverAllPolicy,
	})
	if err != nil {
		return err
	}
	nc.resultConsumer = consumer

	sub, err := nc.conn.SubscribeSync(finishSubject)
	if err != nil {
		return err
	}
	nc.finishSub = sub

	nc.opened = true
	return nil
}

func (nc *NATSConnection) AssignJob(ctx context.Context, workerID string, config string) error {
	if !nc.opened {
		return ErrConnNotOpen
	}
	if nc.closed {
		return ErrConnClosed
	}

	timeoutCtx, stop := context.WithTimeout(ctx, nc.assignJobTimeout)
	defer stop()

	_, err := nc.conn.RequestWithContext(
		timeoutCtx,
		assignSubject+"."+workerID,
		[]byte(config),
	)

	return err
}

func (nc *NATSConnection) SendPing() (internal.Responders, error) {
	if !nc.opened {
		return nil, ErrConnNotOpen
	}
	if nc.closed {
		return nil, ErrConnClosed
	}

	inbox := nc.conn.NewInbox()

	responders := make(internal.Responders)

	sub, err := nc.conn.Subscribe(inbox, func(msg *nats.Msg) {
		responders[string(msg.Data)] = struct{}{}
	})
	if err != nil {
		return nil, err
	}
	defer sub.Unsubscribe()

	err = nc.conn.PublishRequest(healthSubject, inbox, nil)
	if err != nil {
		return nil, err
	}

	time.Sleep(nc.pongTimeout)

	return responders, nil
}

func (nc *NATSConnection) GetResult(ctx context.Context) (internal.Message, error) {
	if !nc.opened {
		return nil, ErrConnNotOpen
	}
	if nc.closed {
		return nil, ErrConnClosed
	}

	msg, err := nc.resultConsumer.Next(jetstream.FetchContext(ctx))
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// NOTE: function doesn't guarantee the message is from a valid worker
func (nc *NATSConnection) GetFinish(ctx context.Context) (string, error) {
	if !nc.opened {
		return "", ErrConnNotOpen
	}
	if nc.closed {
		return "", ErrConnClosed
	}

	msg, err := nc.finishSub.NextMsgWithContext(ctx)
	if err != nil {
		return "", err
	}
	return string(msg.Data), nil
}

func (nc *NATSConnection) Close() error {
	// c.conn == nil allows partialy opened NATSConn to be closed
	if !nc.opened && nc.conn == nil {
		return ErrConnNotOpen
	}
	if nc.closed {
		return ErrConnAlreadyClosed
	}
	nc.closed = true
	return nc.conn.Drain()
}
