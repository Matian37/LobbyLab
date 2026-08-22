package adapters

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testLogger = slog.New(slog.DiscardHandler)

func TestGenerateCommand(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		expected := []string{"a", "--match-config", "b", "--match-result", "c"}
		inputCommand := []string{"a"}

		result := generateCommand(inputCommand, "b", "c")
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

func TestRemoveFile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		file, err := os.CreateTemp("", "*")
		require.NoError(t, err)
		path := file.Name()

		removeFile(file, "test file", slog.Default())

		_, err = os.Stat(path)
		assert.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestKillProcessGroup(t *testing.T) {
	t.Run("kills group but leaves different one", func(t *testing.T) {
		waitWithTimeout := func(cmd *exec.Cmd) {
			t.Helper()
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("process did not exit in time")
			}
		}

		leader := exec.Command("sleep", "inf")
		leader.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		require.NoError(t, leader.Start())
		group := leader.Process.Pid

		member := exec.Command("sleep", "inf")
		member.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: group}
		require.NoError(t, member.Start())

		outsider := exec.Command("sleep", "inf")
		outsider.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		require.NoError(t, outsider.Start())

		killProcessGroup(group, slog.Default())

		// reap group members so the process group is actually gone
		waitWithTimeout(leader)
		waitWithTimeout(member)

		assert.Eventually(t, func() bool {
			return errors.Is(syscall.Kill(-group, 0), syscall.ESRCH)
		}, 1*time.Second, 20*time.Millisecond)

		// outsider in a different group is left untouched
		assert.NoError(t, syscall.Kill(-outsider.Process.Pid, 0))
		_ = outsider.Process.Kill()
		waitWithTimeout(outsider)
	})
}

func TestExecutor_Start(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		exc := NewExecutor([]string{"echo"}, testLogger)

		err := exc.Start("")
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
		exc := NewExecutor([]string{"echo"}, testLogger)
		err := exc.Start("")
		require.NoError(t, err)

		err = exc.Start("")
		assert.ErrorIs(t, err, ErrExecutorAlreadyActive)
	})

	t.Run("partial start", func(t *testing.T) {
		exc := NewExecutor([]string{}, testLogger)
		err := exc.Start("")
		require.ErrorIs(t, err, ErrGameServerFailedToStart)

		assert.True(t, exc.active)
		assert.NotNil(t, exc.configFile)
		assert.NotNil(t, exc.resultFile)
		assert.NotNil(t, exc.cmd)
		assert.Zero(t, exc.pgid)
	})

	t.Run("does not mutate command", func(t *testing.T) {
		command := make([]string, 1, 4)
		command[0] = "echo"

		exc := NewExecutor(command, testLogger)
		require.NoError(t, exc.Start(""))
		defer func() { require.NoError(t, exc.Stop(context.Background())) }()

		assert.Equal(t, []string{"echo"}, command)
		assert.Equal(t, []string{"echo"}, exc.cmdArgs)
		assert.NotSame(t, &command[0], &exc.cmdArgs[0])
	})
}

func TestExecutor_Stop(t *testing.T) {
	assertProcessGroupEnded := func(pgid int) {
		t.Helper()
		require.Positive(t, pgid)
		assert.Error(t, syscall.ESRCH, syscall.Kill(-pgid, 0))
	}

	t.Run("not started", func(t *testing.T) {
		exc := NewExecutor(nil, testLogger)
		err := exc.Stop(context.Background())
		assert.ErrorIs(t, err, ErrExecutorNotActive)
	})

	t.Run("success", func(t *testing.T) {
		exc := NewExecutor([]string{"sleep", "inf"}, testLogger)
		err := exc.Start("")
		require.NoError(t, err)

		pgid := exc.pgid

		ctx := context.Background()
		err = exc.Stop(ctx)
		require.NoError(t, err)
		assert.Nil(t, ctx.Err())

		assert.Equal(t, &Executor{cmdArgs: []string{"sleep", "inf"}, logger: exc.logger}, exc)
		assertProcessGroupEnded(pgid)
	})

	t.Run("context cancel", func(t *testing.T) {
		exc := NewExecutor([]string{"echo"}, testLogger)
		err := exc.Start("")
		require.NoError(t, err)

		pgid := exc.pgid

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err = exc.Stop(ctx)
		assert.ErrorIs(t, err, context.Canceled)
		assertProcessGroupEnded(pgid)
	})

	t.Run("process ignores SIGTERM", func(t *testing.T) {
		cmd := []string{"sh", "-c", "trap '' TERM; kill -USR1 $PPID; sleep inf"}
		exc := NewExecutor(cmd, testLogger)
		err := exc.Start("")
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
		cmd := []string{"sh", "-c", "sleep inf & exit"}
		exc := NewExecutor(cmd, testLogger)
		require.NoError(t, exc.Start(""))

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

	t.Run("partially started", func(t *testing.T) {
		exc := NewExecutor([]string{}, testLogger)
		err := exc.Start("")
		require.Error(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		require.NoError(t, exc.Stop(ctx))
		assert.False(t, exc.active)
		assert.Nil(t, exc.configFile)
		assert.Nil(t, exc.resultFile)
		assert.Nil(t, exc.cmd)
		assert.Zero(t, exc.pgid)
	})

	t.Run("does not mutate command", func(t *testing.T) {
		command := make([]string, 1, 4)
		command[0] = "echo"

		exc := NewExecutor(command, testLogger)
		require.NoError(t, exc.Start(""))
		require.NoError(t, exc.Stop(context.Background()))

		assert.Equal(t, []string{"echo"}, command)
		assert.Equal(t, []string{"echo"}, exc.cmdArgs)
	})

	t.Run("partially initialized", func(t *testing.T) {
		tests := []struct {
			name  string
			setup func(t *testing.T) *Executor
		}{
			{
				name:  "active only",
				setup: func(t *testing.T) *Executor { return &Executor{active: true} },
			},
			{
				name: "cmd nil but pgid set",
				setup: func(t *testing.T) *Executor {
					return &Executor{active: true, pgid: 1234}
				},
			},
			{
				name: "config file only",
				setup: func(t *testing.T) *Executor {
					file, err := os.CreateTemp("", "*")
					require.NoError(t, err)
					return &Executor{active: true, configFile: file}
				},
			},
			{
				name: "result file only",
				setup: func(t *testing.T) *Executor {
					file, err := os.CreateTemp("", "*")
					require.NoError(t, err)
					return &Executor{active: true, resultFile: file}
				},
			},
			{
				name: "cmd waiter only",
				setup: func(t *testing.T) *Executor {
					return &Executor{active: true, cmdWaiter: new(CmdWaiter)}
				},
			},
			{
				name: "all but cmd",
				setup: func(t *testing.T) *Executor {
					config, err := os.CreateTemp("", "*")
					require.NoError(t, err)
					result, err := os.CreateTemp("", "*")
					require.NoError(t, err)
					return &Executor{
						active:     true,
						pgid:       1234,
						configFile: config,
						resultFile: result,
						cmdWaiter:  new(CmdWaiter),
					}
				},
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				exc := test.setup(t)

				paths := []string{}
				if exc.configFile != nil {
					paths = append(paths, exc.configFile.Name())
				}
				if exc.resultFile != nil {
					paths = append(paths, exc.resultFile.Name())
				}

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				require.NoError(t, exc.Stop(ctx))
				assert.Equal(t, Executor{}, *exc)

				for _, path := range paths {
					_, err := os.Stat(path)
					assert.ErrorIs(t, err, os.ErrNotExist)
				}
			})
		}
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
			exc := NewExecutor([]string{"echo"}, testLogger)
			require.NoError(t, exc.Start(""))
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
		exc := NewExecutor([]string{"sleep", "inf"}, testLogger)
		require.NoError(t, exc.Start("config"))
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

	for idx, iteration := range iterations {
		t.Logf("iteration %v", idx)

		exc := NewExecutor(iteration.command, testLogger)

		err := exc.Start("config")
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
		require.Equal(t, &Executor{cmdArgs: iteration.command, logger: exc.logger}, exc)
	}
}
