//go:build e2e

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Matian37/multiplayer-asset/game-server/app"
	"github.com/Matian37/multiplayer-asset/game-server/internal"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// gameScript identifies a game server script under testdata.
type gameScript string

const (
	scriptSuccess                  gameScript = "success.sh"
	scriptFail                     gameScript = "fail.sh"
	scriptInvalidResult            gameScript = "invalid_result.sh"
	scriptUnresponsiveSIGTERM      gameScript = "unresponsive_sigterm.sh"
	scriptUnresponsiveSIGKILL      gameScript = "unresponsive_sigkill.sh"
	scriptUnresponsiveWithChildren gameScript = "unresponsive_with_children.sh"
	scriptMultiMode                gameScript = "multi_mode.sh"
)

func verifyNoGoroutineLeaks(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { goleak.VerifyNone(t, goleak.IgnoreCurrent()) })
}

const (
	healthSubject  = "workers.health"
	assignSubject  = "workers.assign"
	resultsSubject = "workers.results"
	testWorkerID   = "e2e-worker"
)

func testDataPath(t *testing.T, name gameScript) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok, "failed to determine test file path")
	return filepath.Join(filepath.Dir(filename), "testdata", string(name))
}

func setArgs(t *testing.T, args []string) {
	t.Helper()
	original := os.Args
	os.Args = args
	t.Cleanup(func() { os.Args = original })
}

func setupNATS(t *testing.T) (nc *nats.Conn, resultSub *nats.Subscription) {
	t.Helper()

	opts := &server.Options{
		Port:      -1,
		Host:      "127.0.0.1",
		JetStream: true,
		StoreDir:  t.TempDir(),
		NoSigs:    true,
	}
	s, err := server.NewServer(opts)
	require.NoError(t, err)

	s.Start()
	require.True(t, s.ReadyForConnections(5*time.Second))

	t.Cleanup(func() {
		s.Shutdown()
		s.WaitForShutdown()
	})

	addr := fmt.Sprintf("nats://127.0.0.1:%d", s.Addr().(*net.TCPAddr).Port)
	t.Setenv("NATS_URI", addr)

	nc, resultSub = setupStream(t, addr)
	return
}

func setupStream(t *testing.T, addr string) (*nats.Conn, *nats.Subscription) {
	t.Helper()

	nc, err := nats.Connect(addr)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	js, err := jetstream.New(nc)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "RESULT",
		Subjects: []string{resultsSubject},
	})
	require.NoError(t, err)

	sub, err := nc.SubscribeSync(resultsSubject)
	require.NoError(t, err)
	require.NoError(t, sub.SetPendingLimits(100, -1))

	return nc, sub
}

func startApp(t *testing.T, script gameScript) (context.CancelFunc, chan error) {
	t.Helper()

	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("WORKER_ID", testWorkerID)
	setArgs(t, []string{"game-server", "sh " + testDataPath(t, script)})

	ctx, cancel := context.WithCancel(context.Background())
	res := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		res <- app.Run(ctx)
	}()

	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("application did not shut down")
		}

		select {
		case err := <-res:
			assert.NoError(t, err, "application should shut down cleanly")
		default:
		}
	})

	return cancel, res
}

func waitForWorkerReady(t *testing.T, nc *nats.Conn) {
	t.Helper()
	require.Eventually(t, func() bool {
		msg, err := nc.Request(healthSubject, nil, 500*time.Millisecond)
		return err == nil && string(msg.Data) == testWorkerID
	}, 10*time.Second, 100*time.Millisecond, "worker should start and anwsers pings")
}

func startHealthPinger(t *testing.T, nc *nats.Conn) chan error {
	t.Helper()
	done := make(chan struct{})

	var wg sync.WaitGroup
	errCh := make(chan error, 1)

	wg.Go(func() { healthPinger(t, done, nc, errCh) })

	t.Cleanup(func() {
		close(done)
		wg.Wait()
	})

	return errCh
}

func healthPinger(t *testing.T, done chan struct{}, nc *nats.Conn, errCh chan error) {
	t.Helper()

	ticker := time.NewTicker(1250 * time.Millisecond)
	defer ticker.Stop()

	alive := false
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
		}

		msg, err := nc.Request(healthSubject, nil, 1250*time.Millisecond)

		// if worker responded
		if err == nil && string(msg.Data) == testWorkerID {
			alive = true
		} else if alive {
			errCh <- errors.New("worker did not answer periodic health ping")
			return
		}
	}
}

func requirePingerOK(t *testing.T, errChan chan error) {
	t.Helper()

	select {
	case err := <-errChan:
		t.Fatalf("health pinger should not error: %v", err)
	default:
	}
}

func assignMatch(t *testing.T, nc *nats.Conn, matchID int, config json.RawMessage) {
	t.Helper()

	payload, err := json.Marshal(internal.MatchConfig{MatchID: matchID, Config: config})
	require.NoError(t, err)

	_, err = nc.Request(assignSubject+"."+testWorkerID, payload, 10*time.Second)
	require.NoError(t, err, "worker should acknowledge match assignment")
}

func expectResult(t *testing.T, sub *nats.Subscription) internal.Result {
	t.Helper()
	msg, err := sub.NextMsg(15 * time.Second)
	require.NoError(t, err, "timed out waiting for match result")

	var r internal.Result
	require.NoError(t, json.Unmarshal(msg.Data, &r))
	return r
}

func requireSuccessResult(t *testing.T, r internal.Result, matchID int, details string) {
	t.Helper()
	assert.True(t, r.Success)
	assert.Equal(t, matchID, r.MatchID)
	assert.JSONEq(t, details, string(r.Details))
}

func requireCancelResult(t *testing.T, r internal.Result, matchID int) {
	t.Helper()
	assert.False(t, r.Success)
	assert.Equal(t, matchID, r.MatchID)
	assert.JSONEq(t, "{}", string(r.Details))
}

func readPIDsFrom(t *testing.T, pidFile string) []int {
	t.Helper()

	data, err := os.ReadFile(pidFile)
	if err != nil {
		return nil
	}

	var pids []int
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err != nil {
			t.Errorf("invalid pid: %v: %v", err, line)
			continue
		}
		pids = append(pids, pid)
	}
	return pids
}

func waitForScriptPIDs(t *testing.T, pidFile string, count int) []int {
	t.Helper()
	var pids []int
	require.Eventually(t, func() bool {
		pids = readPIDsFrom(t, pidFile)
		return len(pids) == count
	}, 10*time.Second, 50*time.Millisecond, "game server should record all of its PIDs")
	return pids
}

func requireProcessesGone(t *testing.T, pids []int) {
	t.Helper()
	for _, pid := range pids {
		require.Eventually(t, func() bool {
			return errors.Is(syscall.Kill(pid, 0), syscall.ESRCH)
		}, 10*time.Second, 100*time.Millisecond, "pid %d should be gone", pid)
	}
}

func setupPidFile(t *testing.T) string {
	t.Helper()

	pidFile := filepath.Join(t.TempDir(), "state")

	t.Setenv("SCRIPT_PID_FILE", pidFile)
	t.Cleanup(func() {
		pids := readPIDsFrom(t, pidFile)

		for _, pid := range pids {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})

	return pidFile
}
