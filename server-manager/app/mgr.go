package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/Matian37/multiplayer-asset/server-manager/adapters"
	"github.com/Matian37/multiplayer-asset/server-manager/internal"

	"github.com/cenkalti/backoff/v6"
)

// Errors returned by WorkerManager operations.
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

// WorkerManager handles the life cycle of server-manager workers.
//
// It runs three loops:
//  1. Checking worker health (healthLoop)
//     * pings workers periodically.
//     * restarts unhealthy ones.
//  2. Consuming worker results (resultLoop)
//     * fetches results from the broker.
//     * frees the worker from its match.
//     * forwards results to saveLoop to be saved.
//  3. Saving match results (saveLoop)
//     * reads results from an internal channel.
//     * saves them to the database.
//     * clears the affected users' match status so they can rejoin matchmaking.
//
// DatabaseConnection is specifically tied to saveLoop,
// because it's not thread-safe and blocks resultLoop from
// processing other events.
//
// FIX: Connection implementations does not use mutexes for internal states
// and this can cause issues for all beside dbConn.
// Preferably add local end-to-end race test to detect it.
//
// WorkerManager exposes functions to allow caller for
// waiting for a free worker and assigning match to any free one
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
	// Notifies when new free worker appears
	newfreeWorker *sync.Cond
	// Whether the manager is waiting for a free worker (used only for tests)
	waiting bool

	logger *slog.Logger

	// Used for worker structs to ensure thread-safety
	mu sync.Mutex
	// Used for WorkerManager loops
	wg sync.WaitGroup
}

// NewWorkerManager builds a WorkerManager from the given config.
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

// Start opens connections, removes zombie containers, spawns workers and runs
// the main loops.
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

	for idx := range wm.workerCount {
		workerID := strconv.Itoa(idx)

		containerID, err := wm.dockerConn.SpawnContainer(ctx, workerID)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrFailedToSpawnWorker, err)
		}
		worker := NewWorker(workerID, containerID, wm.workerMaxPingRetries, wm.workerRestartTimeout)
		wm.workers = append(wm.workers, worker)
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

// Shutdown closes connections, kills containers and stops the loops.
func (wm *WorkerManager) Shutdown() {
	if wm.closed {
		wm.logger.Warn("already closed")
		return
	}

	for _, worker := range wm.workers {
		wm.logger.Debug("killing worker", "worker", worker.ID, "container", worker.ContainerID)
		err := wm.dockerConn.RemoveContainer(context.Background(), worker.ContainerID)
		if err != nil {
			wm.logger.Error(
				"failed to kill worker",
				"worker", worker.ID,
				"container", worker.ContainerID,
				"error", err,
			)
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

// saveLoop is the loop whose responsibility is receiving match save requests
// from resultLoop and saving them. It also frees users and allows them to
// start matchmaking again.
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

// resultLoop fetches results from workers, frees them and redirects each
// result to saveLoop.
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

// healthLoop periodically pings workers and restarts them when they stop
// responding.
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

// WaitForFreeWorker blocks until at least one worker becomes free, or until
// ctx is canceled.
func (wm *WorkerManager) WaitForFreeWorker(ctx context.Context) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	// Wakes up newFreeWorker.Wait() below when ctx is done to perform shutdown
	stop := context.AfterFunc(ctx, func() {
		wm.mu.Lock()
		defer wm.mu.Unlock()
		// Sends signal to wake up one and only waiter
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

// AssignMatch selects a free worker and assigns the match config to it.
// It returns the externally reachable game address of the worker container.
// If no free worker is found then it returns ErrNoFreeWorker.
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

	port, err := wm.dockerConn.GetGamePort(ctx, worker.ContainerID)
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

// handleResults is the core function of resultLoop for handling match results.
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
		wm.logger.Info(
			"worker ready to handle matches",
			"worker", worker.ID,
			"container", worker.ContainerID,
		)
	} else {
		wm.logger.Debug("worker not found for match", "matchID", res.MatchID)
	}

	return nil
}

// healthCheck is the core function of healthLoop for checking worker health.
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

		wm.logger.Info(
			"worker unhealthy, restarting",
			"worker", worker.ID,
			"container", worker.ContainerID,
		)

		if worker.State == WorkerOccupied {
			wm.logger.Info(
				"canceling match due to worker's health",
				"worker", worker.ID,
				"container", worker.ContainerID,
				"matchID", worker.MatchID,
			)
			wm.saveResultChan <- internal.Result{
				MatchID: worker.MatchID,
				Success: false,
				Details: []byte("{}"),
			}
		} else {
			wm.logger.Debug(
				"worker not running any match, skipping match cancelation",
				"worker", worker.ID,
				"container", worker.ContainerID,
			)
		}

		stateID := worker.SetRestarting()

		wm.wg.Go(func() {
			err := wm.restartWorker(ctx, worker, stateID)
			if err != nil && !errors.Is(err, ErrWorkerStateChanged) {
				wm.logger.Error(
					"failed to restart worker",
					"worker", worker.ID,
					"container", worker.ContainerID,
					"error", err,
				)
			}
		})
	}

	return nil
}

// restartWorker restarts the given worker. The function is thread-safe.
func (wm *WorkerManager) restartWorker(ctx context.Context, worker *Worker, restartStateID int) error {
	err := wm.dockerConn.RestartContainer(ctx, worker.ContainerID)
	if err != nil {
		return err
	}

	wm.mu.Lock()
	defer wm.mu.Unlock()

	if worker.StateID != restartStateID {
		return ErrWorkerStateChanged
	}

	worker.SetFree()
	wm.newfreeWorker.Signal()

	return nil
}

func (wm *WorkerManager) getWorkerByMatchID(matchID int) *Worker {
	for _, worker := range wm.workers {
		if worker.MatchID == matchID {
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
