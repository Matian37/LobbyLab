package adapters

import (
	"context"
	"errors"
	"log/slog"
	"server-manager/internal"
	"time"

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

func (dc *DockerConnection) RestartContainer(ctx context.Context, id string) error {
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
		id,
		client.ContainerRestartOptions{Timeout: &dc.containerStopTimeout},
	)
	if err != nil {
		return err
	}
	return nil
}

func (dc *DockerConnection) RemoveContainer(ctx context.Context, id string) error {
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
		id,
		client.ContainerRemoveOptions{Force: true, RemoveVolumes: true},
	)
	if err != nil {
		return err
	}
	return nil
}

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

// generates mapping of given ports to unspecified host bindings
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

// NOTE: portMap must have unspecified host ports
// NOTE: due to container spawning nature, logs cannot be attached to compose logs
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
			// init is required for game-server to reap abandoned child processes
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
