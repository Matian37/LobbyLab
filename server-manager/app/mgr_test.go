//go:generate go run go.uber.org/mock/mockgen -destination=./../internal/mocks/backoff.go -package=mocks github.com/cenkalti/backoff/v6 BackOff
package app

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"server-manager/internal"
	"server-manager/internal/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func newMockWorkerManager(
	t *testing.T,
	workerCount int,
) (*mocks.MockDockerConnection, *mocks.MockBrokerConnection, *mocks.MockDatabaseConnection, *WorkerManager) {
	t.Helper()

	ctrl := gomock.NewController(t)
	docker := mocks.NewMockDockerConnection(ctrl)
	broker := mocks.NewMockBrokerConnection(ctrl)
	db := mocks.NewMockDatabaseConnection(ctrl)

	wm := NewWorkerManager(&internal.EnvConfig{Workercount: workerCount, PublicHost: "public-host"})

	wm.dockerConn = docker
	wm.brokerConn = broker
	wm.dbConn = db
	wm.saveResultChan = make(chan internal.Result, 8)
	wm.newfreeWorker = sync.NewCond(&wm.mu)

	return docker, broker, db, wm
}

func newMockWorkerManagerWithInit(
	t *testing.T,
	workers []*Worker,
) (*mocks.MockDockerConnection, *mocks.MockBrokerConnection, *mocks.MockDatabaseConnection, *WorkerManager) {
	t.Helper()

	docker, broker, db, wm := newMockWorkerManager(t, len(workers))
	wm.workers = workers
	wm.initialized = true

	return docker, broker, db, wm
}

func TestNewWorkerManager(t *testing.T) {
	config := &internal.EnvConfig{}
	wm := NewWorkerManager(config)

	require.NotNil(t, wm)
	assert.Same(t, config, wm.config)
	assert.NotNil(t, wm.dockerConn)
	assert.NotNil(t, wm.brokerConn)
	assert.NotNil(t, wm.dbConn)
}

func TestWorkerManager_Start(t *testing.T) {
	t.Run("already initialized", func(t *testing.T) {
		wm := WorkerManager{initialized: true}

		err := wm.Start(context.Background())
		require.ErrorIs(t, err, ErrMgrAlreadyInit)
	})

	t.Run("already closed", func(t *testing.T) {
		wm := WorkerManager{closed: true}

		err := wm.Start(context.Background())
		require.ErrorIs(t, err, ErrMgrClosed)
	})

	t.Run("docker open error", func(t *testing.T) {
		ctx := context.Background()

		docker, _, _, wm := newMockWorkerManager(t, 1)
		wantErr := errors.New("docker open failed")

		docker.EXPECT().Open(wm.config).Return(wantErr)

		err := wm.Start(ctx)
		require.ErrorIs(t, err, wantErr)
		assert.False(t, wm.initialized)
		assert.False(t, wm.closed)
	})

	t.Run("broker open error", func(t *testing.T) {
		ctx := context.Background()

		docker, broker, _, wm := newMockWorkerManager(t, 1)
		wantErr := errors.New("broker open failed")

		docker.EXPECT().Open(wm.config).Return(nil)
		broker.EXPECT().Open(ctx, wm.config).Return(wantErr)

		err := wm.Start(ctx)
		require.ErrorIs(t, err, wantErr)
		assert.False(t, wm.initialized)
		assert.False(t, wm.closed)
	})

	t.Run("db open error", func(t *testing.T) {
		ctx := context.Background()

		docker, broker, db, wm := newMockWorkerManager(t, 1)
		wantErr := errors.New("db open failed")

		docker.EXPECT().Open(wm.config).Return(nil)
		broker.EXPECT().Open(ctx, wm.config).Return(nil)
		db.EXPECT().Open(ctx).Return(wantErr)

		err := wm.Start(ctx)
		require.ErrorIs(t, err, wantErr)
		assert.False(t, wm.initialized)
		assert.False(t, wm.closed)
	})

	t.Run("worker spawn error", func(t *testing.T) {
		ctx := context.Background()

		docker, broker, db, wm := newMockWorkerManager(t, 2)

		wantErr := errors.New("spawn failed")

		gomock.InOrder(
			docker.EXPECT().Open(wm.config).Return(nil),
			broker.EXPECT().Open(ctx, wm.config).Return(nil),
			db.EXPECT().Open(ctx).Return(nil),
			docker.EXPECT().SpawnContainer(ctx).Return("worker-1", nil),
			docker.EXPECT().SpawnContainer(ctx).Return("", wantErr),
		)

		err := wm.Start(ctx)

		require.ErrorIs(t, err, ErrFailedToSpawnWorker)
		require.ErrorIs(t, err, wantErr)
		assert.False(t, wm.initialized)
		assert.False(t, wm.closed)

		require.Len(t, wm.workers, 1)
		assert.Equal(t, "worker-1", wm.workers[0].ID)
	})

	t.Run("success", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		docker, broker, db, wm := newMockWorkerManager(t, 2)

		gomock.InOrder(
			docker.EXPECT().Open(wm.config).Return(nil),
			broker.EXPECT().Open(ctx, wm.config).Return(nil),
			db.EXPECT().Open(ctx).Return(nil),
			docker.EXPECT().SpawnContainer(ctx).Return("worker-1", nil),
			docker.EXPECT().SpawnContainer(ctx).Return("worker-2", nil),
		)

		err := wm.Start(ctx)
		require.NoError(t, err)

		assert.True(t, wm.initialized)
		assert.False(t, wm.closed)

		require.Len(t, wm.workers, 2)
		assert.Equal(t, "worker-1", wm.workers[0].ID)
		assert.Equal(t, "worker-2", wm.workers[1].ID)
		for _, worker := range wm.workers {
			assert.Equal(t, WorkerFree, worker.State)
			assert.Equal(t, wm.workerMaxPingRetries, worker.maxPingRetries)
			assert.Equal(t, wm.workerRestartTimeout, worker.restartTimeout)
		}

		require.NotNil(t, wm.saveResultChan)
		require.NotNil(t, wm.newfreeWorker)

		wm.wg.Wait()
	})
}

func TestWorkerManager_Shutdown(t *testing.T) {
	t.Run("already closed", func(t *testing.T) {
		wm := WorkerManager{closed: true}

		err := wm.Shutdown()
		require.ErrorIs(t, err, ErrMgrAlreadyClosed)
	})

	t.Run("success", func(t *testing.T) {
		docker, broker, db, wm := newMockWorkerManagerWithInit(t, []*Worker{{ID: "worker-1"}, {ID: "worker-2"}})

		gomock.InOrder(
			docker.EXPECT().KillContainer(context.Background(), "worker-1").Return(nil),
			docker.EXPECT().KillContainer(context.Background(), "worker-2").Return(nil),
			docker.EXPECT().Close().Return(nil),
			broker.EXPECT().Close().Return(nil),
			db.EXPECT().Close().Return(nil),
		)

		err := wm.Shutdown()
		require.NoError(t, err)
		assert.True(t, wm.closed)
	})
}

func TestWorkerManager_getFreeWorker(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		workers := []*Worker{NewWorker("worker-1", 3, 30*time.Second), NewWorker("worker-2", 3, 30*time.Second)}
		workers[0].SetOccupied(1)
		wm := WorkerManager{workers: workers}

		got := wm.getFreeWorker()
		require.NotNil(t, got)
		assert.Same(t, workers[1], got)
	})

	t.Run("no free worker", func(t *testing.T) {
		workers := []*Worker{NewWorker("worker-1", 3, 30*time.Second), NewWorker("worker-2", 3, 30*time.Second)}
		workers[0].SetOccupied(1)
		workers[1].SetOccupied(2)

		wm := WorkerManager{workers: workers}
		assert.Nil(t, wm.getFreeWorker())
	})
}

func TestWorkerManager_AssignMatch(t *testing.T) {
	t.Run("not initialized", func(t *testing.T) {
		wm := WorkerManager{}
		_, err := wm.AssignMatch(context.Background(), internal.MatchConfig{})
		require.ErrorIs(t, err, ErrMgrNoInit)
	})

	t.Run("closed", func(t *testing.T) {
		wm := WorkerManager{initialized: true, closed: true}
		_, err := wm.AssignMatch(context.Background(), internal.MatchConfig{})
		require.ErrorIs(t, err, ErrMgrClosed)
	})

	t.Run("no free worker", func(t *testing.T) {
		workers := []*Worker{NewWorker("worker-1", 3, 30*time.Second)}
		workers[0].SetOccupied(1)
		_, _, _, wm := newMockWorkerManagerWithInit(t, workers)

		_, err := wm.AssignMatch(context.Background(), internal.MatchConfig{})
		require.ErrorIs(t, err, ErrNoFreeWorker)
	})

	t.Run("get game port error", func(t *testing.T) {
		ctx := context.Background()

		workers := []*Worker{NewWorker("worker-1", 3, 30*time.Second)}
		docker, _, _, wm := newMockWorkerManagerWithInit(t, workers)
		wantErr := errors.New("port failed")

		docker.EXPECT().GetGamePort(ctx, "worker-1").Return("", wantErr)

		_, err := wm.AssignMatch(ctx, internal.MatchConfig{})
		require.ErrorIs(t, err, wantErr)
		assert.Equal(t, WorkerFree, wm.workers[0].State)
	})

	t.Run("assign job error", func(t *testing.T) {
		ctx := context.Background()

		workers := []*Worker{NewWorker("worker-1", 3, 30*time.Second)}
		docker, broker, _, wm := newMockWorkerManagerWithInit(t, workers)
		wantErr := errors.New("assign failed")

		config := internal.MatchConfig{MatchID: 43}

		gomock.InOrder(
			docker.EXPECT().GetGamePort(ctx, "worker-1").Return("30001", nil),
			broker.EXPECT().AssignJob(ctx, "worker-1", config).Return(wantErr),
		)

		_, err := wm.AssignMatch(ctx, config)
		require.ErrorIs(t, err, wantErr)
		assert.Equal(t, WorkerFree, wm.workers[0].State)
	})

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()

		config := internal.MatchConfig{MatchID: 42, Config: []byte("{}")}
		workers := []*Worker{NewWorker("worker-1", 3, 30*time.Second)}
		docker, broker, _, wm := newMockWorkerManagerWithInit(t, workers)

		gomock.InOrder(
			docker.EXPECT().GetGamePort(ctx, "worker-1").Return("30001", nil),
			broker.EXPECT().AssignJob(ctx, "worker-1", config).Return(nil),
		)

		info, err := wm.AssignMatch(ctx, config)
		require.NoError(t, err)
		assert.Equal(t, internal.ServerInfo{Host: wm.config.PublicHost, Port: "30001"}, info)
		assert.Equal(t, WorkerOccupied, wm.workers[0].State)
		assert.Equal(t, config.MatchID, wm.workers[0].matchID)
	})
}

func TestWorkerManager_getWorkerByMatchID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		workers := []*Worker{
			NewWorker("worker-1", 3, 30*time.Second),
			NewWorker("worker-2", 3, 30*time.Second),
			NewWorker("worker-3", 3, 30*time.Second),
			NewWorker("worker-4", 3, 30*time.Second),
		}
		workers[0].SetOccupied(5)
		workers[1].SetRestarting()
		workers[3].SetOccupied(1)

		_, _, _, wm := newMockWorkerManagerWithInit(t, workers)

		got := wm.getWorkerByMatchID(1)
		require.NotNil(t, got)
		assert.Same(t, workers[3], got)
	})

	t.Run("no worker found", func(t *testing.T) {
		workers := []*Worker{NewWorker("worker-1", 3, 30*time.Second)}

		_, _, _, wm := newMockWorkerManagerWithInit(t, workers)

		require.Nil(t, wm.getWorkerByMatchID(3))
	})
}

func TestWorkerManager_handleResults(t *testing.T) {
	t.Run("get result error", func(t *testing.T) {
		ctx := context.Background()
		_, broker, _, wm := newMockWorkerManager(t, 1)
		wantErr := errors.New("broker failed")

		broker.EXPECT().GetResult(ctx).Return(nil, wantErr)

		err := wm.handleResults(ctx)
		require.ErrorIs(t, err, ErrFailedToGetResult)
		require.ErrorIs(t, err, wantErr)
	})

	t.Run("unmarshal error", func(t *testing.T) {
		ctx := context.Background()
		_, broker, _, wm := newMockWorkerManager(t, 1)
		msg := mocks.NewMockMessage(gomock.NewController(t))

		broker.EXPECT().GetResult(ctx).Return(msg, nil)
		msg.EXPECT().Data().Return([]byte("not json"))

		err := wm.handleResults(ctx)
		require.ErrorIs(t, err, ErrFailedToUnmarshalResult)
	})

	t.Run("ack error", func(t *testing.T) {
		ctx := context.Background()

		workers := []*Worker{NewWorker("worker-1", 3, 30*time.Second)}
		workers[0].SetOccupied(10)

		_, broker, _, wm := newMockWorkerManagerWithInit(t, workers)
		msg := mocks.NewMockMessage(gomock.NewController(t))

		payload, err := json.Marshal(internal.Result{Success: true, MatchID: 10})
		require.NoError(t, err)

		wantErr := errors.New("ack failed")

		gomock.InOrder(
			broker.EXPECT().GetResult(ctx).Return(msg, nil),
			msg.EXPECT().Data().Return(payload),
			msg.EXPECT().Ack().Return(wantErr),
		)

		err = wm.handleResults(ctx)
		require.ErrorIs(t, err, ErrFailedToAckResult)
		require.ErrorIs(t, err, wantErr)
		// check if state was not modified
		assert.Equal(t, WorkerOccupied, workers[0].State)
		assert.Equal(t, 10, workers[0].matchID)
	})

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()

		workers := []*Worker{NewWorker("worker-1", 3, 30*time.Second)}
		workers[0].SetOccupied(99)

		_, broker, _, wm := newMockWorkerManagerWithInit(t, workers)
		msg := mocks.NewMockMessage(gomock.NewController(t))

		wantResult := internal.Result{
			Success: true,
			MatchID: 99,
			Details: json.RawMessage(`{"score":12}`),
		}
		payload, err := json.Marshal(wantResult)
		require.NoError(t, err)

		gomock.InOrder(
			broker.EXPECT().GetResult(ctx).Return(msg, nil),
			msg.EXPECT().Data().Return(payload),
			msg.EXPECT().Ack().Return(nil),
		)

		ready := make(chan struct{})
		sentSignal := make(chan struct{})

		go func() {
			wm.mu.Lock()
			defer wm.mu.Unlock()

			close(ready)

			for wm.workers[0].State != WorkerFree {
				wm.newfreeWorker.Wait()
			}

			close(sentSignal)
		}()

		<-ready

		err = wm.handleResults(ctx)
		require.NoError(t, err)

		select {
		case got := <-wm.saveResultChan:
			assert.Equal(t, wantResult, got)
		case <-time.After(1 * time.Second):
			t.Fatal("expected result to be forwarded to saveResultChan")
		}

		assert.Equal(t, WorkerFree, workers[0].State)
		assert.Equal(t, 0, workers[0].matchID)

		select {
		case <-sentSignal:
		case <-time.After(1 * time.Second):
			t.Fatal("signal was not sent")
		}
	})
}

func TestWorkerManager_restartWorker(t *testing.T) {
	t.Run("container restart failed", func(t *testing.T) {
		ctx := context.Background()

		docker, _, _, wm := newMockWorkerManagerWithInit(t, []*Worker{})
		wantErr := errors.New("restart failed")

		docker.EXPECT().RestartContainer(ctx, "").Return(wantErr)

		err := wm.restartWorker(ctx, nil, 0, "")
		assert.ErrorIs(t, err, ErrWorkerRestartFailed)
		assert.ErrorIs(t, err, wantErr)
	})

	t.Run("worker changed state", func(t *testing.T) {
		ctx := context.Background()
		docker, _, _, wm := newMockWorkerManagerWithInit(t, []*Worker{&Worker{stateID: 1}})
		docker.EXPECT().RestartContainer(ctx, "").Return(nil)

		err := wm.restartWorker(ctx, wm.workers[0], 0, "")
		assert.ErrorIs(t, err, ErrWorkerStateChanged)
	})

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()

		workers := []*Worker{NewWorker("worker-1", 0, 30*time.Second)}
		stateID := workers[0].SetRestarting()

		docker, _, _, wm := newMockWorkerManagerWithInit(t, workers)

		docker.EXPECT().RestartContainer(ctx, "worker-1").Return(nil)

		ready := make(chan struct{})
		notified := make(chan struct{})

		go func() {
			wm.mu.Lock()
			defer wm.mu.Unlock()

			close(ready)

			for wm.workers[0].State != WorkerFree {
				wm.newfreeWorker.Wait()
			}

			close(notified)
		}()

		<-ready

		err := wm.restartWorker(ctx, workers[0], stateID, workers[0].ID)
		require.NoError(t, err)

		assert.Equal(t, WorkerFree, workers[0].State)
		assert.Equal(t, 0, workers[0].matchID)
		assert.Equal(t, 0, workers[0].failCount)
		assert.Equal(t, 2, workers[0].stateID)

		select {
		case <-notified:
		case <-time.After(2 * time.Second):
			t.Errorf("expected to be notified of new free worker")
		}
	})
}

func TestWorkerManager_healthCheck(t *testing.T) {
	t.Run("ping failed", func(t *testing.T) {
		ctx := context.Background()

		_, broker, _, wm := newMockWorkerManagerWithInit(t, []*Worker{})
		wantErr := errors.New("ping failed")
		broker.EXPECT().GetWorkersPong(ctx).Return(internal.Responders{}, wantErr)

		err := wm.healthCheck(ctx)
		require.ErrorIs(t, err, ErrHealthPingFailed)
		require.ErrorIs(t, err, wantErr)
	})

	t.Run("healthy workers skipped", func(t *testing.T) {
		ctx := context.Background()

		responders := internal.Responders{"worker-1": {}}
		workers := []*Worker{NewWorker("worker-1", 1, 10*time.Second), NewWorker("worker-2", 1, 10*time.Second)}
		workers[0].failCount = 1

		_, broker, _, wm := newMockWorkerManagerWithInit(t, workers)
		broker.EXPECT().GetWorkersPong(ctx).Return(responders, nil)

		err := wm.healthCheck(ctx)
		require.NoError(t, err)

		assert.Equal(t, 0, wm.workers[0].stateID)
		assert.Equal(t, WorkerFree, wm.workers[0].State)

		assert.Equal(t, 0, wm.workers[1].stateID)
		assert.Equal(t, WorkerFree, wm.workers[1].State)
	})

	t.Run("workers unhealthy", func(t *testing.T) {
		ctx := context.Background()

		responders := internal.Responders{}
		workers := []*Worker{
			NewWorker("worker-1", 1, 10*time.Second), // healthy into unhealthy
			NewWorker("worker-2", 1, -1*time.Second), // restart expired
		}
		workers[0].failCount = 1
		workers[1].failCount = 2

		wg := sync.WaitGroup{}
		wg.Add(2)
		doneFunc := func(ctx context.Context, name string) { wg.Done() }

		docker, broker, _, wm := newMockWorkerManagerWithInit(t, workers)
		wantErr := errors.New("test force error")

		broker.EXPECT().GetWorkersPong(ctx).Return(responders, nil)
		docker.EXPECT().RestartContainer(ctx, "worker-1").Do(doneFunc).Return(wantErr)
		docker.EXPECT().RestartContainer(ctx, "worker-2").Do(doneFunc).Return(wantErr)

		err := wm.healthCheck(ctx)
		require.NoError(t, err)

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(1 * time.Second):
			t.Fatal(t, "restart container should be called")
		}

		assert.Equal(t, WorkerRestarting, wm.workers[0].State)
		assert.Equal(t, 1, wm.workers[0].stateID)

		assert.Equal(t, WorkerRestarting, wm.workers[1].State)
		assert.Equal(t, 1, wm.workers[1].stateID)
	})
}

func TestWorkerManager_WaitForFreeWorker(t *testing.T) {
	t.Run("context canceled", func(t *testing.T) {
		workers := []*Worker{NewWorker("worker-1", 3, 30*time.Second)}
		workers[0].SetOccupied(1)

		_, _, _, wm := newMockWorkerManagerWithInit(t, workers)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := make(chan struct{})
		go func() {
			wm.WaitForFreeWorker(ctx)
			close(done)
		}()

		require.Eventually(t, func() bool {
			wm.mu.Lock()
			defer wm.mu.Unlock()
			return wm.waiting
		}, 1*time.Second, 100*time.Millisecond)
		cancel()

		select {
		case <-done:
		case <-time.After(1 * time.Second):
			t.Fatal("expected WaitForFreeWorker to return")
		}
	})

	t.Run("success", func(t *testing.T) {
		workers := []*Worker{NewWorker("worker-1", 3, 30*time.Second)}
		workers[0].SetOccupied(1)
		_, _, _, wm := newMockWorkerManagerWithInit(t, workers)

		done := make(chan struct{})
		go func() {
			wm.WaitForFreeWorker(context.Background())
			close(done)
		}()

		wm.mu.Lock()
		workers[0].SetFree()
		wm.mu.Unlock()

		wm.newfreeWorker.Signal()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("expected WaitForFreeWorker to return")
		}
	})
}

func TestWorkerManager_SaveLoop(t *testing.T) {
	t.Run("not initialized", func(t *testing.T) {
		_, _, _, wm := newMockWorkerManager(t, 1)

		err := wm.saveLoop(context.Background())
		require.ErrorIs(t, err, ErrMgrNoInit)
	})

	t.Run("already closed", func(t *testing.T) {
		_, _, _, wm := newMockWorkerManager(t, 1)
		wm.initialized = true
		wm.closed = true

		err := wm.saveLoop(context.Background())
		require.ErrorIs(t, err, ErrMgrClosed)
	})

	t.Run("context canceled", func(t *testing.T) {
		_, _, _, wm := newMockWorkerManagerWithInit(t, []*Worker{})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := wm.saveLoop(ctx)

		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("success", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		_, _, db, wm := newMockWorkerManagerWithInit(t, []*Worker{})
		res := internal.Result{Success: true, MatchID: 42}

		db.EXPECT().SaveMatchResults(ctx, string(res.Details), res.MatchID).DoAndReturn(
			func(context.Context, string, int) error {
				cancel()
				return ctx.Err()
			})

		done := make(chan error, 1)
		go func() {
			done <- wm.saveLoop(ctx)
		}()

		wm.saveResultChan <- res

		select {
		case err := <-done:
			require.ErrorIs(t, err, context.Canceled)
		case <-time.After(2 * time.Second):
			t.Fatal("expected SaveLoop to exit after cancellation")
		}
	})
}

func TestWorkerManager_ResultLoop(t *testing.T) {
	t.Run("not initialized", func(t *testing.T) {
		_, _, _, wm := newMockWorkerManager(t, 1)

		err := wm.resultLoop(context.Background())
		require.ErrorIs(t, err, ErrMgrNoInit)
	})

	t.Run("already closed", func(t *testing.T) {
		_, _, _, wm := newMockWorkerManager(t, 1)
		wm.initialized = true
		wm.closed = true

		err := wm.resultLoop(context.Background())
		require.ErrorIs(t, err, ErrMgrClosed)
	})

	t.Run("context canceled", func(t *testing.T) {
		_, _, _, wm := newMockWorkerManagerWithInit(t, []*Worker{})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := wm.resultLoop(ctx)

		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("success", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		_, broker, _, wm := newMockWorkerManagerWithInit(t, []*Worker{NewWorker("worker-1", 3, 30*time.Second)})

		msg := mocks.NewMockMessage(gomock.NewController(t))
		payload, err := json.Marshal(internal.Result{})
		require.NoError(t, err)

		broker.EXPECT().GetResult(ctx).Return(msg, nil)
		msg.EXPECT().Data().Return(payload)
		msg.EXPECT().Ack().Do(cancel).Return(nil)

		done := make(chan error, 1)
		go func() {
			done <- wm.resultLoop(ctx)
		}()

		select {
		case err := <-done:
			require.ErrorIs(t, err, context.Canceled)
		case <-time.After(2 * time.Second):
			t.Fatal("expected ResultLoop to exit after cancellation")
		}
	})
}

func TestWorkerManager_HealthLoop(t *testing.T) {
	t.Run("not initialized", func(t *testing.T) {
		_, _, _, wm := newMockWorkerManager(t, 1)

		err := wm.healthLoop(context.Background())
		require.ErrorIs(t, err, ErrMgrNoInit)
	})

	t.Run("already closed", func(t *testing.T) {
		_, _, _, wm := newMockWorkerManager(t, 1)
		wm.initialized = true
		wm.closed = true

		err := wm.healthLoop(context.Background())
		require.ErrorIs(t, err, ErrMgrClosed)
	})

	t.Run("context canceled", func(t *testing.T) {
		_, _, _, wm := newMockWorkerManagerWithInit(t, []*Worker{})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := wm.healthLoop(ctx)

		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("success", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		_, broker, _, wm := newMockWorkerManagerWithInit(t, []*Worker{NewWorker("worker-1", 3, 30*time.Second)})
		wm.healthCheckTick = 20 * time.Millisecond

		broker.EXPECT().GetWorkersPong(ctx).DoAndReturn(func(ctx context.Context) (internal.Responders, error) {
			cancel()
			return internal.Responders{}, ctx.Err()
		})

		done := make(chan error, 1)
		go func() {
			done <- wm.healthLoop(ctx)
		}()

		select {
		case err := <-done:
			require.ErrorIs(t, err, context.Canceled)
		case <-time.After(2 * time.Second):
			t.Fatal("expected HealthLoop to exit after cancellation")
		}
	})
}

func TestWorkerManager_LifeCycle(t *testing.T) {
	t.Run("worker assign match", func(t *testing.T) {
		ctx := context.Background()

		workers := []*Worker{NewWorker("worker-1", 1, 1*time.Hour), NewWorker("worker-2", 1, 1*time.Hour)}
		workers[0].SetOccupied(2)

		config := internal.MatchConfig{MatchID: 41}

		docker, broker, _, wm := newMockWorkerManagerWithInit(t, workers)
		broker.EXPECT().GetWorkersPong(ctx).Return(internal.Responders{"worker-2": {}}, nil)
		docker.EXPECT().GetGamePort(ctx, "worker-2").Return("8080/udp", nil)
		broker.EXPECT().AssignJob(ctx, "worker-2", config).Return(nil)

		type Result struct {
			serverInfo internal.ServerInfo
			err        error
		}
		done := make(chan Result)

		go func() {
			wm.WaitForFreeWorker(ctx)
			serverInfo, err := wm.AssignMatch(ctx, config)
			done <- Result{serverInfo: serverInfo, err: err}
		}()
		require.NoError(t, wm.healthCheck(ctx))

		select {
		case res := <-done:
			require.NoError(t, res.err)
			require.NotNil(t, res.serverInfo.Port, "8080/udp")
			require.NotNil(t, res.serverInfo.Host, wm.config.PublicHost)
		case <-time.After(2 * time.Second):
			t.Fatal("lifecycle did not finish")
		}
	})

	t.Run("worker restart", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		workers := []*Worker{
			NewWorker("worker-1", 0, 1*time.Hour),
			NewWorker("worker-2", 0, 1*time.Hour),
			NewWorker("worker-3", 0, -1*time.Hour),
		}
		workers[1].SetOccupied(1)

		wg := sync.WaitGroup{}
		blockRestart := make(chan struct{})

		docker, broker, _, wm := newMockWorkerManagerWithInit(t, workers)

		block := func(ctx context.Context, id string) {
			<-blockRestart
			wg.Done()
		}
		broker.EXPECT().GetWorkersPong(ctx).Return(internal.Responders{"worker-1": {}}, nil)
		docker.EXPECT().RestartContainer(ctx, "worker-2").Do(block).Return(nil)
		docker.EXPECT().RestartContainer(ctx, "worker-3").Do(block).Return(errors.New(""))

		wg.Add(2)
		require.NoError(t, wm.healthCheck(ctx))

		require.Equal(t, WorkerRestarting, wm.workers[1].State)
		require.Equal(t, WorkerRestarting, wm.workers[2].State)

		close(blockRestart)
		wg.Wait()
		wm.wg.Wait()

		require.Equal(t, WorkerFree, wm.workers[1].State)
		require.Equal(t, WorkerRestarting, wm.workers[2].State)
	})

	t.Run("worker match finished", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		workers := []*Worker{NewWorker("worker-1", 0, 1*time.Hour)}
		workers[0].SetOccupied(1)

		_, broker, db, wm := newMockWorkerManagerWithInit(t, workers)
		msg := mocks.NewMockMessage(gomock.NewController(t))
		res := internal.Result{MatchID: 1, Success: false, Details: json.RawMessage("{}")}
		payload, err := json.Marshal(res)
		require.NoError(t, err)

		broker.EXPECT().GetResult(ctx).Return(msg, nil)
		msg.EXPECT().Data().Return(payload)
		msg.EXPECT().Ack().Return(nil)
		db.EXPECT().SaveMatchResults(ctx, string(res.Details), res.MatchID).
			DoAndReturn(func(ctx context.Context, details string, matchID int) error {
				cancel()
				return nil
			})

		require.NoError(t, wm.handleResults(ctx))
		go func() { _ = wm.saveLoop(ctx) }()
		<-ctx.Done()

		require.Equal(t, WorkerFree, wm.workers[0].State)
	})
}
