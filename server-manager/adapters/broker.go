package adapters

import (
	"context"
	"errors"
	"server-manager/internal"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	healthSubject    = "workers.health"
	assignSubject    = "workers.assign"
	resultSubject    = "workers.results"
	finishSubject    = "workers.finish"
	resultStreamName = "RESULT"
	finishStreamName = "FINISH"
)
const pongBufferSize = 4096

var (
	ErrNATSConnCannotBeReopened = errors.New("connection cannot be reopened")
	ErrNATSConnNotOpen          = errors.New("connection not open")
	ErrNATSConnClosed           = errors.New("connection closed")
	ErrNATSConnAlreadyOpen      = errors.New("connection already open")
	ErrNATSConnAlreadyClosed    = errors.New("connection already closed")
)

type NATSConnection struct {
	conn *nats.Conn
	js   *jetstream.JetStream

	finishConsumer jetstream.Consumer
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
		return ErrNATSConnCannotBeReopened
	}
	if nc.opened {
		return ErrNATSConnAlreadyOpen
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

	resultStream, err := js.CreateOrUpdateStream(
		ctx,
		jetstream.StreamConfig{
			Name:        resultStreamName,
			Subjects:    []string{resultSubject},
			Retention:   jetstream.LimitsPolicy,
			Storage:     jetstream.FileStorage,
			Replicas:    1,
			Compression: jetstream.S2Compression,
		},
	)
	if err != nil {
		return err
	}

	consumer, err := resultStream.CreateConsumer(ctx, jetstream.ConsumerConfig{
		DeliverPolicy: jetstream.DeliverAllPolicy,
	})
	if err != nil {
		return err
	}
	nc.resultConsumer = consumer

	finishStream, err := js.CreateOrUpdateStream(
		ctx,
		jetstream.StreamConfig{
			Name:        finishStreamName,
			Subjects:    []string{finishSubject},
			Retention:   jetstream.LimitsPolicy,
			Storage:     jetstream.MemoryStorage,
			Replicas:    1,
			Compression: jetstream.NoCompression,
			MaxAge:      5 * time.Minute,
			MaxMsgs:     -1,
			MaxBytes:    -1,
			Discard:     jetstream.DiscardOld,
		},
	)
	if err != nil {
		return err
	}

	// purge any leftover messages from a previous run if manager crashed
	if err := finishStream.Purge(ctx); err != nil {
		return err
	}

	consumer, err = finishStream.CreateConsumer(ctx, jetstream.ConsumerConfig{
		DeliverPolicy: jetstream.DeliverAllPolicy,
		AckPolicy:     jetstream.AckNonePolicy,
	})
	if err != nil {
		return err
	}
	nc.finishConsumer = consumer

	nc.opened = true
	return nil
}

func (nc *NATSConnection) AssignJob(ctx context.Context, workerID string, config string) error {
	if !nc.opened {
		return ErrNATSConnNotOpen
	}
	if nc.closed {
		return ErrNATSConnClosed
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

// TODO: make pongTimeout as func argument + ctx
func (nc *NATSConnection) SendPing() (internal.Responders, error) {
	if !nc.opened {
		return nil, ErrNATSConnNotOpen
	}
	if nc.closed {
		return nil, ErrNATSConnClosed
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
		return nil, ErrNATSConnNotOpen
	}
	if nc.closed {
		return nil, ErrNATSConnClosed
	}

	msg, err := nc.resultConsumer.Next(jetstream.FetchContext(ctx))
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// NOTE: function does not validate returned worker id
func (nc *NATSConnection) GetFinish(ctx context.Context) (string, error) {
	if !nc.opened {
		return "", ErrNATSConnNotOpen
	}
	if nc.closed {
		return "", ErrNATSConnClosed
	}

	msg, err := nc.finishConsumer.Next(jetstream.FetchContext(ctx))
	if err != nil {
		return "", err
	}
	return string(msg.Data()), nil
}

func (nc *NATSConnection) Close() error {
	// c.conn == nil allows partialy opened NATSConn to be closed
	if !nc.opened && nc.conn == nil {
		return ErrNATSConnNotOpen
	}
	if nc.closed {
		return ErrNATSConnAlreadyClosed
	}
	nc.closed = true
	return nc.conn.Drain()
}
