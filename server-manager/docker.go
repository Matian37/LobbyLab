package main

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
	ErrImageEnvNotFound  = errors.New("game server image env not found")
	ErrClientNotInit     = errors.New("client not initialized")
	ErrClientAlreadyInit = errors.New("client already initialized")
	ErrClientClosed      = errors.New("client closed")
)

type DockerClient struct {
	client *client.Client
	config *internal.EnvConfig

	createTimeout           time.Duration
	killTimeout             time.Duration
	restartTimeout          time.Duration
	inspectTimeout          time.Duration
	containerRestartTimeout int // in seconds

	initialized bool
	closed      bool
}

func NewDockerClient() *DockerClient {
	return &DockerClient{
		createTimeout:           5 * time.Second,
		restartTimeout:          40 * time.Second,
		killTimeout:             5 * time.Second,
		inspectTimeout:          5 * time.Second,
		containerRestartTimeout: 30,
	}
}

func (dc *DockerClient) Init(config *internal.EnvConfig) error {
	if dc.initialized {
		return ErrClientAlreadyInit
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

func (dc *DockerClient) CreateWorkerContainer(ctx context.Context) (string, error) {
	if !dc.initialized {
		return "", ErrClientNotInit
	}
	if dc.closed {
		return "", ErrClientClosed
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, dc.createTimeout)
	defer cancel()

	res, err := dc.client.ContainerCreate(timeoutCtx,
		client.ContainerCreateOptions{
			Image: dc.config.Image,
			HostConfig: &container.HostConfig{
				PortBindings: genPortMap(dc.config.ExposePorts),
				RestartPolicy: container.RestartPolicy{
					Name:              container.RestartPolicyDisabled,
					MaximumRetryCount: 0,
				},
			},
		},
	)
	if err != nil {
		return "", err
	}

	if len(res.Warnings) != 0 {
		slog.Warn("container creation warnings", "id", res.ID, "warnings", res.Warnings)
	}

	return res.ID, nil
}

func (dc *DockerClient) RestartContainer(ctx context.Context, id string) error {
	if !dc.initialized {
		return ErrClientNotInit
	}
	if dc.closed {
		return ErrClientClosed
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, dc.restartTimeout)
	defer cancel()

	_, err := dc.client.ContainerRestart(
		timeoutCtx,
		id,
		client.ContainerRestartOptions{Timeout: &dc.containerRestartTimeout},
	)
	if err != nil {
		return err
	}
	return nil
}

func (dc *DockerClient) KillContainer(ctx context.Context, id string) error {
	if !dc.initialized {
		return ErrClientNotInit
	}
	if dc.closed {
		return ErrClientClosed
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, dc.killTimeout)
	defer cancel()

	_, err := dc.client.ContainerKill(timeoutCtx, id, client.ContainerKillOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (dc *DockerClient) GetGamePorts(ctx context.Context, containerID string) (network.PortMap, error) {
	if !dc.initialized {
		return nil, ErrClientNotInit
	}
	if dc.closed {
		return nil, ErrClientClosed
	}

	portMap, err := dc.getPorts(ctx, containerID)
	if err != nil {
		return nil, err
	}
	return dc.filterPorts(portMap, containerID), nil
}

func (dc *DockerClient) IsContainerStarted(ctx context.Context, containerID string) (bool, error) {
	if !dc.initialized {
		return false, ErrClientNotInit
	}
	if dc.closed {
		return false, ErrClientClosed
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, dc.inspectTimeout)
	defer cancel()

	res, err := dc.client.ContainerInspect(timeoutCtx, containerID, client.ContainerInspectOptions{})
	if err != nil {
		return false, err
	}

	status := res.Container.State.Status
	return status == container.StateRunning || status == container.StateExited, nil
}

func (dc *DockerClient) Close() error {
	return dc.client.Close()
}

// generates mapping of given ports to unspecified host bindings
func genPortMap(ports map[network.Port]struct{}) network.PortMap {
	portBindings := network.PortMap{}
	for port := range ports {
		portBindings[port] = []network.PortBinding{{}}
	}
	return portBindings
}

// filter ports to only include client ports from config
func (dc *DockerClient) filterPorts(portMap network.PortMap, containerID string) network.PortMap {
	for port, bindings := range portMap {
		if _, ok := dc.config.ClientPorts[port]; !ok {
			delete(portMap, port)
		}
		portMap[port] = bindings[0:1]
	}
	return portMap
}

func (dc *DockerClient) getPorts(ctx context.Context, containerID string) (network.PortMap, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, dc.inspectTimeout)
	defer cancel()

	res, err := dc.client.ContainerInspect(timeoutCtx, containerID, client.ContainerInspectOptions{})
	if err != nil {
		return nil, err
	}
	return res.Container.NetworkSettings.Ports, nil
}
