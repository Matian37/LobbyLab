package adapters

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/Matian37/multiplayer-asset/server-manager/internal"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

var (
	ErrImageEnvNotFound      = errors.New("game server image env not found")
	ErrDockerConnNotInit     = errors.New("connection not initialized")
	ErrDockerConnAlreadyInit = errors.New("connection already initialized")
	ErrDockerConnClosed      = errors.New("connection closed")
	ErrGamePortNoBinding     = errors.New("game port binding not found")
)

// DockerConnection implements internal.DockerConnection over the Docker Engine
// API.
type DockerConnection struct {
	client *client.Client
	config *internal.EnvConfig

	createTimeout        time.Duration
	killTimeout          time.Duration
	restartTimeout       time.Duration
	inspectTimeout       time.Duration
	containerStopTimeout int // in seconds

	initialized bool
	closed      bool
}

func NewDockerConnection() *DockerConnection {
	return &DockerConnection{
		createTimeout:        5 * time.Second,
		restartTimeout:       40 * time.Second,
		killTimeout:          5 * time.Second,
		inspectTimeout:       5 * time.Second,
		containerStopTimeout: 5,
	}
}

func (dc *DockerConnection) Open(config *internal.EnvConfig) error {
	if dc.closed {
		return ErrDockerConnClosed
	}
	if dc.initialized {
		return ErrDockerConnAlreadyInit
	}

	client, err := client.New(client.FromEnv)
	if err != nil {
		return err
	}
	dc.client = client

	dc.config = config
	dc.initialized = true

	return nil
}

// SpawnContainer creates and starts a new game-server container for workerID
// and returns its container ID.
func (dc *DockerConnection) SpawnContainer(ctx context.Context, workerID string) (string, error) {
	if !dc.initialized {
		return "", ErrDockerConnNotInit
	}
	if dc.closed {
		return "", ErrDockerConnClosed
	}

	portMap := genPortMap(dc.config.ExposePorts)

	timeoutCtx, cancel := context.WithTimeout(ctx, dc.createTimeout)
	defer cancel()

	res, err := dc.client.ContainerCreate(timeoutCtx, dc.containerCreateOptions(portMap, workerID))
	if err != nil {
		return "", err
	}

	if len(res.Warnings) != 0 {
		slog.Warn("container creation warnings", "id", res.ID, "warnings", res.Warnings)
	}

	_, err = dc.client.ContainerStart(timeoutCtx, res.ID, client.ContainerStartOptions{})
	if err != nil {
		return "", err
	}

	return res.ID, nil
}

func (dc *DockerConnection) RestartContainer(ctx context.Context, containerID string) error {
	if !dc.initialized {
		return ErrDockerConnNotInit
	}
	if dc.closed {
		return ErrDockerConnClosed
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, dc.restartTimeout)
	defer cancel()

	_, err := dc.client.ContainerRestart(
		timeoutCtx,
		containerID,
		client.ContainerRestartOptions{Timeout: &dc.containerStopTimeout},
	)
	if err != nil {
		return err
	}
	return nil
}

// RemoveContainer forcefully removes the given container, discarding any
// volumes it uses.
func (dc *DockerConnection) RemoveContainer(ctx context.Context, containerID string) error {
	if !dc.initialized {
		return ErrDockerConnNotInit
	}
	if dc.closed {
		return ErrDockerConnClosed
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, dc.killTimeout)
	defer cancel()

	_, err := dc.client.ContainerRemove(
		timeoutCtx,
		containerID,
		client.ContainerRemoveOptions{Force: true, RemoveVolumes: true},
	)
	if err != nil {
		return err
	}
	return nil
}

// RemoveZombieWorkers finds and forcefully removes any leftover worker
// containers from a previous run, identified by the worker label.
func (dc *DockerConnection) RemoveZombieWorkers(ctx context.Context) error {
	if !dc.initialized {
		return ErrDockerConnNotInit
	}
	if dc.closed {
		return ErrDockerConnClosed
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, dc.killTimeout)
	defer cancel()

	containers, err := dc.client.ContainerList(timeoutCtx, client.ContainerListOptions{
		All:     true,
		Filters: client.Filters{}.Add("label", "com.github.multiplayer-asset.worker=true"),
	})
	if err != nil {
		return err
	}

	for _, c := range containers.Items {
		_, err := dc.client.ContainerRemove(
			timeoutCtx,
			c.ID,
			client.ContainerRemoveOptions{
				Force:         true,
				RemoveVolumes: true,
			},
		)
		if err != nil {
			slog.Error("failed to kill zombie container", "id", c.ID, "error", err)
		}
	}

	return nil
}

// GetGamePort returns the port assigned for the client to connect to the game
// server.
func (dc *DockerConnection) GetGamePort(ctx context.Context, containerID string) (string, error) {
	if !dc.initialized {
		return "", ErrDockerConnNotInit
	}
	if dc.closed {
		return "", ErrDockerConnClosed
	}

	portMap, err := dc.getPorts(ctx, containerID)
	if err != nil {
		return "", err
	}

	bindings := portMap[dc.config.ClientPort]
	if len(bindings) == 0 {
		return "", ErrGamePortNoBinding
	}
	return bindings[0].HostPort, nil
}

func (dc *DockerConnection) Close() error {
	if !dc.initialized {
		return ErrDockerConnNotInit
	}
	if dc.closed {
		return ErrDockerConnClosed
	}
	err := dc.client.Close()
	dc.closed = true
	return err
}

// Generates mapping of given ports to unspecified host bindings
func genPortMap(ports network.PortSet) network.PortMap {
	portBindings := network.PortMap{}
	for port := range ports {
		portBindings[port] = []network.PortBinding{{}}
	}
	return portBindings
}

func (dc *DockerConnection) getPorts(ctx context.Context, containerID string) (network.PortMap, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, dc.inspectTimeout)
	defer cancel()

	res, err := dc.client.ContainerInspect(timeoutCtx, containerID, client.ContainerInspectOptions{})
	if err != nil {
		return nil, err
	}
	return res.Container.NetworkSettings.Ports, nil
}

// containerCreateOptions creates container options for container spawning.
// For the game-server to properly use the ports, the binding must be
// unspecified. Note: due to container spawning nature, logs cannot be attached
// to compose logs.
func (dc *DockerConnection) containerCreateOptions(
	portMap network.PortMap,
	workerID string,
) client.ContainerCreateOptions {
	options := client.ContainerCreateOptions{
		Image: dc.config.Image,
		Config: &container.Config{
			ExposedPorts: dc.config.ExposePorts,
			Labels: map[string]string{
				"com.github.multiplayer-asset.worker": "true",
			},
			Env: []string{
				"LOG_LEVEL=" + dc.config.GameServerLogLevel.String(),
				"NATS_URI=" + dc.config.BrokerURI,
				"WORKER_ID=" + workerID,
			},
		},
		HostConfig: &container.HostConfig{
			// Init set to true is required for game-server to reap abandoned child processes
			// in case when actual game-server fails to exit gracefully
			Init:         new(true),
			PortBindings: portMap,
			RestartPolicy: container.RestartPolicy{
				Name:              container.RestartPolicyDisabled,
				MaximumRetryCount: 0,
			},
		},
		NetworkingConfig: &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				dc.config.BrokerNetworkName: {},
			},
		},
	}

	if dc.config.TestMakeContainerDummy {
		options.Config.Cmd = []string{"sleep", "inf"}
	}

	return options
}
