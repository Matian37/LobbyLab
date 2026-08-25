package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/Matian37/multiplayer-asset/game-server/internal"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const ResultStreamName = "RESULTS"

func createNATSServer(t *testing.T) string {
	t.Helper()

	opts := &server.Options{
		Port:      -1,
		Host:      "127.0.0.1",
		JetStream: true,
		StoreDir:  t.TempDir(),
	}
	s, err := server.NewServer(opts)
	require.NoError(t, err)

	s.Start()
	require.True(t, s.ReadyForConnections(5*time.Second))

	t.Cleanup(s.Shutdown)

	return fmt.Sprintf("nats://127.0.0.1:%d", s.Addr().(*net.TCPAddr).Port)
}

func newNATSServer(t *testing.T) string {
	addr := createNATSServer(t)

	nc, err := nats.Connect(addr)
	require.NoError(t, err)
	t.Cleanup(func() { nc.Close() })

	js, err := jetstream.New(nc)
	require.NoError(t, err)

	_, err = js.CreateStream(
		context.Background(),
		jetstream.StreamConfig{
			Name:     ResultStreamName,
			Subjects: []string{resultSubject},
		},
	)
	require.NoError(t, err)

	return addr
}

func newNATSServerWithoutResultStream(t *testing.T) string {
	return createNATSServer(t)
}

func getHelperConn(t *testing.T, addr string) *nats.Conn {
	nc, err := nats.Connect(addr)
	require.NoError(t, err)
	t.Cleanup(func() { nc.Close() })
	return nc
}

func TestNewConnection(t *testing.T) {
	brokerURI := "a"
	workerID := "b"

	c := NewConnection(brokerURI, workerID, testLogger)

	require.NotNil(t, c)

	assert.Equal(t, brokerURI, c.brokerURI)
	assert.Equal(t, workerID, c.workerID)

	assert.Nil(t, c.conn)
	assert.Nil(t, c.requestSub)

	assert.False(t, c.opened)
	assert.False(t, c.closed)
}

func TestNATSConnection_subscribeAssign(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		addr := newNATSServer(t)

		c := NewConnection(addr, "a", testLogger)
		c.conn = getHelperConn(t, addr)

		require.NoError(t, c.subscribeAssign())
		assert.NotNil(t, c.requestSub)
		assert.True(t, c.requestSub.IsValid())

		msgLimit, bytesLimit, err := c.requestSub.PendingLimits()
		require.NoError(t, err)
		assert.Equal(t, 1, msgLimit)
		assert.Equal(t, -1, bytesLimit)
	})
}

func TestNATSConnection_subscribeHealth(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		addr := newNATSServer(t)

		c := NewConnection(addr, "a", testLogger)
		c.conn = getHelperConn(t, addr)

		require.NoError(t, c.subscribeHealth())

		pub := getHelperConn(t, addr)
		msg, err := pub.Request(healthSubject, []byte{}, time.Second)
		require.NoError(t, err)
		assert.Equal(t, []byte(c.workerID), msg.Data)
	})
}

func TestNATSConnection_Open(t *testing.T) {
	t.Run("already open", func(t *testing.T) {
		c := NATSConnection{opened: true}
		assert.ErrorIs(t, c.Open(0), ErrConnectionAlreadyOpen)
	})

	t.Run("already closed", func(t *testing.T) {
		c := NATSConnection{opened: true, closed: true}
		assert.ErrorIs(t, c.Open(0), ErrConnectionAlreadyClosed)
	})

	t.Run("failure", func(t *testing.T) {
		c := NewConnection("nats://10.255.255.1:4222", "b", testLogger)
		err := c.Open(0)
		assert.ErrorContains(t, err, "i/o timeout")

		assert.False(t, c.opened)
		assert.False(t, c.closed)
		assert.Nil(t, c.conn)
	})

	t.Run("missing result stream", func(t *testing.T) {
		addr := newNATSServerWithoutResultStream(t)

		c := NewConnection(addr, "a", testLogger)
		require.ErrorIs(t, c.Open(150*time.Millisecond), jetstream.ErrStreamNotFound)

		assert.False(t, c.opened)
		assert.False(t, c.closed)
	})

	t.Run("success", func(t *testing.T) {
		addr := newNATSServer(t)

		c := NewConnection(addr, "a", testLogger)
		require.NoError(t, c.Open(150*time.Millisecond))

		assert.True(t, c.opened)
		assert.False(t, c.closed)

		require.NotNil(t, c.conn)
		assert.True(t, c.conn.IsConnected())

		require.NotNil(t, c.js)

		require.NotNil(t, c.requestSub)
		assert.True(t, c.requestSub.IsValid())
	})

	t.Run("partial opening", func(t *testing.T) {
		addr := newNATSServer(t)

		// space in workerID triggers error in subscribeAssign
		c := NewConnection(addr, "a b", testLogger)
		assert.ErrorIs(t, c.Open(150*time.Millisecond), nats.ErrBadSubject)

		assert.False(t, c.opened)
		assert.False(t, c.closed)
	})
}

func TestNATSConnection_Close(t *testing.T) {
	t.Run("not open", func(t *testing.T) {
		c := NATSConnection{}
		assert.ErrorIs(t, c.Close(), ErrConnectionNotOpen)
	})

	t.Run("already closed", func(t *testing.T) {
		c := NATSConnection{opened: true, closed: true}
		assert.ErrorIs(t, c.Close(), ErrConnectionAlreadyClosed)
	})

	t.Run("success", func(t *testing.T) {
		addr := newNATSServer(t)

		c := NewConnection(addr, "a", testLogger)
		require.NoError(t, c.Open(150*time.Millisecond))
		require.True(t, c.conn.IsConnected())

		assert.NoError(t, c.Close())
		assert.True(t, c.closed)
		assert.True(t, c.conn.IsDraining())
	})

	t.Run("partially opened", func(t *testing.T) {
		addr := newNATSServer(t)

		c := NewConnection(addr, "a", testLogger)
		require.NoError(t, c.Open(150*time.Millisecond))
		require.True(t, c.conn.IsConnected())

		assert.NoError(t, c.Close())
		assert.True(t, c.closed)
		assert.True(t, c.conn.IsDraining())
	})
}

func TestNATSConnection_GetMatchConfig(t *testing.T) {
	t.Run("not open", func(t *testing.T) {
		c := NATSConnection{}
		_, err := c.GetMatchConfig(context.Background())
		assert.ErrorIs(t, err, ErrConnectionNotOpen)
	})

	t.Run("closed", func(t *testing.T) {
		c := NATSConnection{opened: true, closed: true}
		_, err := c.GetMatchConfig(context.Background())
		assert.ErrorIs(t, err, ErrConnectionAlreadyClosed)
	})

	t.Run("context canceled", func(t *testing.T) {
		addr := newNATSServer(t)

		c := NewConnection(addr, "a", testLogger)
		require.NoError(t, c.Open(150*time.Millisecond))

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := c.GetMatchConfig(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("success", func(t *testing.T) {
		addr := newNATSServer(t)

		workerID := "a"
		c := NewConnection(addr, workerID, testLogger)
		require.NoError(t, c.Open(150*time.Millisecond))

		nc := getHelperConn(t, addr)

		expectedConfig := internal.MatchConfig{MatchID: 1, Config: []byte(`{"gameconfig": 123}`)}
		expectedConfigJSON, err := json.Marshal(expectedConfig)
		require.NoError(t, err)

		msgChan := make(chan *nats.Msg)
		errChan := make(chan error)
		go func() {
			msg, err := nc.Request(
				assignSubject+"."+workerID,
				[]byte(expectedConfigJSON),
				1*time.Second,
			)
			if err != nil {
				errChan <- err
			} else {
				msgChan <- msg
			}
		}()

		config, err := c.GetMatchConfig(context.Background())
		require.NoError(t, err)
		assert.Equal(t, expectedConfig.MatchID, config.MatchID)
		assert.JSONEq(t, string(expectedConfig.Config), string(config.Config))

		select {
		case msg := <-msgChan:
			assert.Empty(t, msg.Data)
		case <-errChan:
			t.Fatal("error waiting for msgChan", err)
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for assign ack")
		}
	})
}

func TestNATSConnection_SendCancel(t *testing.T) {
	t.Run("not open", func(t *testing.T) {
		c := NATSConnection{}
		err := c.SendCancel(context.Background(), 0)
		assert.ErrorIs(t, err, ErrConnectionNotOpen)
	})

	t.Run("closed", func(t *testing.T) {
		c := NATSConnection{opened: true, closed: true}
		err := c.SendCancel(context.Background(), 0)
		assert.ErrorIs(t, err, ErrConnectionAlreadyClosed)
	})

	t.Run("context canceled", func(t *testing.T) {
		addr := newNATSServer(t)

		c := NewConnection(addr, "a", testLogger)
		require.NoError(t, c.Open(150*time.Millisecond))

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		require.ErrorIs(t, c.SendCancel(ctx, 0), context.Canceled)
	})

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		addr := newNATSServer(t)

		c := NewConnection(addr, "a", testLogger)
		require.NoError(t, c.Open(150*time.Millisecond))

		nc := getHelperConn(t, addr)
		js, err := jetstream.New(nc)
		require.NoError(t, err)
		stream, err := js.Stream(ctx, ResultStreamName)
		require.NoError(t, err)

		require.NoError(t, c.SendCancel(context.Background(), 1))

		msg, err := stream.GetLastMsgForSubject(ctx, resultSubject)
		require.NoError(t, err)

		var result internal.Result
		require.NoError(t, json.Unmarshal(msg.Data, &result))
		assert.False(t, result.Success)
		assert.Equal(t, json.RawMessage("{}"), result.Details)
		assert.Equal(t, 1, result.MatchID)
	})
}

func TestNATSConnection_SendResult(t *testing.T) {
	t.Run("not open", func(t *testing.T) {
		c := NATSConnection{}
		err := c.SendResult(context.Background(), 0, []byte{})
		assert.ErrorIs(t, err, ErrConnectionNotOpen)
	})

	t.Run("closed", func(t *testing.T) {
		c := NATSConnection{opened: true, closed: true}
		err := c.SendResult(context.Background(), 0, []byte{})
		assert.ErrorIs(t, err, ErrConnectionAlreadyClosed)
	})

	t.Run("context canceled", func(t *testing.T) {
		addr := newNATSServer(t)

		c := NewConnection(addr, "a", testLogger)
		require.NoError(t, c.Open(150*time.Millisecond))

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := c.SendResult(ctx, 0, []byte("{}"))
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		addr := newNATSServer(t)

		c := NewConnection(addr, "a", testLogger)
		require.NoError(t, c.Open(150*time.Millisecond))

		nc := getHelperConn(t, addr)

		js, err := jetstream.New(nc)
		require.NoError(t, err)

		stream, err := js.Stream(ctx, ResultStreamName)
		require.NoError(t, err)

		expectedResult := internal.Result{Success: true, MatchID: 1, Details: []byte(`{"data":123}`)}
		expectedJSON, err := json.Marshal(expectedResult)
		require.NoError(t, err)

		require.NoError(t, c.SendResult(context.Background(), expectedResult.MatchID, expectedResult.Details))

		msg, err := stream.GetLastMsgForSubject(ctx, resultSubject)
		require.NoError(t, err)
		assert.JSONEq(t, string(expectedJSON), string(msg.Data))
	})
}
