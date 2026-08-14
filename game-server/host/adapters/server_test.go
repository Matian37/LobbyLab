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

func TestExecutor_Cleanup(t *testing.T) {
	t.Run("nil files", func(t *testing.T) {
		exc := Executor{}
		exc.cleanup()
	})

	t.Run("success", func(t *testing.T) {
		exc := Executor{
			active:      true,
			cmd:         exec.Command("echo"),
			pgid:        1,
			waitChannel: make(chan error),
		}
		configFile, err := os.CreateTemp("", "*")
		require.NoError(t, err)
		exc.configFile = configFile
		resultFile, err := os.CreateTemp("", "*")
		require.NoError(t, err)
		exc.resultFile = resultFile

		configPath := exc.configFile.Name()
		resultPath := exc.resultFile.Name()

		exc.cleanup()
		require.Equal(t, Executor{}, exc)

		_, err = os.Stat(configPath)
		assert.ErrorIs(t, err, os.ErrNotExist)
		_, err = os.Stat(resultPath)
		assert.Error(t, err, os.ErrNotExist)
	})
}

func TestExecutor_StartWait(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		exc := Executor{}
		require.NoError(t, exc.Start("", []string{"echo"}))

		exc.startWait()
		require.NotNil(t, exc.waitChannel)

		assert.NoError(t, exc.Stop(context.Background()))
	})
}

func TestExecutor_Start(t *testing.T) {
	t.Run("no command", func(t *testing.T) {
		exc := Executor{}
		err := exc.Start("", []string{})
		assert.ErrorIs(t, err, ErrCommandEmpty)
		assert.False(t, exc.active)
	})

	t.Run("success", func(t *testing.T) {
		exc := Executor{}

		err := exc.Start("", []string{"echo"})
		require.NoError(t, err)

		require.NotNil(t, exc.cmd)
		require.NotNil(t, exc.cmd.Process)
		require.NotNil(t, exc.configFile)
		require.NotNil(t, exc.resultFile)
		assert.NotZero(t, exc.pgid)
		assert.True(t, exc.active)

		_, err = exc.configFile.Stat()
		assert.NoError(t, err)
		_, err = exc.configFile.Stat()
		assert.NoError(t, err)

		pgid, err := syscall.Getpgid(exc.cmd.Process.Pid)
		require.NoError(t, err)
		assert.Equal(t, pgid, exc.pgid)

	})

	t.Run("already started", func(t *testing.T) {
		exc := Executor{}
		err := exc.Start("", []string{"echo"})
		require.NoError(t, err)

		err = exc.Start("", []string{"echo"})
		assert.ErrorIs(t, err, ErrExecutorAlreadyActive)
	})
}

func TestExecutor_Wait(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		exc := Executor{}
		require.NoError(t, exc.Start("", []string{"sh", "-c", "exit 1"}))

		err := exc.wait(context.Background())

		_, ok := err.(*exec.ExitError)
		assert.True(t, ok)
	})

	t.Run("context cancel", func(t *testing.T) {
		exc := Executor{}
		require.NoError(t, exc.Start("", []string{"sleep", "inf"}))

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := exc.wait(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestExecutor_Stop(t *testing.T) {
	assertProcessGroupEnded := func(pgid int) {
		t.Helper()
		require.Positive(t, pgid)
		assert.Error(t, syscall.ESRCH, syscall.Kill(-pgid, 0))
	}

	t.Run("not started", func(t *testing.T) {
		exc := Executor{}
		err := exc.Stop(context.Background())
		assert.ErrorIs(t, err, ErrExecutorNotActive)
	})

	t.Run("success", func(t *testing.T) {
		exc := Executor{}
		err := exc.Start("", []string{"sleep", "inf"})
		require.NoError(t, err)

		pgid := exc.pgid

		ctx := context.Background()
		err = exc.Stop(ctx)
		require.NoError(t, err)
		assert.Nil(t, ctx.Err())

		assert.Equal(t, exc, Executor{})
		assertProcessGroupEnded(pgid)
	})

	t.Run("context cancel", func(t *testing.T) {
		exc := Executor{}
		err := exc.Start("", []string{"echo"})
		require.NoError(t, err)

		pgid := exc.pgid

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err = exc.Stop(ctx)
		assert.ErrorIs(t, err, context.Canceled)
		assertProcessGroupEnded(pgid)
	})

	t.Run("process ignores SIGTERM", func(t *testing.T) {
		exc := Executor{}

		cmd := []string{"sh", "-c", "trap '' TERM; kill -USR1 $PPID; sleep inf"}
		err := exc.Start("", cmd)
		require.NoError(t, err)

		pgid := exc.pgid

		// wait for script to send signal to inform that he now ignores SIGTERM
		ready := make(chan os.Signal, 1)
		signal.Notify(ready, syscall.SIGUSR1)
		<-ready

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err = exc.Stop(ctx)
		assert.ErrorIs(t, err, context.Canceled)
		assertProcessGroupEnded(pgid)
	})

	t.Run("process died but its child is alive", func(t *testing.T) {
		exc := Executor{}

		cmd := []string{"sh", "-c", "sleep inf & exit"}
		require.NoError(t, exc.Start("", cmd))

		pgid := exc.cmd.Process.Pid
		require.NoError(t, exc.cmd.Wait())

		// assert child is alive
		assert.NoError(t, syscall.Kill(-pgid, 0))

		require.NoError(t, exc.Stop(context.Background()))

		// wait for the process group to be gone
		assert.Eventually(t, func() bool {
			err := syscall.Kill(-pgid, 0)
			return errors.Is(err, syscall.ESRCH)
		}, 1*time.Second, 20*time.Millisecond)
	})
}

func TestExecutor_GetResult(t *testing.T) {
	t.Run("not started", func(t *testing.T) {
		exc := Executor{}
		_, err := exc.GetResult(context.Background())
		assert.ErrorIs(t, err, ErrExecutorNotActive)
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
			exc := Executor{}
			require.NoError(t, exc.Start("", []string{"echo"}))
			defer func() { require.NoError(t, exc.Stop(context.Background())) }()

			n, err := exc.resultFile.Write(test.payload)
			require.NoError(t, err)
			assert.Equal(t, len(test.payload), n)

			res, err := exc.GetResult(context.Background())
			require.NoError(t, err)
			assert.Equal(t, test.payload, res)
		})

	}

	t.Run("context canceled", func(t *testing.T) {
		exc := Executor{}
		require.NoError(t, exc.Start("config", []string{"sleep", "inf"}))
		defer func() { require.NoError(t, exc.Stop(context.Background())) }()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := exc.GetResult(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestExecutor_LifeCycle(t *testing.T) {
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

	exc := Executor{}

	for idx, iteration := range iterations {
		t.Logf("iteration %v", idx)

		err := exc.Start("config", iteration.command)
		require.NoError(t, err)

		n, err := exc.resultFile.Write(iteration.expected)
		require.NoError(t, err)
		assert.Equal(t, len(iteration.expected), n)

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		res, err := exc.GetResult(ctx)
		if iteration.getResultError == nil {
			require.NoError(t, err)
		} else {
			require.ErrorIs(t, err, iteration.getResultError)
		}

		if err == nil {
			assert.Equal(t, iteration.expected, res)
		}

		err = exc.Stop(context.Background())
		require.NoError(t, err)
		require.Equal(t, Executor{}, exc)
	}
}
