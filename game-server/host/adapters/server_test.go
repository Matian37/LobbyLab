package adapters

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		require.NoError(t, err)
		_, err = file.Stat()
		assert.NoError(t, err)
	})
}

func TestGameServer_Cleanup(t *testing.T) {
	t.Run("nil files", func(t *testing.T) {
		server := GameServer{}
		server.cleanup()
	})

	t.Run("success", func(t *testing.T) {
		server := GameServer{
			started:     true,
			cmd:         exec.Command("echo"),
			pgid:        1,
			waitChannel: make(chan error),
		}
		configFile, err := os.CreateTemp("", "*")
		require.NoError(t, err)
		server.configFile = configFile
		resultFile, err := os.CreateTemp("", "*")
		require.NoError(t, err)
		server.resultFile = resultFile

		configPath := server.configFile.Name()
		resultPath := server.resultFile.Name()

		server.cleanup()
		require.Equal(t, GameServer{}, server)

		_, err = os.Stat(configPath)
		assert.ErrorIs(t, err, os.ErrNotExist)
		_, err = os.Stat(resultPath)
		assert.Error(t, err, os.ErrNotExist)
	})
}

func TestGameServer_StartWait(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := GameServer{}
		require.NoError(t, server.Start("", []string{"echo"}))

		server.startWait()
		require.NotNil(t, server.waitChannel)

		assert.NoError(t, server.Stop(context.Background()))
	})
}

func TestGameServer_Start(t *testing.T) {
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

		require.Equal(t, GameServer{}, server)
	})

	t.Run("success", func(t *testing.T) {
		server := GameServer{}

		err := server.Start("", []string{"echo"})
		require.NoError(t, err)

		require.NotNil(t, server.cmd)
		require.NotNil(t, server.cmd.Process)
		require.NotNil(t, server.configFile)
		require.NotNil(t, server.resultFile)
		assert.NotZero(t, server.pgid)
		assert.True(t, server.started)

		_, err = server.configFile.Stat()
		assert.NoError(t, err)
		_, err = server.configFile.Stat()
		assert.NoError(t, err)

		pgid, err := syscall.Getpgid(server.cmd.Process.Pid)
		require.NoError(t, err)
		assert.Equal(t, pgid, server.pgid)

	})

	t.Run("already started", func(t *testing.T) {
		server := GameServer{}
		err := server.Start("", []string{"echo"})
		require.NoError(t, err)

		err = server.Start("", []string{"echo"})
		assert.ErrorIs(t, err, ErrGameServerAlreadyStarted)
	})
}

func TestGameServer_Wait(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := GameServer{}
		require.NoError(t, server.Start("", []string{"sh", "-c", "exit 1"}))

		err := server.wait(context.Background())

		_, ok := err.(*exec.ExitError)
		assert.True(t, ok)
	})

	t.Run("context cancel", func(t *testing.T) {
		server := GameServer{}
		require.NoError(t, server.Start("", []string{"sleep", "inf"}))

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := server.wait(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestGameServer_Stop(t *testing.T) {
	assertProcessGroupEnded := func(pgid int) {
		t.Helper()
		require.Positive(t, pgid)
		assert.Error(t, syscall.ESRCH, syscall.Kill(-pgid, 0))
	}

	t.Run("not started", func(t *testing.T) {
		server := GameServer{}
		err := server.Stop(context.Background())
		assert.ErrorIs(t, err, ErrGameServerNotStarted)
	})

	t.Run("success", func(t *testing.T) {
		server := GameServer{}
		err := server.Start("", []string{"sleep", "inf"})
		require.NoError(t, err)

		pgid := server.pgid

		ctx := context.Background()
		err = server.Stop(ctx)
		require.NoError(t, err)
		assert.Nil(t, ctx.Err())

		assert.Equal(t, server, GameServer{})
		assertProcessGroupEnded(pgid)
	})

	t.Run("context cancel", func(t *testing.T) {
		server := GameServer{}
		err := server.Start("", []string{"echo"})
		require.NoError(t, err)

		pgid := server.pgid

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err = server.Stop(ctx)
		assert.ErrorIs(t, err, context.Canceled)
		assertProcessGroupEnded(pgid)
	})

	t.Run("process ignores SIGTERM", func(t *testing.T) {
		server := GameServer{}

		cmd := []string{"sh", "-c", "trap '' TERM; kill -USR1 $PPID; sleep inf"}
		err := server.Start("", cmd)
		require.NoError(t, err)

		pgid := server.pgid

		// wait for script to send signal to inform that he now ignores SIGTERM
		ready := make(chan os.Signal, 1)
		signal.Notify(ready, syscall.SIGUSR1)
		<-ready

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err = server.Stop(ctx)
		assert.ErrorIs(t, err, context.Canceled)
		assertProcessGroupEnded(pgid)
	})

	t.Run("process died but its child is alive", func(t *testing.T) {
		server := GameServer{}

		cmd := []string{"sh", "-c", "sleep inf & exit"}
		require.NoError(t, server.Start("", cmd))

		pgid := server.cmd.Process.Pid
		require.NoError(t, server.cmd.Wait())

		// assert child is alive
		assert.NoError(t, syscall.Kill(-pgid, 0))

		require.NoError(t, server.Stop(context.Background()))

		// wait for the process group to be gone
		assert.Eventually(t, func() bool {
			err := syscall.Kill(-pgid, 0)
			return errors.Is(err, syscall.ESRCH)
		}, 1*time.Second, 20*time.Millisecond)
	})
}

func TestGameServer_GetResult(t *testing.T) {
	t.Run("not started", func(t *testing.T) {
		server := GameServer{}
		_, err := server.GetResult(context.Background())
		assert.ErrorIs(t, err, ErrGameServerNotStarted)
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
			require.NoError(t, server.Start("", []string{"echo"}))
			defer func() { require.NoError(t, server.Stop(context.Background())) }()

			n, err := server.resultFile.Write(test.payload)
			require.NoError(t, err)
			assert.Equal(t, len(test.payload), n)

			res, err := server.GetResult(context.Background())
			require.NoError(t, err)
			assert.Equal(t, test.payload, res)
		})

	}

	t.Run("context canceled", func(t *testing.T) {
		server := GameServer{}
		require.NoError(t, server.Start("config", []string{"sleep", "inf"}))
		defer func() { require.NoError(t, server.Stop(context.Background())) }()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := server.GetResult(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestGameServer_LifeCycle(t *testing.T) {
	// sh -c "..." is used to ignore additional args (path to config, result)
	iterations := []struct {
		expected       []byte
		command        []string
		getResultError error
	}{
		{
			expected:       []byte{1, 2},
			command:        []string{"sh", "-c", "sleep 0.1"},
			getResultError: nil,
		},
		{
			expected:       []byte{3},
			command:        []string{"sh", "-c", "sleep 0.1"},
			getResultError: nil,
		},
		{
			expected:       []byte{4},
			command:        []string{"sh", "-c", "sleep inf"},
			getResultError: context.DeadlineExceeded,
		},
		{
			expected:       []byte{5, 6, 7},
			command:        []string{"sh", "-c", "sleep 0.1"},
			getResultError: nil,
		},
	}

	server := GameServer{}

	for idx, iteration := range iterations {
		t.Logf("iteration %v", idx)

		err := server.Start("config", iteration.command)
		require.NoError(t, err)

		n, err := server.resultFile.Write(iteration.expected)
		require.NoError(t, err)
		assert.Equal(t, len(iteration.expected), n)

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		res, err := server.GetResult(ctx)
		if iteration.getResultError == nil {
			require.NoError(t, err)
		} else {
			require.ErrorIs(t, err, iteration.getResultError)
		}

		if err == nil {
			assert.Equal(t, iteration.expected, res)
		}

		err = server.Stop(context.Background())
		require.NoError(t, err)
		require.Equal(t, GameServer{}, server)
	}
}
