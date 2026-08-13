//go:build e2e

package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

func isContainerMine(t *testing.T, c *container.Summary) bool {
	t.Helper()
	return c.Labels["com.github.multiplayer-asset.worker"] == "true"
}

func isContainerHealthy(t *testing.T, c *container.Summary) bool {
	t.Helper()
	return c.State == container.StateRunning
}

func getContainerWorkerID(t *testing.T, containerID string, cli *client.Client) (string, error) {
	t.Helper()

	res, err := cli.ContainerInspect(context.Background(), containerID, client.ContainerInspectOptions{})
	if err != nil {
		return "", errors.New("failed to inspect container")
	}
	if res.Container.Config == nil {
		return "", errors.New("container config is nil")
	}

	for _, env := range res.Container.Config.Env {
		if !strings.HasPrefix(env, "WORKER_ID=") {
			continue
		}
		return strings.TrimPrefix(env, "WORKER_ID="), nil
	}

	return "", errors.New("worker ID not found")
}

func forEachHealthyWorker(t *testing.T, cli *client.Client, errChan chan error, fn func(containerID string, workerID string)) {
	t.Helper()
	containers, err := cli.ContainerList(context.Background(), client.ContainerListOptions{})
	if err != nil {
		t.Fatalf("failed to list containers: %v", err)
	}

	for _, c := range containers.Items {
		if !isContainerMine(t, &c) {
			continue
		}
		if !isContainerHealthy(t, &c) {
			continue
		}

		workerID, err := getContainerWorkerID(t, c.ID, cli)
		if err != nil {
			errChan <- err
			continue
		}
		fn(c.ID, workerID)
	}
}
