package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"server-manager/adapters"
	"server-manager/internal"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v6"
)

var (
	ErrMgrAlreadyInit          = errors.New("manager already initialized")
	ErrMgrAlreadyClosed        = errors.New("manager already closed")
	ErrMgrClosed               = errors.New("manager closed")
	ErrMgrNoInit               = errors.New("manager no init")
	ErrFailedToSpawnWorker     = errors.New("failed to spawn worker")
	ErrFailedToGetResult       = errors.New("failed to get result")
	ErrFailedToUnmarshalResult = errors.New("failed to unmarshal result")
	ErrFailedToAckResult       = errors.New("failed to ack result")
	ErrHealthPingFailed        = errors.New("health ping failed")
	ErrNoFreeWorker            = errors.New("free worker not found")
	ErrWorkerRestartFailed     = errors.New("failed to restart worker")
	ErrWorkerStateChanged      = errors.New("worker changed state during restart")
)

type WorkerManager struct {
	workers []*Worker

	dockerConn internal.DockerConnection
	brokerConn internal.BrokerConnection
	dbConn     internal.DatabaseConnection

	initialized bool
	closed      bool

	config *internal.EnvConfig

	healthCheckTick      time.Duration
	resultChanSize       int
	workerCount          int
	workerMaxPingRetries int
	workerRestartTimeout time.Duration
	workerPongTimeout    time.Duration

	saveResultChan chan internal.Result
	newfreeWorker  *sync.Cond
	// whether the manager is waiting for a free worker (tests only)
	waiting bool

	mu sync.Mutex
	wg sync.WaitGroup
}

func NewWorkerManager(config *internal.EnvConfig) *WorkerManager {
	return &WorkerManager{
		dockerConn:           adapters.NewDockerConnection(),
		brokerConn:           adapters.NewNATSConnection(),
		dbConn:               adapters.NewDatabaseConnection(config),
		config:               config,
		healthCheckTick:      3 * time.Second,
		resultChanSize:       8192,
		workerCount:          config.Workercount,
		workerMaxPingRetries: 3,
		workerRestartTimeout: 30 * time.Second,
		workerPongTimeout:    2 * time.Second,
	}
}

func (wm *WorkerManager) Start(ctx context.Context) error {
	if wm.closed {
		return ErrMgrClosed
	}
	if wm.initialized {
		return ErrMgrAlreadyInit
	}

	if err := wm.dockerConn.Open(wm.config); err != nil {
		return err
	}
	if err := wm.brokerConn.Open(ctx, wm.config); err != nil {
		return err
	}
	if err := wm.dbConn.Open(ctx); err != nil {
		return err
	}

	for range wm.workerCount {
		id, err := wm.dockerConn.SpawnContainer(ctx)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrFailedToSpawnWorker, err)
		}
		wm.workers = append(wm.workers, NewWorker(id, wm.workerMaxPingRetries, wm.workerRestartTimeout))
	}

	wm.saveResultChan = make(chan internal.Result, wm.resultChanSize)
	wm.newfreeWorker = sync.NewCond(&wm.mu)

	wm.initialized = true

	wm.wg.Go(func() { _ = wm.saveLoop(ctx) })
	wm.wg.Go(func() { _ = wm.resultLoop(ctx) })
	wm.wg.Go(func() { _ = wm.healthLoop(ctx) })

	return nil
}

// TODO: error logging
func (wm *WorkerManager) Shutdown() error {
	if wm.closed {
		return ErrMgrAlreadyClosed
	}

	for _, worker := range wm.workers {
		_ = wm.dockerConn.KillContainer(context.Background(), worker.ID)
	}

	_ = wm.dockerConn.Close()
	_ = wm.brokerConn.Close()
	_ = wm.dbConn.Close()

	wm.wg.Wait()

	wm.closed = true
	return nil
}

func (wm *WorkerManager) saveLoop(ctx context.Context) error {
	if !wm.initialized {
		return ErrMgrNoInit
	}
	if wm.closed {
		return ErrMgrClosed
	}

	b := backoff.NewExponentialBackOff()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case res := <-wm.saveResultChan:
			err := wm.dbConn.SaveMatchResults(ctx, string(res.Details), res.MatchID)
			if err != nil {
				slog.Error("failed to save result", "error", err)
			}
			HandleBackoff(ctx, b, err)
		}
	}
}

func (wm *WorkerManager) resultLoop(ctx context.Context) error {
	if !wm.initialized {
		return ErrMgrNoInit
	}
	if wm.closed {
		return ErrMgrClosed
	}

	b := backoff.NewExponentialBackOff()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			err := wm.handleResults(ctx)
			if err != nil {
				slog.Error("result loop error", "error", err)
			}
			HandleBackoff(ctx, b, err)
		}
	}
}

func (wm *WorkerManager) healthLoop(ctx context.Context) error {
	if !wm.initialized {
		return ErrMgrNoInit
	}
	if wm.closed {
		return ErrMgrClosed
	}

	ticker := time.NewTicker(wm.healthCheckTick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := wm.healthCheck(ctx); err != nil {
				slog.Error("failed to do health check for workers", "error", err)
			}
		}
	}
}

func (wm *WorkerManager) WaitForFreeWorker(ctx context.Context) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	stop := context.AfterFunc(ctx, func() {
		wm.mu.Lock()
		defer wm.mu.Unlock()
		wm.newfreeWorker.Signal()
	})
	defer stop()

	for wm.getFreeWorker() == nil {
		if ctx.Err() != nil {
			return
		}

		// adds extra info for tests
		wm.waiting = true
		defer func() { wm.waiting = false }()

		wm.newfreeWorker.Wait()
	}
}

func (wm *WorkerManager) AssignMatch(ctx context.Context, config internal.MatchConfig) (internal.ServerInfo, error) {
	if !wm.initialized {
		return internal.ServerInfo{}, ErrMgrNoInit
	}
	if wm.closed {
		return internal.ServerInfo{}, ErrMgrClosed
	}

	wm.mu.Lock()
	defer wm.mu.Unlock()

	worker := wm.getFreeWorker()
	if worker == nil {
		return internal.ServerInfo{}, ErrNoFreeWorker
	}

	port, err := wm.dockerConn.GetGamePort(ctx, worker.ID)
	if err != nil {
		return internal.ServerInfo{}, err
	}

	err = wm.brokerConn.AssignJob(ctx, worker.ID, config)
	if err != nil {
		return internal.ServerInfo{}, err
	}
	worker.SetOccupied(config.MatchID)

	return internal.ServerInfo{Host: wm.config.PublicHost, Port: port}, nil
}

func (wm *WorkerManager) handleResults(ctx context.Context) error {
	msg, err := wm.brokerConn.GetResult(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFailedToGetResult, err)
	}

	var res internal.Result
	if err := json.Unmarshal(msg.Data(), &res); err != nil {
		return fmt.Errorf("%w: %w", ErrFailedToUnmarshalResult, err)
	}

	if err := msg.Ack(); err != nil {
		return fmt.Errorf("%w: %w", ErrFailedToAckResult, err)
	}

	wm.saveResultChan <- res

	wm.mu.Lock()
	defer wm.mu.Unlock()

	worker := wm.getWorkerByMatchID(res.MatchID)

	if worker != nil {
		worker.SetFree()
		wm.newfreeWorker.Signal()
	}

	return nil
}

func (wm *WorkerManager) healthCheck(ctx context.Context) error {
	responders, err := wm.brokerConn.GetWorkersPong(ctx, wm.workerPongTimeout)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrHealthPingFailed, err)
	}

	wm.mu.Lock()
	defer wm.mu.Unlock()

	for _, worker := range wm.workers {
		_, pong := responders[worker.ID]
		worker.HandlePong(pong)

		if worker.IsHealthy() {
			continue
		}

		stateID := worker.SetRestarting()

		wm.wg.Go(func() { _ = wm.restartWorker(ctx, worker, stateID, worker.ID) })
	}

	return nil
}

func (wm *WorkerManager) restartWorker(ctx context.Context, worker *Worker, restartStateID int, workerID string) error {
	// TODO: make match canceled in DB
	err := wm.dockerConn.RestartContainer(ctx, workerID)
	if err != nil {
		slog.Error("failed to restart worker", "id", workerID, "error", err)
		return fmt.Errorf("%w: %w", ErrWorkerRestartFailed, err)
	}

	wm.mu.Lock()
	defer wm.mu.Unlock()

	if worker.stateID != restartStateID {
		return ErrWorkerStateChanged
	}

	worker.SetFree()
	wm.newfreeWorker.Signal()

	return nil
}

func (wm *WorkerManager) getWorkerByMatchID(matchID int) *Worker {
	for _, worker := range wm.workers {
		if worker.matchID == matchID {
			return worker
		}
	}
	return nil
}

func (wm *WorkerManager) getFreeWorker() *Worker {
	for _, worker := range wm.workers {
		if worker.State == WorkerFree {
			return worker
		}
	}
	return nil
}
