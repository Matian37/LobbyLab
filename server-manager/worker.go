package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"server-manager/internal"
	"sync"
)

const (
	WorkerStarting WorkerState = iota
	WorkerFree
	WorkerOccupied
)

var (
	ErrMgrNotInit         = errors.New("server manager not initialized")
	ErrMgrAlreadyInit     = errors.New("server manager already initialized")
	ErrMgrClosed          = errors.New("server manager closed")
	ErrMgrAlreadyClosed   = errors.New("server manager already closed")
	ErrFreeWorkerNotFound = errors.New("free worker not found")
	ErrPongLoopFailure    = errors.New("pong loop failure")
)

type WorkerState int

type WorkerInfo struct {
	id string

	state   WorkerState
	matchID int // NOTE: value zero of matchID means no match is happening

	failCount int
}

func NewWorkerInfo(id string) *WorkerInfo {
	return &WorkerInfo{state: WorkerStarting, id: id}
}

func (wi *WorkerInfo) SetStarting() {
	wi.state = WorkerStarting
	wi.failCount = 0
	wi.matchID = 0
}

func (wi *WorkerInfo) SetFree() {
	wi.state = WorkerFree
	wi.matchID = 0
}

func (wi *WorkerInfo) SetOccupied(matchID int) {
	wi.state = WorkerOccupied
	wi.matchID = matchID
}

type WorkerManager struct {
	dockerConn internal.DockerConnection
	brokerConn internal.BrokerConnection
	dbConn     internal.DatabaseConnection

	workers          []*WorkerInfo
	workerFreeNotify chan struct{}

	maxPingRetries int
	publicHost     string

	initialized bool
	closed      bool

	mu sync.Mutex
}

// TODO: add logging

func NewWorkerManager() *WorkerManager {
	return &WorkerManager{
		dockerConn:     NewDockerConnection(),
		brokerConn:     NewNATSConnection(),
		maxPingRetries: 3,
	}
}

// initializes connections and spawns workers
func (wm *WorkerManager) Init(
	ctx context.Context,
	config *internal.EnvConfig,
	workerCount int,
) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if wm.closed {
		return ErrMgrClosed
	}
	if wm.initialized {
		return ErrMgrAlreadyInit
	}

	if err := wm.dbConn.Init(ctx, config); err != nil {
		return err
	}
	if err := wm.brokerConn.Open(ctx, config); err != nil {
		return err
	}
	if err := wm.dockerConn.Init(config); err != nil {
		return err
	}

	for i := 0; i < workerCount; i++ {
		id, err := wm.dockerConn.SpawnContainer(ctx)
		if err != nil {
			return err
		}
		wm.workers = append(wm.workers, NewWorkerInfo(id))
	}

	wm.workerFreeNotify = make(chan struct{})

	return nil
}

func (wm *WorkerManager) WaitForFreeWorker(ctx context.Context) error {
	wm.mu.Lock()

	if !wm.initialized {
		return ErrMgrNotInit
	}
	if wm.closed {
		return ErrMgrClosed
	}

	for _, worker := range wm.workers {
		if worker.state != WorkerFree {
			continue
		}
		wm.mu.Unlock()
		return nil
	}

	resultChan := make(chan error, 1)
	go func() {
		select {
		case <-wm.workerFreeNotify:
			resultChan <- nil
		case <-ctx.Done():
			resultChan <- ctx.Err()
		}
	}()

	// unlock to allow for worker state changes
	wm.mu.Unlock()

	return <-resultChan
}

func (wm *WorkerManager) AssignMatch(
	ctx context.Context,
	matchID int,
	config string,
) (*internal.ServerEndpoints, error) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if !wm.initialized {
		return nil, ErrMgrNotInit
	}
	if wm.closed {
		return nil, ErrMgrClosed
	}

	worker, err := wm.getFreeWorker()
	if err != nil {
		return nil, err
	}

	portMap, err := wm.dockerConn.GetClientPortMap(ctx, worker.id)
	if err != nil {
		return nil, err
	}

	err = wm.brokerConn.AssignJob(ctx, worker.id, config)
	if err != nil {
		return nil, err
	}
	worker.SetOccupied(matchID)

	return &internal.ServerEndpoints{Host: wm.publicHost, Ports: portMap}, nil
}

func (wm *WorkerManager) HealthLoop(ctx context.Context) error {
	for ctx.Done() != nil {
		err := wm.healthCheck(ctx)

		if err != nil {
			slog.Error("encountered error during workers healthcheck", "error", err)
			// TODO: if sleep didn't happen due to error then loop would spin forever
		}
	}
	return ctx.Err()
}

func (wm *WorkerManager) SaveLoop(ctx context.Context) error {
	// TODO: make sure loop won't be spinning
	for {
		msg, err := wm.brokerConn.GetResult(ctx)

		if err != nil {
			slog.Error("encountered error during getting match result", "error", err)
			continue
		}

		var res internal.Result

		if err := json.Unmarshal(msg.Data(), &res); err != nil {
			slog.Error("result unmarshal failed", "err", err)
			continue
		}

		err = wm.dbConn.SaveMatchResult(ctx, res.Success, string(res.Details))
		if err != nil {
			slog.Error("failed to save match result", "err", err)
			continue
		}

		err = msg.Ack()
		slog.Error("failed to ack match result", "error", err)
	}
}

func (wm *WorkerManager) SetWorkerFreeLoop(ctx context.Context) error {
	for ctx.Done() != nil {
		workerID, err := wm.brokerConn.GetFinish(ctx)
		if err != nil {
			slog.Error("failed to get finished worker", "error", err)
		}

		wm.mu.Lock()

		worker := wm.findWorkerByID(workerID)
		if worker == nil {
			slog.Error("finished worker does not exist", "id", workerID)
			continue
		}

		// TODO: may fail if worker restarted and got new job
		// this can only happen in extereme cases, but can have severe consequences
		// solution: make game-server send matchID and use findWorkerByMatchID here
		// but to make this work, game-server must know matchID and it currently does not
		if worker.state == WorkerOccupied {
			worker.SetFree()
		}

		wm.mu.Unlock()
	}
	return ctx.Err()
}

func (wm *WorkerManager) Close(ctx context.Context) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if !wm.initialized {
		return ErrMgrNotInit
	}
	if wm.closed {
		return ErrMgrClosed
	}

	wm.closeContainers(ctx)

	_ = wm.dbConn.Close()
	_ = wm.brokerConn.Close()
	_ = wm.dockerConn.Close()

	close(wm.workerFreeNotify)

	wm.closed = true
	return nil
}

func (wm *WorkerManager) cancelMatch(ctx context.Context) error {
	return wm.dbConn.SaveMatchResult(ctx, false, "{}")
}

// Returns pointer to work if it exists otherwise nil
func (wm *WorkerManager) getFreeWorker() (*WorkerInfo, error) {
	for _, worker := range wm.workers {
		if worker.state == WorkerFree {
			return worker, nil
		}
	}
	return nil, ErrFreeWorkerNotFound
}

func (wm *WorkerManager) findWorkerByID(workerID string) *WorkerInfo {
	for _, worker := range wm.workers {
		if workerID == worker.id {
			return worker
		}
	}
	return nil
}

func (wm *WorkerManager) closeContainers(ctx context.Context) {
	for _, worker := range wm.workers {
		err := wm.dockerConn.KillContainer(ctx, worker.id)
		slog.Error("failed to kill container", "error", err)
	}
}

func (wm *WorkerManager) healthCheck(ctx context.Context) error {
	if !wm.initialized {
		return ErrMgrNotInit
	}
	if wm.closed {
		return ErrMgrClosed
	}

	responders, err := wm.brokerConn.SendPing()
	if err != nil {
		return err
	}

	wm.mu.Lock()
	defer wm.mu.Unlock()

	for _, worker := range wm.workers {
		// skip workers who sent pong
		if _, ok := responders[worker.id]; ok {
			if worker.state == WorkerStarting {
				worker.SetFree()
			} else {
				worker.failCount = 0
			}
			continue
		}

		if worker.state == WorkerStarting {
			started, err := wm.dockerConn.IsContainerStarted(ctx, worker.id)
			if err != nil {
				slog.Error("failed to get starting container state", "id", worker.id, "error", err)
				continue
			}
			if started {
				worker.SetFree()
			}
			continue
		}

		worker.failCount += 1
		if worker.failCount <= wm.maxPingRetries {
			continue
		}

		// TODO: make it concurrent?
		if worker.matchID != 0 {
			wm.cancelMatch(ctx)
		}

		err = wm.dockerConn.RestartContainer(ctx, worker.id)
		if err != nil {
			slog.Error("error during container restart", "id", worker.id, "error", err)
		}
		worker.SetStarting()
	}

	return nil
}
