package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newNATSServer(t *testing.T) string {
	t.Helper()

	opts := &server.Options{
		Port: -1,
		Host: "127.0.0.1",
	}
	s, err := server.NewServer(opts)
	require.NoError(t, err)

	s.Start()
	require.True(t, s.ReadyForConnections(5*time.Second))

	t.Cleanup(s.Shutdown)

	return fmt.Sprintf("nats://127.0.0.1:%d", s.Addr().(*net.TCPAddr).Port)
}

func TestNewConnection(t *testing.T) {
	brokerUri := "a"
	containerId := "b"

	c := NewConnection(brokerUri, containerId)

	assert.NotNil(t, c)

	assert.Equal(t, brokerUri, c.brokerUri)
	assert.Equal(t, containerId, c.containerId)

	assert.Nil(t, c.conn)
	assert.Nil(t, c.healthSub)
	assert.Nil(t, c.requestSub)

	assert.False(t, c.opened)
	assert.False(t, c.closed)
}

func TestNATSConnection_Open(t *testing.T) {
	t.Run("already_open", func(t *testing.T) {
		c := NATSConnection{opened: true}
		err := c.Open(150 * time.Millisecond)
		assert.ErrorIs(t, err, ErrConnectionNotReopenable)
	})

	t.Run("failure", func(t *testing.T) {
		c := NewConnection("nats://10.255.255.1:4222", "b")
		err := c.Open(150 * time.Millisecond)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "i/o timeout")

		assert.False(t, c.opened)
		assert.Nil(t, c.conn)
	})

	t.Run("success", func(t *testing.T) {
		addr := newNATSServer(t)

		c := NewConnection(addr, "a")
		err := c.Open(150 * time.Millisecond)
		require.NoError(t, err)

		assert.True(t, c.opened)
		assert.False(t, c.closed)

		require.NotNil(t, c.conn)
		assert.True(t, c.conn.IsConnected())
		assert.NotNil(t, c.healthSub)
		assert.NotNil(t, c.requestSub)
	})
}

func TestNATSConnection_Close(t *testing.T) {
	t.Run("not_open", func(t *testing.T) {
		c := NATSConnection{}
		err := c.Close()
		assert.ErrorIs(t, err, ErrConnectionNotOpen)
	})

	t.Run("already_closed", func(t *testing.T) {
		c := NATSConnection{opened: true, closed: true}
		err := c.Close()
		assert.ErrorIs(t, err, ErrConnectionClosed)
	})

	t.Run("success", func(t *testing.T) {
		addr := newNATSServer(t)

		c := NewConnection(addr, "a")
		err := c.Open(150 * time.Millisecond)
		require.NoError(t, err)
		require.True(t, c.conn.IsConnected())

		assert.NoError(t, c.Close())
		assert.True(t, c.closed)
		assert.True(t, c.conn.IsDraining())
	})
}

func TestNATSConnection_GetMatchConfig(t *testing.T) {
	t.Run("not_open", func(t *testing.T) {
		c := NATSConnection{}
		_, err := c.GetMatchConfig(context.Background())
		assert.ErrorIs(t, err, ErrConnectionNotOpen)
	})

	t.Run("closed", func(t *testing.T) {
		c := NATSConnection{opened: true, closed: true}
		_, err := c.GetMatchConfig(context.Background())
		assert.ErrorIs(t, err, ErrConnectionClosed)
	})

	t.Run("context_cancelled", func(t *testing.T) {
		addr := newNATSServer(t)

		c := NewConnection(addr, "a")
		err := c.Open(150 * time.Millisecond)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err = c.GetMatchConfig(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("success", func(t *testing.T) {
		addr := newNATSServer(t)

		containerId := "a"
		c := NewConnection(addr, containerId)
		err := c.Open(150 * time.Millisecond)
		require.NoError(t, err)

		nc, err := nats.Connect(addr)
		require.NoError(t, err)
		t.Cleanup(func() { nc.Close() })

		expectedConfig := `{"config": 123}`

		doneChan := make(chan struct{})
		go func() {
			defer close(doneChan)

			msg, err := nc.Request(
				assignSubject+"."+containerId,
				[]byte(expectedConfig),
				1*time.Second,
			)
			require.NoError(t, err)
			assert.Empty(t, msg.Data)
		}()

		config, err := c.GetMatchConfig(context.Background())
		require.NoError(t, err)
		assert.Equal(t, string(expectedConfig), config)

		<-doneChan
	})
}

func TestNATSConnection_SendCancel(t *testing.T) {
	t.Run("not_open", func(t *testing.T) {
		c := NATSConnection{}
		err := c.SendCancel()
		assert.ErrorIs(t, err, ErrConnectionNotOpen)
	})

	t.Run("closed", func(t *testing.T) {
		c := NATSConnection{opened: true, closed: true}
		err := c.SendCancel()
		assert.ErrorIs(t, err, ErrConnectionClosed)
	})

	t.Run("success", func(t *testing.T) {
		addr := newNATSServer(t)

		c := NewConnection(addr, "a")
		err := c.Open(150 * time.Millisecond)
		require.NoError(t, err)

		nc, err := nats.Connect(addr)
		require.NoError(t, err)
		t.Cleanup(func() { nc.Close() })

		sub, err := nc.SubscribeSync(resultSubject)
		require.NoError(t, err)
		nc.Flush()

		require.NoError(t, c.SendCancel())

		msg, err := sub.NextMsg(2 * time.Second)
		require.NoError(t, err)

		var result Result
		err = json.Unmarshal(msg.Data, &result)
		require.NoError(t, err)

		assert.False(t, result.Success)
		assert.Equal(t, json.RawMessage("{}"), result.Details)
	})
}

func TestNATSConnection_SendResult(t *testing.T) {
	t.Run("not_open", func(t *testing.T) {
		c := NATSConnection{}
		err := c.SendResult([]byte("data"))
		assert.ErrorIs(t, err, ErrConnectionNotOpen)
	})

	t.Run("closed", func(t *testing.T) {
		c := NATSConnection{opened: true, closed: true}
		err := c.SendResult([]byte("data"))
		assert.ErrorIs(t, err, ErrConnectionClosed)
	})

	t.Run("success", func(t *testing.T) {
		addr := newNATSServer(t)

		c := NewConnection(addr, "a")
		err := c.Open(150 * time.Millisecond)
		require.NoError(t, err)

		nc, err := nats.Connect(addr)
		require.NoError(t, err)
		t.Cleanup(func() { nc.Close() })

		sub, err := nc.SubscribeSync(resultSubject)
		require.NoError(t, err)
		nc.Flush()

		result := `{"data":123}`
		expected := `{"success":true,"details":` + result + `}`

		require.NoError(t, c.SendResult([]byte(result)))

		msg, err := sub.NextMsg(150 * time.Millisecond)
		require.NoError(t, err)
		assert.Equal(t, []byte(expected), msg.Data)
	})
}

func TestNATSConnection_subscribeAssign(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		addr := newNATSServer(t)

		c := NewConnection(addr, "a")

		nc, err := nats.Connect(addr)
		require.NoError(t, err)
		c.conn = nc
		t.Cleanup(func() { nc.Close() })

		err = c.subscribeAssign()
		require.NoError(t, err)

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

		c := NewConnection(addr, "a")

		nc, err := nats.Connect(addr)
		require.NoError(t, err)
		c.conn = nc
		t.Cleanup(func() { nc.Close() })

		require.NoError(t, c.subscribeHealth())
		assert.NotNil(t, c.healthSub)

		pub, err := nats.Connect(addr)
		require.NoError(t, err)
		t.Cleanup(func() { pub.Close() })

		pongSubject := healthSubject + "." + c.containerId
		pong, err := pub.SubscribeSync(pongSubject)
		require.NoError(t, err)
		pub.Flush()

		err = pub.Publish(healthSubject, []byte{})
		require.NoError(t, err)

		msg, err := pong.NextMsg(150 * time.Millisecond)
		require.NoError(t, err)
		assert.Empty(t, msg.Data)
	})
}

func TestNATSConnection_HealthPing(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		addr := newNATSServer(t)

		c := NewConnection(addr, "a")
		err := c.Open(150 * time.Millisecond)
		require.NoError(t, err)

		nc, err := nats.Connect(addr)
		require.NoError(t, err)
		t.Cleanup(func() { nc.Close() })

		pong, err := nc.SubscribeSync(healthSubject + "." + c.containerId)
		require.NoError(t, err)
		nc.Flush()

		require.NoError(t, nc.Publish(healthSubject, []byte{}))
		msg, err := pong.NextMsg(1 * time.Second)
		require.NoError(t, err)
		assert.Empty(t, msg.Data)
	})
}
