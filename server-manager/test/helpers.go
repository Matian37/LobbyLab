//go:build e2e

package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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
		return "", fmt.Errorf("failed to inspect container: %w", err)
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

func forEachHealthyWorker(
	t *testing.T,
	cli *client.Client,
	errChan chan error,
	workerCount int,
	fn func(containerID string, workerID string),
) {
	t.Helper()
	containers, err := cli.ContainerList(context.Background(), client.ContainerListOptions{})
	if err != nil {
		errChan <- fmt.Errorf("failed to list containers: %w", err)
		return
	}

	seen := make(map[string]struct{})
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
		if !isWorkerIDValid(workerID, workerCount) {
			errChan <- fmt.Errorf("unexpected worker id: %s", workerID)
			continue
		}
		if _, ok := seen[workerID]; ok {
			errChan <- fmt.Errorf("duplicate worker id: %s", workerID)
			continue
		}
		seen[workerID] = struct{}{}

		fn(c.ID, workerID)
	}
}

// forEachMatchingWorker calls fn for the single healthy worker matching
// workerID and returns whether it was found. More than one matching worker is
// reported as a duplicate.
func forMatchingWorker(
	t *testing.T,
	cli *client.Client,
	errChan chan error,
	workerCount int,
	workerID string,
	fn func(),
) bool {
	t.Helper()

	matched := false
	forEachHealthyWorker(t, cli, errChan, workerCount, func(_ string, id string) {
		if workerID != id {
			return
		}
		matched = true
		fn()
	})
	return matched
}

func isWorkerIDValid(workerID string, workerCount int) bool {
	idx, err := strconv.Atoi(workerID)
	return err == nil && idx >= 0 && idx < workerCount
}
