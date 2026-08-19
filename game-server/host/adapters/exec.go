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

var (
	ErrFailedToCreateTempFile  = errors.New("failed to create temp file")
	ErrFailedToWriteConfig     = errors.New("failed to write config")
	ErrGameServerFailedToStart = errors.New("failed to start game server")
	ErrExecutorNotActive       = errors.New("executor not active")
	ErrExecutorAlreadyActive   = errors.New("executor already active")
)

type Executor struct {
	command []string

	active bool

	configFile *os.File
	resultFile *os.File

	cmd       *exec.Cmd
	cmdWaiter *CmdWaiter
	pgid      int
}

func NewExecutor(command []string) *Executor {
	return &Executor{command: slices.Clone(command)}
}

// starts the executor with given command
// NOTE: requires command to be non-empty
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

	command := attachParams(slices.Clone(s.command), s.configFile.Name(), s.resultFile.Name())
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
		removeFile(s.configFile, "config file", slog.Default())
		s.configFile = nil
	}

	if s.resultFile != nil {
		removeFile(s.resultFile, "result file", slog.Default())
		s.resultFile = nil
	}

	s.active = false

	return returnErr
}

func (s *Executor) stopCommand(ctx context.Context) error {
	if s.cmd == nil || s.cmd.Process == nil {
		return nil
	}

	s.cmdWaiter.WaitAsync()

	if err := s.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		slog.Warn("failed to send SIGTERM", "pid", s.cmd.Process.Pid, "err", err)
	}

	// TODO: add force kill after X seconds
	if err := s.cmdWaiter.Wait(ctx); errors.Is(err, context.Canceled) {
		killProcessGroup(s.pgid)
		_ = s.cmdWaiter.Wait(context.Background())
		return err
	} else {
		killProcessGroup(s.pgid)
		return nil
	}
}

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

func createTempFile(subname string) (*os.File, error) {
	configFile, err := os.CreateTemp("", fmt.Sprintf("game-server-%v-*", subname))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToCreateTempFile, err)
	}
	return configFile, nil
}

func attachParams(cmdArgs []string, configFileName string, resultFileName string) []string {
	return append(cmdArgs,
		"--match-config", configFileName,
		"--match-result", resultFileName,
	)
}

func removeFile(file *os.File, name string, logger *slog.Logger) {
	if err := file.Close(); err != nil {
		logger.Warn("failed to close temp file", "err", err, "fileName", name)
	}
	if err := os.Remove(file.Name()); err != nil {
		logger.Warn("failed to remove temp file", "err", err, "fileName", name)
	}
}

func killProcessGroup(pgid int) {
	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
		slog.Warn("failed to kill remaining children", "pgid", pgid, "err", err)
	}
}
