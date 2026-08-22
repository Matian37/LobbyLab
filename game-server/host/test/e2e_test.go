//go:build e2e

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	app "server/app"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_ConfigParsingFailure(t *testing.T) {
	verifyNoGoroutineLeaks(t)

	cases := []struct {
		name     string
		env      map[string]string
		args     []string
		contains string
	}{
		{
			name: "MissingRequiredEnv",
			env: map[string]string{
				"LOG_LEVEL": "debug",
				"WORKER_ID": "0",
			},
			args:     []string{"game-server", "sh /bin/true"},
			contains: "NATS_URI",
		},
		{
			name: "EmptyRequiredEnv",
			env: map[string]string{
				"NATS_URI":  "",
				"LOG_LEVEL": "debug",
				"WORKER_ID": "0",
			},
			args:     []string{"game-server", "sh /bin/true"},
			contains: "NATS_URI",
		},
		{
			name: "InvalidLogLevel",
			env: map[string]string{
				"NATS_URI":  "nats://127.0.0.1:4222",
				"LOG_LEVEL": "not-a-level",
				"WORKER_ID": "0",
			},
			args:     []string{"game-server", "sh /bin/true"},
			contains: "invalid log level",
		},
		{
			name: "MissingCommandArgument",
			env: map[string]string{
				"NATS_URI":  "nats://127.0.0.1:4222",
				"LOG_LEVEL": "debug",
				"WORKER_ID": "0",
			},
			args:     []string{"game-server"},
			contains: "expected 1 argument",
		},
		{
			name: "TooManyArguments",
			env: map[string]string{
				"NATS_URI":  "nats://127.0.0.1:4222",
				"LOG_LEVEL": "debug",
				"WORKER_ID": "0",
			},
			args:     []string{"game-server", "sh /bin/true", "extra"},
			contains: "expected 1 argument",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for key, value := range tc.env {
				t.Setenv(key, value)
			}
			setArgs(t, tc.args)

			err := app.Run(context.Background())
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to load config")
			assert.Contains(t, err.Error(), tc.contains)
		})
	}
}

func TestE2E_SuccessfulMatch(t *testing.T) {
	verifyNoGoroutineLeaks(t)

	nc, results := setupNATS(t)
	_, _ = startApp(t, scriptSuccess)
	waitForWorkerReady(t, nc)
	pingerErrChan := startHealthPinger(t, nc)

	const matchID = 1
	config := `{"game":"pong","player":"player1"}`
	assignMatch(t, nc, matchID, json.RawMessage(config))

	expectedDetails := fmt.Sprintf(`{"winner":"player1","score":10,"config":%s}`, config)
	result := expectResult(t, results)
	requireSuccessResult(t, result, matchID, expectedDetails)
	requirePingerOK(t, pingerErrChan)
}

func TestE2E_CanceledMatch(t *testing.T) {
	verifyNoGoroutineLeaks(t)

	nc, results := setupNATS(t)
	_, _ = startApp(t, scriptFail)
	waitForWorkerReady(t, nc)
	pingerErrChan := startHealthPinger(t, nc)

	const matchID = 1
	assignMatch(t, nc, matchID, json.RawMessage(`{"game":"pong"}`))

	requireCancelResult(t, expectResult(t, results), matchID)
	requirePingerOK(t, pingerErrChan)
}

func TestE2E_InvalidResultCancelsMatch(t *testing.T) {
	verifyNoGoroutineLeaks(t)

	nc, results := setupNATS(t)
	_, _ = startApp(t, scriptInvalidResult)
	waitForWorkerReady(t, nc)
	pingerErrChan := startHealthPinger(t, nc)

	const matchID = 1
	assignMatch(t, nc, matchID, json.RawMessage(`{"game":"pong"}`))

	requireCancelResult(t, expectResult(t, results), matchID)
	requirePingerOK(t, pingerErrChan)
}

func TestE2E_InvalidAssignmentCancelsMatch(t *testing.T) {
	verifyNoGoroutineLeaks(t)

	nc, _ := setupNATS(t)
	_, _ = startApp(t, scriptSuccess)
	waitForWorkerReady(t, nc)
	pingerErrChan := startHealthPinger(t, nc)

	_, err := nc.Request(assignSubject+"."+testWorkerID, []byte(`not-json`), 2*time.Second)
	assert.Error(t, err, "an invalid assignment should not be acknowledged")

	requirePingerOK(t, pingerErrChan)
}

func TestE2E_UnresponsiveServerShutsDown(t *testing.T) {
	tests := []struct {
		name     string
		script   gameScript
		pidCount int
	}{
		{
			name:     "sigterm single process",
			script:   scriptUnresponsiveSIGTERM,
			pidCount: 1,
		},
		{
			name:     "sigkill single process",
			script:   scriptUnresponsiveSIGKILL,
			pidCount: 1,
		},
		{
			name:     "sigkill multiple processes",
			script:   scriptUnresponsiveWithChildren,
			pidCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifyNoGoroutineLeaks(t)

			nc, results := setupNATS(t)
			pidFile := setupPidFile(t)
			cancel, _ := startApp(t, tt.script)
			waitForWorkerReady(t, nc)
			pingerErrChan := startHealthPinger(t, nc)

			const matchID = 1
			assignMatch(t, nc, matchID, json.RawMessage(`{"game":"pong"}`))
			pids := waitForScriptPIDs(t, pidFile, tt.pidCount)

			cancel()
			requireCancelResult(t, expectResult(t, results), matchID)
			requireProcessesGone(t, pids)

			requirePingerOK(t, pingerErrChan)
		})
	}
}

func TestE2E_GracefulShutdownWithoutMatch(t *testing.T) {
	verifyNoGoroutineLeaks(t)

	nc, _ := setupNATS(t)
	cancel, appResult := startApp(t, scriptSuccess)
	waitForWorkerReady(t, nc)
	_ = startHealthPinger(t, nc)

	cancel()
	select {
	case err := <-appResult:
		require.NoError(t, err, "application should shut down cleanly")
	case <-time.After(10 * time.Second):
		t.Fatal("application did not shut down")
	}
}

func TestE2E_GracefulShutdownWithHangingMatch(t *testing.T) {
	verifyNoGoroutineLeaks(t)

	nc, results := setupNATS(t)
	pidFile := setupPidFile(t)

	cancel, appResult := startApp(t, scriptUnresponsiveSIGKILL)
	waitForWorkerReady(t, nc)
	pingerErrChan := startHealthPinger(t, nc)

	const matchID = 1
	assignMatch(t, nc, matchID, json.RawMessage(`{"game":"pong"}`))
	pids := waitForScriptPIDs(t, pidFile, 1)

	cancel()
	requireCancelResult(t, expectResult(t, results), matchID)
	requireProcessesGone(t, pids)

	select {
	case err := <-appResult:
		require.NoError(t, err, "application should shut down cleanly")
	case <-time.After(10 * time.Second):
		t.Fatal("application did not shut down")
	}

	requirePingerOK(t, pingerErrChan)
}

func TestE2E_MultipleMatchesMixedOutcomes(t *testing.T) {
	verifyNoGoroutineLeaks(t)

	nc, results := setupNATS(t)
	pidFile := setupPidFile(t)

	cancel, _ := startApp(t, scriptMultiMode)
	waitForWorkerReady(t, nc)
	pingerErrChan := startHealthPinger(t, nc)

	matchID := 1
	successConfig := fmt.Sprintf(`{"mode":"success","player":"player-%d"}`, time.Now().UnixNano())
	assignMatch(t, nc, matchID, json.RawMessage(successConfig))

	expectedDetails := fmt.Sprintf(`{"winner":"player1","score":10,"config":%s}`, successConfig)
	successResult := expectResult(t, results)
	requireSuccessResult(t, successResult, matchID, expectedDetails)

	matchID++
	assignMatch(t, nc, matchID, json.RawMessage(`{"mode":"fail"}`))
	requireCancelResult(t, expectResult(t, results), matchID)

	matchID++
	assignMatch(t, nc, matchID, json.RawMessage(`{"mode":"invalid"}`))
	requireCancelResult(t, expectResult(t, results), matchID)

	matchID++
	assignMatch(t, nc, matchID, json.RawMessage(`{"mode":"hang"}`))
	pids := waitForScriptPIDs(t, pidFile, 1)

	cancel()
	requireCancelResult(t, expectResult(t, results), matchID)
	requireProcessesGone(t, pids)

	requirePingerOK(t, pingerErrChan)
}
