package app

import (
	"time"
)

type WorkerState int

const (
	WorkerFree WorkerState = iota
	WorkerOccupied
	WorkerRestarting
)

type Worker struct {
	ID          string
	ContainerID string

	State           WorkerState
	stateID         int
	stateValidUntil time.Time
	matchID         int // ids must be positive numbers
	failCount       int

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

func (w *Worker) SetFree() int {
	w.State = WorkerFree
	w.stateID++
	w.stateValidUntil = time.Time{}

	w.matchID = 0
	w.failCount = 0

	return w.stateID
}

func (w *Worker) SetRestarting() int {
	w.State = WorkerRestarting
	w.stateID++
	w.stateValidUntil = time.Now().Add(w.restartTimeout)

	w.matchID = 0
	w.failCount = 0

	return w.stateID
}

func (w *Worker) SetOccupied(matchID int) int {
	w.State = WorkerOccupied
	w.stateID++
	w.stateValidUntil = time.Time{}

	w.matchID = matchID
	w.failCount = 0

	return w.stateID
}

func (w *Worker) HandlePong(pong bool) {
	if w.State == WorkerRestarting {
		// pong arrive before restartWorker changed worker's state
		// best action is to wait for the state to be changed to free
		return
	}

	if !pong {
		w.failCount++
	} else {
		w.failCount = 0
	}
}

func (w *Worker) isStateValid() bool {
	return w.stateValidUntil.IsZero() || time.Now().Before(w.stateValidUntil)
}

func (w *Worker) IsHealthy() bool {
	return w.isStateValid() && w.failCount <= w.maxPingRetries
}
