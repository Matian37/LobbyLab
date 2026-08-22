package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestWorkerFree(stateID int) Worker {
	return Worker{
		State:           WorkerFree,
		stateID:         stateID,
		stateValidUntil: time.Time{},
		matchID:         0,
	}
}

func newTestWorkerOccupied(stateID int, matchID int) Worker {
	return Worker{
		State:           WorkerOccupied,
		stateID:         stateID,
		stateValidUntil: time.Time{},
		matchID:         matchID,
	}
}

func newTestWorkerRestarting(stateID int, now time.Time) Worker {
	return Worker{
		State:           WorkerRestarting,
		stateID:         stateID,
		stateValidUntil: now.Add(time.Hour),
		matchID:         0,
	}
}

func TestNewWorker(t *testing.T) {
	worker := NewWorker("wid", "cid", 3, 30*time.Second)
	require.NotNil(t, worker)

	assert.Equal(t, "wid", worker.ID)
	assert.Equal(t, "cid", worker.ContainerID)
	assert.Equal(t, WorkerFree, worker.State)
	assert.Equal(t, 3, worker.maxPingRetries)
	assert.Equal(t, 30*time.Second, worker.restartTimeout)
}

func TestWorker_SetFree(t *testing.T) {
	timeNow := time.Now()
	stateID := 3
	matchID := 42

	tests := []struct {
		name   string
		worker Worker
	}{
		{name: "free worker", worker: newTestWorkerFree(stateID)},
		{name: "occupied worker", worker: newTestWorkerOccupied(stateID, matchID)},
		{name: "restarting worker", worker: newTestWorkerRestarting(stateID, timeNow)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expected := newTestWorkerFree(stateID + 1)

			worker := test.worker
			res := worker.SetFree()
			assert.Equal(t, worker.stateID, res)

			assert.Equal(t, expected, worker)
		})
	}
}

func TestWorker_SetRestarting(t *testing.T) {
	timeNow := time.Now()
	stateID := 3
	matchID := 42

	tests := []struct {
		name   string
		worker Worker
	}{
		{name: "free worker", worker: newTestWorkerFree(stateID)},
		{name: "occupied worker", worker: newTestWorkerOccupied(stateID, matchID)},
		{name: "restarting worker", worker: newTestWorkerRestarting(stateID, timeNow)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expected := newTestWorkerRestarting(stateID+1, timeNow)

			worker := test.worker

			nowLowerBound := time.Now()
			res := worker.SetRestarting()
			nowUpperBound := time.Now()

			assert.Equal(t, worker.stateID, res)

			assert.GreaterOrEqual(t, worker.stateValidUntil, nowLowerBound.Add(worker.restartTimeout))
			assert.LessOrEqual(t, worker.stateValidUntil, nowUpperBound.Add(worker.restartTimeout))
			expected.stateValidUntil = worker.stateValidUntil

			assert.Equal(t, expected, worker)
		})
	}
}

func TestWorker_SetOccupied(t *testing.T) {
	timeNow := time.Now()
	stateID := 3
	matchID := 42
	nextMatchID := 123

	tests := []struct {
		name   string
		worker Worker
	}{
		{name: "free worker", worker: newTestWorkerFree(stateID)},
		{name: "occupied worker", worker: newTestWorkerOccupied(stateID, matchID)},
		{name: "restarting worker", worker: newTestWorkerRestarting(stateID, timeNow)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expected := newTestWorkerOccupied(stateID+1, nextMatchID)

			worker := test.worker

			res := worker.SetOccupied(nextMatchID)
			assert.Equal(t, worker.stateID, res)
			assert.Equal(t, expected, worker)
		})
	}
}

func TestWorker_HandlePong(t *testing.T) {
	tests := []struct {
		name        string
		initState   WorkerState
		initStateID int
		initFails   int
		pong        bool
		wantState   WorkerState
		wantStateID int
		wantFails   int
	}{
		{
			name:        "worker restarting with successful pong",
			initState:   WorkerRestarting,
			initStateID: 3,
			initFails:   2,
			pong:        true,
			wantState:   WorkerRestarting,
			wantStateID: 3,
			wantFails:   2,
		},
		{
			name:        "worker restarting with failed pong",
			initState:   WorkerRestarting,
			initStateID: 3,
			initFails:   2,
			pong:        false,
			wantState:   WorkerRestarting,
			wantStateID: 3,
			wantFails:   2,
		},
		{
			name:        "free worker with successful pong",
			initState:   WorkerFree,
			initStateID: 1,
			initFails:   2,
			pong:        true,
			wantState:   WorkerFree,
			wantStateID: 1,
			wantFails:   0,
		},
		{
			name:        "free worker with failed pong",
			initState:   WorkerFree,
			initStateID: 2,
			initFails:   2,
			pong:        false,
			wantState:   WorkerFree,
			wantStateID: 2,
			wantFails:   3,
		},
		{
			name:        "worker occupied with successful pong",
			initState:   WorkerOccupied,
			initStateID: 1,
			initFails:   2,
			pong:        true,
			wantState:   WorkerOccupied,
			wantStateID: 1,
			wantFails:   0,
		},
		{
			name:        "worker occupied with failed pong",
			initState:   WorkerOccupied,
			initStateID: 4,
			initFails:   0,
			pong:        false,
			wantState:   WorkerOccupied,
			wantStateID: 4,
			wantFails:   1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			worker := &Worker{
				State:     test.initState,
				stateID:   test.initStateID,
				failCount: test.initFails,
			}

			worker.HandlePong(test.pong)

			assert.Equal(t, test.wantState, worker.State)
			assert.Equal(t, test.wantStateID, worker.stateID)
			assert.Equal(t, test.wantFails, worker.failCount)
		})
	}
}

func TestWorker_isStateValid(t *testing.T) {
	tests := []struct {
		name            string
		stateValidUntil time.Time
		valid           bool
	}{
		{
			name:            "zero time",
			stateValidUntil: time.Time{},
			valid:           true,
		},
		{
			name:            "future time",
			stateValidUntil: time.Now().Add(time.Hour),
			valid:           true,
		},
		{
			name:            "past time",
			stateValidUntil: time.Now().Add(-time.Second),
			valid:           false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			worker := &Worker{stateValidUntil: tc.stateValidUntil}
			assert.Equal(t, tc.valid, worker.isStateValid())
		})
	}
}

func TestWorker_IsHealthy(t *testing.T) {
	tests := []struct {
		name            string
		failCount       int
		maxPingRetries  int
		stateValidUntil time.Time
		healthy         bool
	}{
		{
			name:            "fail count less than limit",
			failCount:       2,
			maxPingRetries:  3,
			stateValidUntil: time.Time{},
			healthy:         true,
		},
		{
			name:            "fail count equal limit",
			failCount:       3,
			maxPingRetries:  3,
			stateValidUntil: time.Time{},
			healthy:         true,
		},
		{
			name:            "fail count exceed limit",
			failCount:       4,
			maxPingRetries:  3,
			stateValidUntil: time.Time{},
			healthy:         false,
		},
		{
			name:            "invalid state",
			failCount:       0,
			maxPingRetries:  3,
			stateValidUntil: time.Now().Add(-time.Hour),
			healthy:         false,
		},
		{
			name:            "invalid state and fail count exceed limit",
			failCount:       4,
			maxPingRetries:  3,
			stateValidUntil: time.Now().Add(-time.Hour),
			healthy:         false,
		},
		{
			name:            "valid state and fail count under limit",
			failCount:       1,
			maxPingRetries:  3,
			stateValidUntil: time.Now().Add(time.Hour),
			healthy:         true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			worker := &Worker{
				failCount:       tc.failCount,
				maxPingRetries:  tc.maxPingRetries,
				stateValidUntil: tc.stateValidUntil,
			}
			assert.Equal(t, tc.healthy, worker.IsHealthy())
		})
	}
}

func TestWorker_LifeCycle(t *testing.T) {
	t.Run("worker fail limit exceeded", func(t *testing.T) {
		worker := NewWorker("wid", "cid", 1, 30*time.Second)

		worker.HandlePong(false)
		require.True(t, worker.IsHealthy())

		worker.HandlePong(false)
		require.False(t, worker.IsHealthy())
	})

	t.Run("worker no fail limit for restarting", func(t *testing.T) {
		worker := NewWorker("wid", "cid", 0, 30*time.Second)
		worker.SetRestarting()
		worker.HandlePong(false)
		assert.True(t, worker.IsHealthy())
	})

	t.Run("worker fail flow", func(t *testing.T) {
		worker := NewWorker("wid", "cid", 0, 1000*time.Hour)
		require.True(t, worker.IsHealthy())

		worker.HandlePong(false)
		require.False(t, worker.IsHealthy())

		worker.SetRestarting()
		require.True(t, worker.IsHealthy())
	})

	t.Run("stateID increments monotonically", func(t *testing.T) {
		worker := NewWorker("wid", "cid", 3, 30*time.Second)

		require.Equal(t, 1, worker.SetOccupied(1))
		require.Equal(t, 2, worker.SetFree())
		require.Equal(t, 3, worker.SetRestarting())
		require.Equal(t, 4, worker.SetRestarting())
		require.Equal(t, 5, worker.SetOccupied(2))

		assert.Equal(t, 5, worker.stateID)
	})
}
