package app

import (
	"time"
)

// WorkerState enumerates the availability states a worker can be in.
type WorkerState int

const (
	// WorkerFree means that the worker is idle and can accept a new match.
	WorkerFree WorkerState = iota
	// WorkerOccupied means that the worker is currently running a match.
	WorkerOccupied
	// WorkerRestarting means that the worker's container is being restarted.
	WorkerRestarting
)

// Worker tracks a single game-server container and the state it is currently
// in.
type Worker struct {
	// ID is the unique identifier assigned by the server manager.
	// It is different from the container ID, which is assigned by Docker.
	ID          string
	ContainerID string

	State WorkerState
	// StateID identifies the current state of the worker.
	// It is monotonically incremented whenever the worker's state changes.
	// It is used to detect concurrent state changes.
	StateID         int
	stateValidUntil time.Time

	// MatchID identifies the match currently running on the worker.
	// Value of zero means that the worker has no match assigned.
	MatchID int

	// failCount counts the number of consecutive failed pings received from
	// the worker.
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

// SetFree marks the worker as free.
// It returns the new, monotonically incremented state ID.
func (w *Worker) SetFree() int {
	w.State = WorkerFree
	w.StateID++
	w.stateValidUntil = time.Time{}

	w.MatchID = 0
	w.failCount = 0

	return w.StateID
}

// SetRestarting marks the worker as restarting.
// It sets a validity timeout on the state of restartTimeout.
// It returns the new, monotonically incremented state ID.
func (w *Worker) SetRestarting() int {
	w.State = WorkerRestarting
	w.StateID++
	w.stateValidUntil = time.Now().Add(w.restartTimeout)

	w.MatchID = 0
	w.failCount = 0

	return w.StateID
}

// SetOccupied marks the worker as busy with the given match.
// It returns the new, monotonically incremented state ID.
func (w *Worker) SetOccupied(matchID int) int {
	w.State = WorkerOccupied
	w.StateID++
	w.stateValidUntil = time.Time{}

	w.MatchID = matchID
	w.failCount = 0

	return w.StateID
}

// HandlePong updates the worker's failure counter based on the outcome of a
// health ping.
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

// isStateValid checks if the state validity time did not expire.
// TODO: this can be simplified; set stateValidUntil to math.MaxInt64 in other parts of code
// instead of value zero. This won't break anything and will remove IsZero() case in condition
func (w *Worker) isStateValid() bool {
	return w.stateValidUntil.IsZero() || time.Now().Before(w.stateValidUntil)
}

// IsHealthy reports whether the worker is in a valid state and has not
// exceeded its allowed consecutive failed pings.
func (w *Worker) IsHealthy() bool {
	return w.isStateValid() && w.failCount <= w.maxPingRetries
}
