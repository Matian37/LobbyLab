package main

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
	ID string

	State           WorkerState
	stateID         int
	stateValidUntil time.Time
	matchID         int
	failCount       int

	maxPingRetries int
	restartTimeout time.Duration
}

func NewWorker(id string, maxPingRetries int, restartTimeout time.Duration) *Worker {
	return &Worker{
		ID:             id,
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
	switch w.State {
	case WorkerRestarting:
		if pong {
			w.SetFree()
		}
	default:
		if !pong {
			w.failCount++
		} else {
			w.failCount = 0
		}
	}
}

func (w *Worker) isStateValid() bool {
	return w.stateValidUntil.IsZero() || time.Now().Before(w.stateValidUntil)
}

func (w *Worker) IsHealthy() bool {
	return w.isStateValid() && w.failCount <= w.maxPingRetries
}
