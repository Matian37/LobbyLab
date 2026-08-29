package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/Matian37/multiplayer-asset/server-manager/internal"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// NATS broker subjects (see docs/architecture.md for details)
const (
	healthSubject    = "workers.health"
	assignSubject    = "workers.assign"
	resultSubject    = "workers.results"
	resultStreamName = "RESULT"
)

// Errors returned by NATSConnection operations.
var (
	ErrNATSConnCannotBeReopened = errors.New("connection cannot be reopened")
	ErrNATSConnNotOpen          = errors.New("connection not open")
	ErrNATSConnClosed           = errors.New("connection closed")
	ErrNATSConnAlreadyOpen      = errors.New("connection already open")
	ErrNATSConnAlreadyClosed    = errors.New("connection already closed")
)

// NATSConnection implements internal.BrokerConnection over a NATS server with
// JetStream; the persistence is used to queue match results.
type NATSConnection struct {
	conn *nats.Conn
	js   *jetstream.JetStream

	resultConsumer jetstream.Consumer

	openTimeout      time.Duration
	assignJobTimeout time.Duration

	opened bool
	closed bool
}

// NewNATSConnection builds a NATSConnection with default timeouts.
func NewNATSConnection() *NATSConnection {
	return &NATSConnection{
		assignJobTimeout: 5 * time.Second,
		openTimeout:      5 * time.Second,
	}
}

// Open opens the connection and creates the result stream and its consumer.
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

// AssignJob publishes a match config to the worker-specific assign channel and
// waits for the worker to acknowledge it; on assign job timeout it returns an
// error.
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

	_, err = nc.conn.RequestWithContext(
		timeoutCtx,
		assignSubject+"."+workerID,
		payload,
	)

	return err
}

// GetWorkersPong broadcasts a health ping on the associated subject, waits for
// workers' replies, and returns the set of worker IDs that respond before the
// pong timeout.
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

// GetResult fetches the next undelivered match result from the result stream.
// It blocks until a message is available or ctx is canceled.
func (nc *NATSConnection) GetResult(ctx context.Context) (internal.Message, error) {
	if !nc.opened {
		return nil, ErrNATSConnNotOpen
	}
	if nc.closed {
		return nil, ErrNATSConnClosed
	}

	// HACK: Set deadline for timeout to approx 290 years.
	// Library for undocumented reason enforces expiry time even when
	// context without deadline is provided. The only way for Next to "disable" it,
	// is to set deadline to max duration. Possible way to remove this hack is to
	// refactor the codebase to use Consume function instead of Next, but this would
	// require bigger changes to the codebase.
	ctx, cancel := context.WithTimeout(ctx, math.MaxInt64)
	defer cancel()

	msg, err := nc.resultConsumer.Next(
		jetstream.FetchContext(ctx),
	)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// Close drains and closes the NATS connection. Partially opened connections
// are allowed to be closed.
func (nc *NATSConnection) Close() error {
	// when nc.conn is not nil then it is partially open
	if !nc.opened && nc.conn == nil {
		return ErrNATSConnNotOpen
	}
	if nc.closed {
		return ErrNATSConnAlreadyClosed
	}
	nc.closed = true
	return nc.conn.Drain()
}
