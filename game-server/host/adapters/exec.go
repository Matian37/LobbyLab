package adapters

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"syscall"
)

// Errors returned by Executor operations.
var (
	ErrFailedToCreateTempFile  = errors.New("failed to create temp file")
	ErrFailedToWriteConfig     = errors.New("failed to write config")
	ErrGameServerFailedToStart = errors.New("failed to start game server")
	ErrExecutorNotActive       = errors.New("executor not active")
	ErrExecutorAlreadyActive   = errors.New("executor already active")
)

// Executor implements internal.Executor, which manages the life cycle of the
// actual game server process.
//
// When started it creates two temporary files: one for the match configuration and one
// for the result. It injects them into the command line arguments before launching
// the game server (--match-config <path> and --match-result <path>).
// Configuration file is for actual game server to read from and result file to write to.
//
// Match is finished when game server exits and nonzero exit code indicates a failure.
//
// Executor can be started again after stopping. It's reusable.
type Executor struct {
	cmdArgs []string
	logger  *slog.Logger

	active bool

	configFile *os.File
	resultFile *os.File

	cmd       *exec.Cmd
	cmdWaiter *CmdWaiter
	pgid      int
}

// NewExecutor builds an Executor for the given game-server command arguments.
func NewExecutor(cmdArgs []string, logger *slog.Logger) *Executor {
	return &Executor{
		cmdArgs: slices.Clone(cmdArgs),
		logger:  logger.With("component", "executor"),
	}
}

// Start launches the configured game server process with the given
// configuration. It returns an error when the executor is already active.
func (s *Executor) Start(config string) error {
	if s.active {
		return ErrExecutorAlreadyActive
	}
	s.active = true

	configFile, err := createTempFile("config")
	if err != nil {
		return err
	}
	s.configFile = configFile

	if _, err := s.configFile.WriteString(config); err != nil {
		return fmt.Errorf("%w: %w", ErrFailedToWriteConfig, err)
	}

	resultFile, err := createTempFile("result")
	if err != nil {
		return err
	}
	s.resultFile = resultFile

	command := generateCommand(s.cmdArgs, s.configFile.Name(), s.resultFile.Name())
	s.cmd = exec.Command(command[0], command[1:]...)
	s.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:   true,
		Pdeathsig: syscall.SIGKILL,
	}

	s.cmdWaiter = NewCmdWaiter(s.cmd)

	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("%w: %w", ErrGameServerFailedToStart, err)
	}
	s.pgid = s.cmd.Process.Pid

	return nil
}

// Stop stops the running game server process and cleans up resources.
func (s *Executor) Stop(ctx context.Context) error {
	if !s.active {
		return ErrExecutorNotActive
	}

	var returnErr error

	if s.cmd != nil {
		returnErr = s.stopCommand(ctx)
		s.cmd = nil
	}
	s.pgid = 0
	s.cmdWaiter = nil

	if s.configFile != nil {
		removeFile(s.configFile, "config", s.logger)
		s.configFile = nil
	}

	if s.resultFile != nil {
		removeFile(s.resultFile, "result", s.logger)
		s.resultFile = nil
	}

	s.active = false

	return returnErr
}

// Stops the running game server process.
//
// It sends SIGTERM to the process and waits for it to exit.
// After that it sends SIGKILL to the process group to kill leftover children.
// However, if the context is canceled, it sends SIGKILL immediately instead.
//
// Returns process exit error or nil if the process does not exist.
func (s *Executor) stopCommand(ctx context.Context) error {
	if s.cmd == nil || s.cmd.Process == nil {
		return nil
	}

	s.cmdWaiter.WaitAsync()

	if err := s.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		s.logger.Warn("failed to send SIGTERM", "pid", s.cmd.Process.Pid, "err", err)
	}

	if err := s.cmdWaiter.Wait(ctx); errors.Is(err, context.Canceled) {
		killProcessGroup(s.pgid, s.logger)
		_ = s.cmdWaiter.Wait(context.Background())
		return err
	} else {
		killProcessGroup(s.pgid, s.logger)
		return nil
	}
}

// GetResult blocks until the process exits, then reads the result file and
// returns its content.
func (s *Executor) GetResult(ctx context.Context) ([]byte, error) {
	if !s.active {
		return []byte{}, ErrExecutorNotActive
	}

	if err := s.cmdWaiter.Wait(ctx); err != nil {
		return []byte{}, err
	}

	_, err := s.resultFile.Seek(0, 0)
	if err != nil {
		return []byte{}, err
	}
	result, err := io.ReadAll(s.resultFile)
	if err != nil {
		return []byte{}, err
	}
	return result, nil
}

// createTempFile creates a uniquely named temporary file for specific subname
func createTempFile(subname string) (*os.File, error) {
	configFile, err := os.CreateTemp("", fmt.Sprintf("game-server-%v-*", subname))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToCreateTempFile, err)
	}
	return configFile, nil
}

// generateCommand returns the command with the config and result file paths
// appended as --match-config and --match-result flags.
func generateCommand(cmdArgs []string, configFileName string, resultFileName string) []string {
	return append(slices.Clone(cmdArgs),
		"--match-config", configFileName,
		"--match-result", resultFileName,
	)
}

// removeFile closes and deletes a file, on each failure it logs a warning.
func removeFile(file *os.File, name string, logger *slog.Logger) {
	if err := file.Close(); err != nil {
		logger.Warn(
			fmt.Sprintf("failed to close %v file", name),
			"err", err,
			"path", file.Name(),
		)
	}
	if err := os.Remove(file.Name()); err != nil {
		logger.Warn(
			fmt.Sprintf("failed to remove %v file", name),
			"err", err,
			"path", file.Name(),
		)
	}
}

func killProcessGroup(pgid int, logger *slog.Logger) {
	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
		logger.Warn("failed to kill process group", "pgid", pgid, "err", err)
	}
}
