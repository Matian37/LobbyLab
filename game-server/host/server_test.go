package main

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAttachParams(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		expected := []string{"a", "--match-config", "b", "--match-result", "c"}
		inputCommand := []string{"a"}

		result := attachParams(inputCommand, "b", "c")
		assert.Equal(t, expected, result)
		assert.Equal(t, &inputCommand[0], &result[0])
	})
}

func TestCreateTempFile(t *testing.T) {
	t.Run("incorrect subname", func(t *testing.T) {
		_, err := createTempFile("/")
		assert.ErrorIs(t, err, ErrFailedToCreateTempFile)
	})

	t.Run("succes", func(t *testing.T) {
		file, err := createTempFile("")
		assert.NoError(t, err)
		_, err = file.Stat()
		assert.NoError(t, err)
	})
}

func TestCleanup(t *testing.T) {
	t.Run("nil files", func(t *testing.T) {
		server := GameServer{}
		server.cleanup()
	})

	t.Run("success", func(t *testing.T) {
		server := GameServer{}

		server.configFile, _ = os.CreateTemp("", "*")
		server.resultFile, _ = os.CreateTemp("", "*")

		configPath := server.configFile.Name()
		resultPath := server.resultFile.Name()

		server.cleanup()

		assert.Nil(t, server.configFile)
		assert.Nil(t, server.resultFile)

		_, err := os.Stat(configPath)
		assert.ErrorIs(t, err, os.ErrNotExist)
		_, err = os.Stat(resultPath)
		assert.Error(t, err, os.ErrNotExist)
	})
}

func TestStartWait(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := GameServer{}
		assert.NoError(t, server.Start("", []string{"echo"}))

		assert.NoError(t, server.startWait())
		assert.NotNil(t, server.waitChannel)

		assert.NoError(t, server.Stop(context.Background()))
	})
}

func TestStart(t *testing.T) {
	t.Run("no command", func(t *testing.T) {
		server := GameServer{}
		err := server.Start("", []string{})
		assert.ErrorIs(t, err, ErrCommandEmpty)
		assert.False(t, server.started)
	})

	t.Run("invalid command", func(t *testing.T) {
		server := GameServer{}
		err := server.Start("", []string{""})
		assert.ErrorIs(t, err, ErrGameServerStartFailed)
		assert.False(t, server.started)
		assert.Nil(t, server.cmd.Process)
	})

	t.Run("success", func(t *testing.T) {
		server := GameServer{}

		err := server.Start("", []string{"echo"})
		assert.NoError(t, err)

		assert.NotNil(t, server.cmd)
		assert.NotNil(t, server.configFile)
		assert.NotNil(t, server.resultFile)
		assert.NotZero(t, server.pgid)
		assert.True(t, server.started)
		assert.False(t, server.closed)

		_, err = server.configFile.Stat()
		assert.NoError(t, err)
		_, err = server.configFile.Stat()
		assert.NoError(t, err)

		pgid, err := syscall.Getpgid(server.cmd.Process.Pid)
		assert.NoError(t, err)
		assert.Equal(t, pgid, server.pgid)

	})

	t.Run("already started", func(t *testing.T) {
		server := GameServer{}
		err := server.Start("", []string{"echo"})
		assert.NoError(t, err)

		err = server.Start("", []string{"echo"})
		assert.ErrorIs(t, err, ErrGameServerAlreadyStarted)
	})
}

func TestWait(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := GameServer{}
		server.Start("", []string{"sh", "-c", "exit 1"})

		err := server.Wait(context.Background())

		_, ok := err.(*exec.ExitError)
		assert.True(t, ok)
	})

	t.Run("context cancel", func(t *testing.T) {
		server := GameServer{}
		server.Start("", []string{"sleep", "inf"})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := server.Wait(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestStop(t *testing.T) {
	assertProcessEnded := func(server *GameServer, signal syscall.Signal) {
		assert.ErrorIs(t, server.cmd.Process.Signal(syscall.Signal(0)), os.ErrProcessDone)

		assert.False(t, server.cmd.ProcessState.Exited()) // check if sent signal to kill
		assert.Equal(t, signal, server.cmd.ProcessState.Sys().(syscall.WaitStatus).Signal())
	}

	t.Run("not started", func(t *testing.T) {
		server := GameServer{}
		err := server.Stop(context.Background())
		assert.ErrorIs(t, err, ErrGameServerNotStarted)
	})

	t.Run("success", func(t *testing.T) {
		server := GameServer{}
		err := server.Start("", []string{"sleep", "inf"})
		assert.NoError(t, err)

		ctx := context.Background()
		err = server.Stop(ctx)
		assert.NoError(t, err)
		assert.Nil(t, ctx.Err())

		assert.True(t, server.closed)
		assertProcessEnded(&server, syscall.SIGTERM)
	})

	t.Run("already closed", func(t *testing.T) {
		server := GameServer{}
		err := server.Start("", []string{"echo"})
		assert.NoError(t, err)

		err = server.Stop(context.Background())
		assert.NoError(t, err)

		err = server.Stop(context.Background())
		assert.ErrorIs(t, err, ErrGameServerAlreadyClosed)
	})

	t.Run("context cancel", func(t *testing.T) {
		server := GameServer{}
		err := server.Start("", []string{"echo"})
		assert.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err = server.Stop(ctx)
		assert.ErrorIs(t, err, context.Canceled)
		assertProcessEnded(&server, syscall.SIGTERM)
	})

	t.Run("process ignores SIGTERM", func(t *testing.T) {
		server := GameServer{}

		cmd := []string{"sh", "-c", "trap '' TERM; kill -USR1 $PPID; sleep inf"}
		err := server.Start("", cmd)
		assert.NoError(t, err)

		// wait for script to send signal
		// it must be set to ignore SIGTERM signals before it can be stopped
		ready := make(chan os.Signal, 1)
		signal.Notify(ready, syscall.SIGUSR1)
		<-ready

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err = server.Stop(ctx)
		assert.ErrorIs(t, err, context.Canceled)
		assertProcessEnded(&server, syscall.SIGKILL)
	})

	t.Run("process died but its child is alive", func(t *testing.T) {
		server := GameServer{}

		cmd := []string{"sh", "-c", "sleep inf & exit"}
		err := server.Start("", cmd)
		assert.NoError(t, err)

		pgid := server.cmd.Process.Pid
		server.cmd.Wait()

		// assert child is alive
		assert.NoError(t, syscall.Kill(-pgid, 0))

		err = server.Stop(context.Background())
		assert.NoError(t, err)

		// poll until the process group is gone, or 1s elapses
		err = nil
		for range 50 {
			err = syscall.Kill(-pgid, 0)
			if err != nil {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		assert.ErrorIs(t, err, syscall.ESRCH)
	})
}

func TestGetResult(t *testing.T) {
	t.Run("not started", func(t *testing.T) {
		server := GameServer{}
		_, err := server.GetResult(context.Background())
		assert.ErrorIs(t, err, ErrGameServerNotStarted)
	})

	t.Run("closed", func(t *testing.T) {
		server := GameServer{}
		server.Start("", []string{"echo"})
		server.Stop(context.Background())

		_, err := server.GetResult(context.Background())
		assert.ErrorIs(t, err, ErrGameServerAlreadyClosed)
	})

	tests := []struct {
		name    string
		payload []byte
	}{
		{
			name:    "success",
			payload: []byte{1, 2, 3},
		},
		{
			name:    "empty payload",
			payload: []byte{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := GameServer{}
			server.Start("", []string{"echo"})
			defer server.Stop(context.Background())

			n, err := server.resultFile.Write(test.payload)
			assert.NoError(t, err)
			assert.Equal(t, len(test.payload), n)

			res, err := server.GetResult(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, test.payload, res)
		})

	}

	t.Run("context canceled", func(t *testing.T) {
		server := GameServer{}
		server.Start("config", []string{"sleep", "inf"})
		defer server.Stop(context.Background())

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := server.GetResult(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})
}
