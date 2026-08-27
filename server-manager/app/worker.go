package app

import (
	"time"
)

// Enumerates the availability states a worker can be in.
type WorkerState int

const (
	// Means that worker is idle and can accept a new match.
	WorkerFree WorkerState = iota
	// Means that worker is currently running a match.
	WorkerOccupied
	// Means that worker's container is being restarted.
	WorkerRestarting
)

// Tracks a single game-server container and the state it is currently in.
type Worker struct {
	// Unique identifier assigned by the server manager.
	// It is different from the container ID, which is assigned by Docker.
	ID          string
	ContainerID string

	State WorkerState
	// Identifies the current state of the worker.
	// Monotonically incremented whenever the worker's state changes.
	// It is used to detect concurrent state changes.
	StateID         int
	stateValidUntil time.Time

	// Identifies the match currently running on the worker.
	// Value of zero means that worker has no match assigned.
	MatchID int

	// Counts the number of consecutive failed pings received from the worker.
	failCount int

	maxPingRetries int
	restartTimeout time.Duration
}

func NewWorker(workerID string, containerID string, maxPingRetries int, restartTimeout time.Duration) *Worker {
	return &Worker{
		ID:             workerID,
		ContainerID:    containerID,
		State:          WorkerFree,
		maxPingRetries: maxPingRetries,
		restartTimeout: restartTimeout,
	}
}

// Marks the worker to the free state.
// It returns the new, monotonically incremented state ID.
func (w *Worker) SetFree() int {
	w.State = WorkerFree
	w.StateID++
	w.stateValidUntil = time.Time{}

	w.MatchID = 0
	w.failCount = 0

	return w.StateID
}

// Marks the worker as restarting.
// It sets timeout for state to restartTimeout.
// It returns the new, monotonically incremented state ID.
func (w *Worker) SetRestarting() int {
	w.State = WorkerRestarting
	w.StateID++
	w.stateValidUntil = time.Now().Add(w.restartTimeout)

	w.MatchID = 0
	w.failCount = 0

	return w.StateID
}

// Marks the worker as busy with the match.
// It returns the new, monotonically incremented state ID.
func (w *Worker) SetOccupied(matchID int) int {
	w.State = WorkerOccupied
	w.StateID++
	w.stateValidUntil = time.Time{}

	w.MatchID = matchID
	w.failCount = 0

	return w.StateID
}

// Updates the worker's failure counter based on the outcome of a health ping.
func (w *Worker) HandlePong(pong bool) {
	if w.State == WorkerRestarting {
		// Pong arrived before container was restarted, so skip
		return
	}

	if !pong {
		w.failCount++
	} else {
		w.failCount = 0
	}
}

// Checks if state valid time did not expire.
// TODO: this can be simplified; set stateValidUntil to math.MaxInt64 in other parts of code
// instead of value zero. This won't break anything and will remove IsZero() case in condition
func (w *Worker) isStateValid() bool {
	return w.stateValidUntil.IsZero() || time.Now().Before(w.stateValidUntil)
}

// Reports whether the worker is in a valid state and has not exceeded its
// allowed consecutive failed pings.
func (w *Worker) IsHealthy() bool {
	return w.isStateValid() && w.failCount <= w.maxPingRetries
}
