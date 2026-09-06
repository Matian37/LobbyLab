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

type ForEachConfig struct {
	// Docker client
	client *client.Client
	// Error channel to report errors
	errChan chan error
	// The number of workers used to validate if worker IDs are correct
	workerCount int
}

type MatchingConfig struct {
	ForEachConfig
	// Worker ID to match against
	workerID string
}

func isContainerMine(t *testing.T, c *container.Summary) bool {
	t.Helper()
	return c.Labels["com.github.Matian37.LobbyLab.service"] == "game-server"
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
	cfg ForEachConfig,
	fn func(containerID string, workerID string),
) {
	t.Helper()
	containers, err := cfg.client.ContainerList(context.Background(), client.ContainerListOptions{})
	if err != nil {
		cfg.errChan <- fmt.Errorf("failed to list containers: %w", err)
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

		workerID, err := getContainerWorkerID(t, c.ID, cfg.client)
		if err != nil {
			cfg.errChan <- err
			continue
		}
		if !isWorkerIDValid(workerID, cfg.workerCount) {
			cfg.errChan <- fmt.Errorf("unexpected worker id: %s", workerID)
			continue
		}
		if _, ok := seen[workerID]; ok {
			cfg.errChan <- fmt.Errorf("duplicate worker id: %s", workerID)
			continue
		}
		seen[workerID] = struct{}{}

		fn(c.ID, workerID)
	}
}

func forMatchingWorker(
	t *testing.T,
	cfg MatchingConfig,
	fn func(),
) {
	t.Helper()

	matched := false
	forEachHealthyWorker(t, cfg.ForEachConfig, func(_ string, id string) {
		if cfg.workerID != id {
			return
		}
		matched = true
		fn()
	})

	if !matched {
		cfg.errChan <- fmt.Errorf("unknown worker id: %s", cfg.workerID)
	}
}

func isWorkerIDValid(workerID string, workerCount int) bool {
	idx, err := strconv.Atoi(workerID)
	return err == nil && idx >= 0 && idx < workerCount
}
