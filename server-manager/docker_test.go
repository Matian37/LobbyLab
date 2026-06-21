package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"server-manager/internal"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	unreachableDockerHost = "tcp://10.255.255.1:2375"
	containerImage        = "busybox:latest"
)

// TODO: make integration test skippable

func newTestConnWithPorts(t *testing.T, exposePorts []string, clientPorts []string) *DockerConnection {
	t.Helper()

	exposeSet := make(network.PortSet)
	for _, p := range exposePorts {
		exposeSet[network.MustParsePort(p)] = struct{}{}
	}
	clientSet := make(internal.NamedPortSet)
	for _, p := range clientPorts {
		name, portString, ok := strings.Cut(p, ":")
		require.True(t, ok, "name for client port was not provided in test data")
		require.NotEmpty(t, name, "empty name for client port in test data")
		require.NotEmpty(t, portString, "empty client port in test data")

		clientSet[internal.NamedPort{Name: name, Port: network.MustParsePort(portString)}] = struct{}{}
	}

	dc := NewDockerConnection()

	err := dc.Init(&internal.EnvConfig{
		Image:       containerImage,
		ExposePorts: exposeSet,
		ClientPorts: clientSet,
	})
	require.NoError(t, err)
	t.Cleanup(func() { dc.Close() })

	return dc
}

func newTestConn(t *testing.T) *DockerConnection {
	t.Helper()
	return newTestConnWithPorts(t, nil, nil)
}

// NOTE: CMD is required to make container hang forever
func createContainer(t *testing.T, dc *DockerConnection) string {
	t.Helper()

	portMap := make(network.PortMap)
	for port := range dc.config.ExposePorts {
		portMap[port] = []network.PortBinding{{}}
	}

	options := dc.containerCreateOptions(portMap)
	if options.Config == nil {
		options.Config = &container.Config{}
	}
	options.Config.Cmd = []string{"sleep", "inf"}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := dc.client.ContainerCreate(ctx, options)
	require.NoError(t, err)

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		dc.client.ContainerRemove(cleanupCtx, res.ID, client.ContainerRemoveOptions{Force: true})
	})

	return res.ID
}

func startContainer(t *testing.T, dc *DockerConnection, id string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := dc.client.ContainerStart(ctx, id, client.ContainerStartOptions{})
	require.NoError(t, err)
}

func inspectContainer(t *testing.T, dc *DockerConnection, id string) client.ContainerInspectResult {
	t.Helper()
	res, err := dc.client.ContainerInspect(context.Background(), id, client.ContainerInspectOptions{})
	require.NoError(t, err)
	return res
}

func TestDockerConnection_New(t *testing.T) {
	dc := NewDockerConnection()
	assert.NotNil(t, dc)
	assert.False(t, dc.initialized)
	assert.False(t, dc.closed)
	assert.Nil(t, dc.client)
	assert.Nil(t, dc.config)
}

func TestDockerConnection_Init(t *testing.T) {
	t.Run("already init", func(t *testing.T) {
		dc := DockerConnection{initialized: true}
		err := dc.Init(&internal.EnvConfig{})
		assert.ErrorIs(t, err, ErrDockerConnAlreadyInit)
	})

	t.Run("closed", func(t *testing.T) {
		dc := DockerConnection{initialized: true, closed: true}
		err := dc.Init(&internal.EnvConfig{})
		assert.ErrorIs(t, err, ErrDockerConnClosed)
	})

	t.Run("success", func(t *testing.T) {
		dc := NewDockerConnection()
		config := &internal.EnvConfig{}

		err := dc.Init(config)
		t.Cleanup(func() { _ = dc.client.Close() })
		require.NoError(t, err)

		assert.True(t, dc.initialized)
		assert.False(t, dc.closed)
		assert.NotNil(t, dc.client)
		assert.Same(t, config, dc.config)
	})
}

func TestDockerConnection_Close(t *testing.T) {
	t.Run("not init", func(t *testing.T) {
		dc := DockerConnection{}
		err := dc.Close()
		assert.ErrorIs(t, err, ErrDockerConnNotInit)
	})

	t.Run("closed", func(t *testing.T) {
		dc := DockerConnection{initialized: true, closed: true}
		err := dc.Close()
		assert.ErrorIs(t, err, ErrDockerConnClosed)
	})

	t.Run("success", func(t *testing.T) {
		dc := newTestConn(t)
		err := dc.Close()
		assert.NoError(t, err)
		assert.True(t, dc.closed)
	})
}

func TestDockerConnection_containerCreateOptions(t *testing.T) {
	exposePorts := []string{"80", "8080"}
	portMap := network.PortMap{}

	dc := newTestConnWithPorts(t, exposePorts, nil)

	opts := dc.containerCreateOptions(portMap)

	assert.Equal(t, opts.Image, dc.config.Image)
	require.NotNil(t, opts.HostConfig)
	assert.Equal(t, opts.HostConfig.PortBindings, portMap)
}

func TestDockerConnection_SpawnContainer(t *testing.T) {
	t.Run("not init", func(t *testing.T) {
		dc := DockerConnection{}
		_, err := dc.SpawnContainer(context.Background())
		assert.ErrorIs(t, err, ErrDockerConnNotInit)
	})

	t.Run("closed", func(t *testing.T) {
		dc := DockerConnection{initialized: true, closed: true}
		_, err := dc.SpawnContainer(context.Background())
		assert.ErrorIs(t, err, ErrDockerConnClosed)
	})

	t.Run("timeout", func(t *testing.T) {
		dc := newTestConn(t)
		dc.createTimeout = 0

		start := time.Now()
		_, err := dc.SpawnContainer(context.Background())
		elapsed := time.Since(start)

		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Less(t, elapsed, 20*time.Millisecond)
	})

	t.Run("canceled", func(t *testing.T) {
		dc := newTestConn(t)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := dc.SpawnContainer(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("success", func(t *testing.T) {
		dc := newTestConn(t)

		id, err := dc.SpawnContainer(context.Background())
		require.NoError(t, err)
		assert.NotEmpty(t, id)

		t.Cleanup(func() {
			dc.client.ContainerRemove(context.Background(), id, client.ContainerRemoveOptions{Force: true})
		})

		info := inspectContainer(t, dc, id)
		require.NotNil(t, info.Container.State)
		assert.True(t, info.Container.State.Running)
		require.NotNil(t, info.Container.Config)
		assert.Equal(t, dc.config.Image, info.Container.Config.Image)
	})
}

func TestDockerConnection_RestartContainer(t *testing.T) {
	t.Run("not init", func(t *testing.T) {
		dc := NewDockerConnection()
		err := dc.RestartContainer(context.Background(), "")
		assert.ErrorIs(t, err, ErrDockerConnNotInit)
	})

	t.Run("closed", func(t *testing.T) {
		dc := DockerConnection{initialized: true, closed: true}
		err := dc.RestartContainer(context.Background(), "")
		assert.ErrorIs(t, err, ErrDockerConnClosed)
	})

	t.Run("function timeout", func(t *testing.T) {
		dc := newTestConn(t)
		dc.restartTimeout = 0

		start := time.Now()
		err := dc.RestartContainer(context.Background(), "id")
		elapsed := time.Since(start)

		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Less(t, elapsed, 50*time.Millisecond)
	})

	t.Run("canceled", func(t *testing.T) {
		dc := newTestConn(t)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := dc.RestartContainer(ctx, "id")

		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("success", func(t *testing.T) {
		dc := newTestConn(t)
		dc.containerStopTimeout = 0

		id := createContainer(t, dc)
		startContainer(t, dc, id)

		oldState := inspectContainer(t, dc, id)

		start := time.Now()
		err := dc.RestartContainer(context.Background(), id)
		assert.NoError(t, err)

		newState := inspectContainer(t, dc, id)
		assert.NotEqual(t, oldState.Container.State.StartedAt, newState.Container.State.StartedAt)

		// cmd is meant to hang forever,
		// so if containerStopTimeout does not work then this assert should fail
		elapsed := time.Since(start)
		assert.Less(t, elapsed, 5*time.Second)
	})
}

func TestDockerConnection_KillContainer(t *testing.T) {
	t.Run("not init", func(t *testing.T) {
		dc := DockerConnection{}
		err := dc.KillContainer(context.Background(), "id")
		assert.ErrorIs(t, err, ErrDockerConnNotInit)
	})

	t.Run("closed", func(t *testing.T) {
		dc := DockerConnection{initialized: true, closed: true}
		err := dc.KillContainer(context.Background(), "id")
		assert.ErrorIs(t, err, ErrDockerConnClosed)
	})

	t.Run("timeout", func(t *testing.T) {
		dc := newTestConn(t)
		dc.killTimeout = 0

		start := time.Now()
		err := dc.KillContainer(context.Background(), "id")
		elapsed := time.Since(start)

		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Less(t, elapsed, 200*time.Millisecond)
	})

	t.Run("canceled", func(t *testing.T) {
		dc := newTestConn(t)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := dc.KillContainer(ctx, "id")
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("success", func(t *testing.T) {
		dc := newTestConn(t)
		id := createContainer(t, dc)
		startContainer(t, dc, id)

		err := dc.KillContainer(context.Background(), id)
		assert.NoError(t, err)

		info := inspectContainer(t, dc, id)
		assert.Equal(t, container.StateExited, info.Container.State.Status)
	})
}

func TestDockerConnection_getPorts(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		dc := newTestConn(t)
		dc.inspectTimeout = 0

		start := time.Now()
		_, err := dc.getPorts(context.Background(), "id")
		elapsed := time.Since(start)

		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Less(t, elapsed, 200*time.Millisecond)
	})

	t.Run("canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		dc := newTestConn(t)
		_, err := dc.getPorts(ctx, "id")
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("success", func(t *testing.T) {
		exposePorts := []string{"80", "400/udp", "8080/tcp"}

		dc := newTestConnWithPorts(t, exposePorts, nil)
		id := createContainer(t, dc)

		startContainer(t, dc, id)

		res, err := dc.getPorts(context.Background(), id)
		require.NoError(t, err)
		assert.NotNil(t, res)

		// checks if keys of exposePorts and res are equal
		assert.Equal(t, len(exposePorts), len(res))
		for _, port := range exposePorts {
			_, ok := res[network.MustParsePort(port)]
			assert.True(t, ok, "missing port %s", port)
		}
	})
}

func TestDockerConnection_genClientPortMap(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		portMap := network.PortMap{
			network.MustParsePort("80"):       {{}},
			network.MustParsePort("400/udp"):  {{}},
			network.MustParsePort("8080/tcp"): {{}},
			network.MustParsePort("7777/tcp"): {{}},
		}
		clientPorts := internal.NamedPortSet{
			internal.NamedPort{Name: "a", Port: network.MustParsePort("80")}:       {},
			internal.NamedPort{Name: "b", Port: network.MustParsePort("400/udp")}:  {},
			internal.NamedPort{Name: "d", Port: network.MustParsePort("7777/tcp")}: {},
		}

		dc := DockerConnection{config: &internal.EnvConfig{ClientPorts: clientPorts}}
		res := dc.genClientPortMap(portMap, "id")

		expected := internal.NamedPortMap{
			"a": network.MustParsePort("80"),
			"b": network.MustParsePort("400/udp"),
			"d": network.MustParsePort("7777/tcp"),
		}
		assert.Equal(t, expected, res)
	})
}

func TestDockerConnection_GetClientPortMap(t *testing.T) {
	t.Run("not init", func(t *testing.T) {
		dc := DockerConnection{}
		_, err := dc.GetClientPortMap(context.Background(), "id")
		assert.ErrorIs(t, err, ErrDockerConnNotInit)
	})

	t.Run("closed", func(t *testing.T) {
		dc := DockerConnection{initialized: true, closed: true}
		_, err := dc.GetClientPortMap(context.Background(), "id")
		assert.ErrorIs(t, err, ErrDockerConnClosed)
	})

	t.Run("canceled", func(t *testing.T) {
		dc := newTestConn(t)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := dc.GetClientPortMap(ctx, "id")
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("success", func(t *testing.T) {
		dc := newTestConnWithPorts(t, []string{"80", "443"}, []string{"abc:80"})
		id := createContainer(t, dc)
		startContainer(t, dc, id)

		ports, err := dc.GetClientPortMap(context.Background(), id)
		require.NoError(t, err)

		assert.Len(t, ports, 1)
		assert.Equal(t, network.MustParsePort("80"), ports["abc"])
	})
}

func TestDockerConnection_IsContainerStarted(t *testing.T) {
	t.Run("not init", func(t *testing.T) {
		dc := DockerConnection{}
		_, err := dc.IsContainerStarted(context.Background(), "id")
		assert.ErrorIs(t, err, ErrDockerConnNotInit)
	})

	t.Run("closed", func(t *testing.T) {
		dc := DockerConnection{initialized: true, closed: true}
		_, err := dc.IsContainerStarted(context.Background(), "id")
		assert.ErrorIs(t, err, ErrDockerConnClosed)
	})

	t.Run("canceled", func(t *testing.T) {
		dc := newTestConn(t)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := dc.IsContainerStarted(ctx, "id")
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("timeout", func(t *testing.T) {
		dc := newTestConn(t)
		dc.inspectTimeout = 0

		start := time.Now()
		_, err := dc.IsContainerStarted(context.Background(), "id")
		elapsed := time.Since(start)

		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Less(t, elapsed, 200*time.Millisecond)
	})

	t.Run("success", func(t *testing.T) {
		t.Run("created", func(t *testing.T) {
			dc := newTestConn(t)
			id := createContainer(t, dc)

			started, err := dc.IsContainerStarted(context.Background(), id)
			require.NoError(t, err)
			assert.False(t, started)
		})

		t.Run("running", func(t *testing.T) {
			dc := newTestConn(t)
			id := createContainer(t, dc)
			startContainer(t, dc, id)

			started, err := dc.IsContainerStarted(context.Background(), id)
			require.NoError(t, err)
			assert.True(t, started)
		})

		t.Run("exited", func(t *testing.T) {
			dc := newTestConn(t)
			id := createContainer(t, dc)
			startContainer(t, dc, id)

			_, err := dc.client.ContainerKill(context.Background(), id, client.ContainerKillOptions{})
			require.NoError(t, err)

			started, err := dc.IsContainerStarted(context.Background(), id)
			require.NoError(t, err)
			assert.True(t, started)
		})
	})
}
