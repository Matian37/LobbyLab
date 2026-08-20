//go:build race

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"server-manager/internal"
	"server-manager/internal/mocks"
	"strconv"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"
)

func randomResponders(count int) map[string]struct{} {
	responders := make(map[string]struct{}, count)
	for i := 0; i < count; i++ {
		responders[strconv.Itoa(i)] = struct{}{}
	}
	return responders
}

func genPayload(matchID int) []byte {
	res := internal.Result{MatchID: matchID, Details: json.RawMessage("{}")}
	payload, _ := json.Marshal(res)
	return payload
}

func TestRace_ServerManager_LifeCycle(t *testing.T) {
	tests := []struct {
		name             string
		workersCount     int
		failRate         int
		goroutinePerFunc int
	}{
		{
			name:             "high fail rate",
			workersCount:     5,
			failRate:         2,
			goroutinePerFunc: 10000,
		},
		{
			name:             "high success rate",
			workersCount:     5,
			failRate:         100,
			goroutinePerFunc: 10000,
		},
		{
			name:             "single worker with medium fail rate",
			workersCount:     5,
			failRate:         10,
			goroutinePerFunc: 10000,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defer goleak.VerifyNone(t)

			workers := []*Worker{}
			for i := range test.workersCount {
				maxPingRetries := rand.Intn(10)
				workerRestartTimeout := time.Duration(-20+rand.Intn(41)) * time.Second
				worker := NewWorker(
					fmt.Sprintf("wid-%v", i),
					fmt.Sprintf("cid-%v", i),
					maxPingRetries,
					workerRestartTimeout,
				)
				workers = append(workers, worker)
			}

			docker, broker, db, wm := newMockWorkerManagerWithInit(t, workers)
			ctrl := gomock.NewController(t)
			msg := mocks.NewMockMessage(ctrl)

			wm.saveResultChan = make(chan internal.Result, test.goroutinePerFunc)
			matchChan := make(chan int, test.goroutinePerFunc)

			docker.EXPECT().RestartContainer(gomock.Any(), gomock.Any()).
				AnyTimes().
				DoAndReturn(func(ctx any, id any) error {
					if rand.Intn(test.failRate) == 0 {
						return errors.New("")
					}
					return nil
				})
			docker.EXPECT().GetGamePort(gomock.Any(), gomock.Any()).AnyTimes().Return("", nil)
			broker.EXPECT().AssignJob(gomock.Any(), gomock.Any(), gomock.Any()).
				AnyTimes().
				DoAndReturn(func(ctx any, workerID string, config internal.MatchConfig) error {
					if rand.Intn(test.failRate) == 0 {
						return errors.New("")
					}
					matchChan <- config.MatchID
					return nil
				})
			broker.EXPECT().GetWorkersPong(gomock.Any(), gomock.Any()).
				AnyTimes().
				DoAndReturn(func(ctx any, pongTimeout any) (map[string]struct{}, error) {
					if rand.Intn(test.failRate) == 0 {
						return nil, errors.New("")
					}
					return randomResponders(test.workersCount), nil
				})
			broker.EXPECT().GetResult(gomock.Any()).
				AnyTimes().
				DoAndReturn(func(ctx any) (internal.Message, error) {
					if rand.Intn(test.failRate) == 0 {
						return nil, errors.New("")
					}
					return msg, nil
				})
			msg.EXPECT().Ack().AnyTimes().Return(nil)
			msg.EXPECT().Data().AnyTimes().DoAndReturn(func() []byte {
				if rand.Intn(test.failRate) == 0 {
					return genPayload(test.goroutinePerFunc)
				}
				select {
				case matchID := <-matchChan:
					return genPayload(matchID)
				default:
					return genPayload(test.goroutinePerFunc)
				}
			})
			db.EXPECT().RemoveMatchStatus(gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
			db.EXPECT().SaveMatchResults(gomock.Any(), gomock.Any()).AnyTimes().Return(nil)

			wg := sync.WaitGroup{}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			saveLoopDone := make(chan struct{})
			go func() {
				defer close(saveLoopDone)
				wm.saveLoop(ctx)
			}()

			for idx := range test.goroutinePerFunc {
				switch rand.Intn(3) {
				case 0:
					wg.Go(func() { wm.AssignMatch(ctx, internal.MatchConfig{MatchID: idx}) })
				case 1:
					wg.Go(func() { wm.healthCheck(ctx) })
				case 2:
					wg.Go(func() { wm.handleResults(ctx) })
				}
			}

			wg.Wait()
			cancel()
			<-saveLoopDone
		})
	}
}

// restart is tested here, because data races can easily slip up in lifecycle tests for it
func TestRace_ServerManager_Restart(t *testing.T) {
	workers := []*Worker{NewWorker("0", "0", 0, -1*time.Second)}
	docker, _, _, wm := newMockWorkerManagerWithInit(t, workers)

	docker.EXPECT().RestartContainer(gomock.Any(), gomock.Any()).AnyTimes().Return(nil)

	wg := sync.WaitGroup{}

	for _ = range 1000 {
		wg.Go(func() {
			wm.mu.Lock()
			worker := *workers[0]
			wm.mu.Unlock()
			wm.restartWorker(context.Background(), workers[0], worker.stateID)
		})
	}

	wg.Wait()
}
