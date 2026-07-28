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

	logger *slog.Logger

	mu sync.Mutex
	wg sync.WaitGroup
}

func NewWorkerManager(config *internal.EnvConfig, logger *slog.Logger) *WorkerManager {
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
		logger:               logger.With("service", "workerManager"),
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
		return fmt.Errorf("failed to open docker connection: %w", err)
	}
	if err := wm.brokerConn.Open(ctx, wm.config); err != nil {
		return fmt.Errorf("failed to open broker connection: %w", err)
	}
	if err := wm.dbConn.Open(ctx); err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}

	if err := wm.dockerConn.RemoveZombieWorkers(ctx); err != nil {
		return fmt.Errorf("failed to remove zombie workers: %w", err)
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

	attachLogging := func(ctx context.Context, loop func(context.Context) error, loopName string) func() {
		return func() {
			wm.logger.Debug("starting loop", "loop", loopName)
			err := loop(ctx)
			// nil check not required loop shouldn't return it as error
			// context errors will be hidden by handler accordingly
			wm.logger.Error("loop exited", "loop", loopName, "error", err)
		}
	}

	wm.wg.Go(attachLogging(ctx, wm.saveLoop, "saveLoop"))
	wm.wg.Go(attachLogging(ctx, wm.resultLoop, "resultLoop"))
	wm.wg.Go(attachLogging(ctx, wm.healthLoop, "healthLoop"))

	return nil
}

func (wm *WorkerManager) Shutdown() {
	if wm.closed {
		wm.logger.Warn("already closed")
		return
	}

	for _, worker := range wm.workers {
		wm.logger.Debug("killing worker", "worker", worker.ID)
		err := wm.dockerConn.KillContainer(context.Background(), worker.ID)
		if err != nil {
			wm.logger.Error("failed to kill worker", "worker", worker.ID, "error", err)
		}
	}

	if err := wm.dockerConn.Close(); err != nil {
		wm.logger.Error("failed to close docker connection", "error", err)
	}
	if err := wm.brokerConn.Close(); err != nil {
		wm.logger.Error("failed to close broker connection", "error", err)
	}
	if err := wm.dbConn.Close(); err != nil {
		wm.logger.Error("failed to close database connection", "error", err)
	}

	wm.logger.Debug("waiting to finish remaining tasks")

	wm.wg.Wait()
	wm.closed = true

	wm.logger.Debug("shutdown complete")
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
			wm.logger.Debug("received save request", "result", res)

			if err := wm.dbConn.RemoveMatchStatus(ctx, res.MatchID); err != nil {
				wm.logger.Error(
					"failed to remove match status from match users. this will block their matchmaking indefinitely",
					"matchID", res.MatchID, "error", err,
				)
			}

			err := wm.dbConn.SaveMatchResults(ctx, res)
			if err != nil {
				wm.logger.Error("failed to save result", "error", err)
			} else {
				wm.logger.Debug("match result saved", "result", res)
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
				wm.logger.Error("result loop error", "error", err)
			} else {
				wm.logger.Debug("match results handled")
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
				wm.logger.Error("failed to do health check workers", "error", err)
			} else {
				wm.logger.Debug("workers health check completed")
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
	wm.logger.Debug("received match result", "data", string(msg.Data()))

	var res internal.Result
	if err := json.Unmarshal(msg.Data(), &res); err != nil {
		return fmt.Errorf("%w: %w", ErrFailedToUnmarshalResult, err)
	}

	if err := msg.Ack(); err != nil {
		return fmt.Errorf("%w: %w", ErrFailedToAckResult, err)
	}
	wm.logger.Debug("result acked")

	wm.saveResultChan <- res

	wm.mu.Lock()
	defer wm.mu.Unlock()

	worker := wm.getWorkerByMatchID(res.MatchID)

	if worker != nil {
		worker.SetFree()
		wm.newfreeWorker.Signal()
		wm.logger.Info("worker ready to handle matches", "worker", worker.ID)
	} else {
		wm.logger.Debug("worker not found for match", "matchID", res.MatchID)
	}

	return nil
}

func (wm *WorkerManager) healthCheck(ctx context.Context) error {
	responders, err := wm.brokerConn.GetWorkersPong(ctx, wm.workerPongTimeout)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrHealthPingFailed, err)
	}
	wm.logger.Debug("health check completed", "responders", responders)

	wm.mu.Lock()
	defer wm.mu.Unlock()

	for _, worker := range wm.workers {
		_, pong := responders[worker.ID]
		worker.HandlePong(pong)

		if worker.IsHealthy() {
			continue
		}

		wm.logger.Info("worker unhealthy, restarting", "worker", worker.ID)

		if worker.State == WorkerOccupied {
			wm.logger.Info("canceling match due to worker's health", "worker", worker.ID, "matchID", worker.matchID)
			wm.saveResultChan <- internal.Result{
				MatchID: worker.matchID,
				Success: false,
				Details: []byte("{}"),
			}
		} else {
			wm.logger.Debug("worker not running any match, skipping match cancelation", "worker", worker.ID)
		}

		stateID := worker.SetRestarting()

		wm.wg.Go(func() {
			err := wm.restartWorker(ctx, worker, stateID, worker.ID)
			if err != nil && !errors.Is(err, ErrWorkerStateChanged) {
				wm.logger.Error("failed to restart worker", "worker", worker.ID, "error", err)
			}
		})
	}

	return nil
}

func (wm *WorkerManager) restartWorker(ctx context.Context, worker *Worker, restartStateID int, workerID string) error {
	err := wm.dockerConn.RestartContainer(ctx, workerID)
	if err != nil {
		return err
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
