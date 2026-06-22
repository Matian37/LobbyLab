package main

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
