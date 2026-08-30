//go:build integration

package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/Matian37/LobbyLab/server-manager/internal"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createNATSServer(t *testing.T, enableJetStream bool) string {
	t.Helper()

	opts := &server.Options{
		Port:      -1,
		Host:      "127.0.0.1",
		JetStream: enableJetStream,
		StoreDir:  t.TempDir(),
	}
	s, err := server.NewServer(opts)
	require.NoError(t, err)

	s.Start()
	require.True(t, s.ReadyForConnections(5*time.Second))

	t.Cleanup(s.Shutdown)

	addr := fmt.Sprintf("nats://127.0.0.1:%d", s.Addr().(*net.TCPAddr).Port)
	return addr
}

func newNATSServer(t *testing.T) string {
	return createNATSServer(t, true)
}

func newNATSServerWithoutJS(t *testing.T) string {
	return createNATSServer(t, false)
}

func TestIntegration_NewNATSConnection(t *testing.T) {
	conn := NewNATSConnection()

	assert.NotNil(t, conn)
	assert.Equal(t, 5*time.Second, conn.assignJobTimeout)
	assert.Equal(t, 5*time.Second, conn.openTimeout)

	assert.Nil(t, conn.conn)
	assert.Nil(t, conn.js)
	assert.Nil(t, conn.resultConsumer)

	assert.False(t, conn.opened)
	assert.False(t, conn.closed)
}

func TestIntegration_NATSConnection_Open(t *testing.T) {
	t.Run("closed", func(t *testing.T) {
		conn := NATSConnection{closed: true}
		err := conn.Open(context.Background(), &internal.EnvConfig{})
		assert.ErrorIs(t, err, ErrNATSConnCannotBeReopened)
	})

	t.Run("already open", func(t *testing.T) {
		conn := NATSConnection{opened: true}
		err := conn.Open(context.Background(), &internal.EnvConfig{})
		assert.ErrorIs(t, err, ErrNATSConnAlreadyOpen)
	})

	t.Run("connection failure", func(t *testing.T) {
		conn := NewNATSConnection()
		err := conn.Open(context.Background(), &internal.EnvConfig{})
		require.ErrorIs(t, err, nats.ErrNoServers)

		assert.False(t, conn.opened)
		assert.False(t, conn.closed)
		assert.Nil(t, conn.conn)
	})

	t.Run("jetstream failure", func(t *testing.T) {
		addr := newNATSServerWithoutJS(t)

		conn := NewNATSConnection()
		err := conn.Open(context.Background(), &internal.EnvConfig{BrokerURI: addr})
		require.ErrorIs(t, err, nats.ErrNoResponders)

		assert.False(t, conn.opened)
		assert.False(t, conn.closed)
	})

	t.Run("success", func(t *testing.T) {
		addr := newNATSServer(t)

		conn := NewNATSConnection()
		err := conn.Open(context.Background(), &internal.EnvConfig{BrokerURI: addr})
		require.NoError(t, err)

		assert.True(t, conn.opened)
		assert.False(t, conn.closed)

		require.NotNil(t, conn.conn)
		assert.True(t, conn.conn.IsConnected())

		require.NotNil(t, conn.js)

		require.NotNil(t, conn.resultConsumer)
	})
}

func TestIntegration_NATSConnection_AssignJob(t *testing.T) {
	t.Run("not open", func(t *testing.T) {
		conn := NATSConnection{}
		err := conn.AssignJob(context.Background(), "", internal.MatchConfig{})
		assert.ErrorIs(t, err, ErrNATSConnNotOpen)
	})

	t.Run("closed", func(t *testing.T) {
		conn := NATSConnection{opened: true, closed: true}
		err := conn.AssignJob(context.Background(), "", internal.MatchConfig{})
		assert.ErrorIs(t, err, ErrNATSConnClosed)
	})

	t.Run("timeout", func(t *testing.T) {
		addr := newNATSServer(t)

		conn := NewNATSConnection()
		err := conn.Open(context.Background(), &internal.EnvConfig{BrokerURI: addr})
		require.NoError(t, err)

		conn.assignJobTimeout = 1 * time.Millisecond

		start := time.Now()
		err = conn.AssignJob(context.Background(), "", internal.MatchConfig{})
		elapsed := time.Since(start)

		assert.ErrorIs(t, err, nats.ErrNoResponders)
		assert.Less(t, elapsed, 50*time.Millisecond)
	})

	t.Run("context canceled", func(t *testing.T) {
		addr := newNATSServer(t)

		conn := NewNATSConnection()
		err := conn.Open(context.Background(), &internal.EnvConfig{BrokerURI: addr})
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err = conn.AssignJob(ctx, "", internal.MatchConfig{})
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("success", func(t *testing.T) {
		addr := newNATSServer(t)

		conn := NewNATSConnection()
		err := conn.Open(context.Background(), &internal.EnvConfig{BrokerURI: addr})
		require.NoError(t, err)

		workerID := "test-worker"

		expectedConfig := internal.MatchConfig{MatchID: 1, Config: json.RawMessage(`{"game":"test"}`)}
		expectedPayload, err := json.Marshal(expectedConfig)
		require.NoError(t, err)

		nc, err := nats.Connect(addr)
		require.NoError(t, err)
		t.Cleanup(func() { nc.Close() })

		receivedConfig := make(chan string, 1)
		sub, err := nc.Subscribe(assignSubject+"."+workerID, func(msg *nats.Msg) {
			receivedConfig <- string(msg.Data)
			msg.Respond(nil)
		})
		require.NoError(t, err)
		require.NoError(t, sub.AutoUnsubscribe(1))
		require.NoError(t, nc.Flush())

		err = conn.AssignJob(context.Background(), workerID, expectedConfig)
		require.NoError(t, err)

		select {
		case config := <-receivedConfig:
			assert.Equal(t, string(expectedPayload), config)
		case <-time.After(1 * time.Second):
			assert.Fail(t, "did not receive config on worker subject")
		}
	})
}

func TestIntegration_NATSConnection_PingWorkers(t *testing.T) {
	t.Run("not open", func(t *testing.T) {
		conn := NATSConnection{}
		_, err := conn.GetWorkersPong(context.Background(), time.Nanosecond)
		assert.ErrorIs(t, err, ErrNATSConnNotOpen)
	})

	t.Run("closed", func(t *testing.T) {
		conn := NATSConnection{opened: true, closed: true}
		_, err := conn.GetWorkersPong(context.Background(), time.Nanosecond)
		assert.ErrorIs(t, err, ErrNATSConnClosed)
	})

	t.Run("context canceled", func(t *testing.T) {
		addr := newNATSServer(t)

		conn := NewNATSConnection()
		err := conn.Open(context.Background(), &internal.EnvConfig{BrokerURI: addr})
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err = conn.GetWorkersPong(ctx, 150*time.Millisecond)
		require.ErrorIs(t, err, ctx.Err())
	})

	t.Run("success", func(t *testing.T) {
		addr := newNATSServer(t)

		conn := NewNATSConnection()
		err := conn.Open(context.Background(), &internal.EnvConfig{BrokerURI: addr})
		require.NoError(t, err)

		nc, err := nats.Connect(addr)
		require.NoError(t, err)
		t.Cleanup(func() { nc.Close() })

		workerID := "worker-1"
		_, err = nc.Subscribe(healthSubject, func(msg *nats.Msg) {
			msg.Respond([]byte(workerID))
		})
		require.NoError(t, err)
		nc.Flush()

		responders, err := conn.GetWorkersPong(context.Background(), 150*time.Millisecond)
		require.NoError(t, err)

		_, ok := responders[workerID]
		assert.True(t, ok, "expected worker-1 to be in responders")
	})
}

func TestIntegration_NATSConnection_GetResult(t *testing.T) {
	t.Run("not open", func(t *testing.T) {
		conn := NATSConnection{}
		_, err := conn.GetResult(context.Background())
		assert.ErrorIs(t, err, ErrNATSConnNotOpen)
	})

	t.Run("closed", func(t *testing.T) {
		conn := NATSConnection{opened: true, closed: true}
		_, err := conn.GetResult(context.Background())
		assert.ErrorIs(t, err, ErrNATSConnClosed)
	})

	t.Run("context canceled", func(t *testing.T) {
		addr := newNATSServer(t)

		conn := NewNATSConnection()
		err := conn.Open(context.Background(), &internal.EnvConfig{BrokerURI: addr})
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err = conn.GetResult(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("success", func(t *testing.T) {
		addr := newNATSServer(t)

		conn := NewNATSConnection()
		err := conn.Open(context.Background(), &internal.EnvConfig{BrokerURI: addr})
		require.NoError(t, err)

		expectedData := []byte("abc")

		nc, err := nats.Connect(addr)
		require.NoError(t, err)

		js, err := jetstream.New(nc)
		require.NoError(t, err)
		_, err = js.Publish(context.Background(), resultSubject, expectedData)
		require.NoError(t, err)

		msg, err := conn.GetResult(context.Background())
		require.NoError(t, err)

		assert.Equal(t, expectedData, msg.Data())
	})
}

func TestIntegration_NATSConnection_Close(t *testing.T) {
	t.Run("not open", func(t *testing.T) {
		conn := NATSConnection{}
		err := conn.Close()
		assert.ErrorIs(t, err, ErrNATSConnNotOpen)
	})

	t.Run("already closed", func(t *testing.T) {
		conn := NATSConnection{opened: true, closed: true}
		err := conn.Close()
		assert.ErrorIs(t, err, ErrNATSConnAlreadyClosed)
	})

	t.Run("success", func(t *testing.T) {
		addr := newNATSServer(t)

		conn := NewNATSConnection()
		err := conn.Open(context.Background(), &internal.EnvConfig{BrokerURI: addr})
		require.NoError(t, err)
		require.True(t, conn.conn.IsConnected())

		assert.NoError(t, conn.Close())
		assert.True(t, conn.closed)
		assert.True(t, conn.conn.IsDraining())
	})

	t.Run("partially open", func(t *testing.T) {
		addr := newNATSServer(t)

		nc, err := nats.Connect(addr)
		require.NoError(t, err)

		conn := NATSConnection{conn: nc, opened: false}

		assert.NoError(t, conn.Close())
		assert.True(t, conn.closed)
		assert.True(t, nc.IsDraining())
	})
}
