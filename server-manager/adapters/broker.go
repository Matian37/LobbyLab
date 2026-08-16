package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"server-manager/internal"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	healthSubject    = "workers.health"
	assignSubject    = "workers.assign"
	resultSubject    = "workers.results"
	resultStreamName = "RESULT"
)

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

	resultConsumer jetstream.Consumer

	openTimeout      time.Duration
	assignJobTimeout time.Duration

	opened bool
	closed bool
}

func NewNATSConnection() *NATSConnection {
	return &NATSConnection{
		assignJobTimeout: 5 * time.Second,
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

	nc.opened = true
	return nil
}

func (nc *NATSConnection) AssignJob(ctx context.Context, workerID string, config internal.MatchConfig) error {
	if !nc.opened {
		return ErrNATSConnNotOpen
	}
	if nc.closed {
		return ErrNATSConnClosed
	}

	payload, err := json.Marshal(config)
	if err != nil {
		return err
	}

	timeoutCtx, stop := context.WithTimeout(ctx, nc.assignJobTimeout)
	defer stop()

	if len(workerID) > 12 {
		workerID = workerID[:12]
	}

	_, err = nc.conn.RequestWithContext(
		timeoutCtx,
		assignSubject+"."+workerID,
		payload,
	)

	return err
}

func (nc *NATSConnection) GetWorkersPong(ctx context.Context, pongTimeout time.Duration) (internal.Responders, error) {
	if !nc.opened {
		return nil, ErrNATSConnNotOpen
	}
	if nc.closed {
		return nil, ErrNATSConnClosed
	}

	inbox := nc.conn.NewInbox()

	responders := make(internal.Responders)
	mu := sync.Mutex{}

	sub, err := nc.conn.Subscribe(inbox, func(msg *nats.Msg) {
		mu.Lock()
		defer mu.Unlock()
		responders[string(msg.Data)] = struct{}{}
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = sub.Unsubscribe() }()

	err = nc.conn.PublishRequest(healthSubject, inbox, nil)
	if err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(pongTimeout):
	}

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
