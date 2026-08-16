package main

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
	ErrCommandEmpty             = errors.New("args not provided")
	ErrFailedToCreateTempFile   = errors.New("failed to create temp file")
	ErrFailedToWriteConfig      = errors.New("failed to write config")
	ErrGameServerStartFailed    = errors.New("failed to start game server")
	ErrGameServerNotStarted     = errors.New("game server not started")
	ErrGameServerAlreadyStarted = errors.New("game server already started")
)

type GameServer struct {
	started     bool
	configFile  *os.File
	resultFile  *os.File
	cmd         *exec.Cmd
	pgid        int
	waitChannel chan error
}

func (s *GameServer) Start(config string, command []string) error {
	if len(command) == 0 {
		return ErrCommandEmpty
	}
	if s.started {
		return ErrGameServerAlreadyStarted
	}

	configFile, err := createTempFile("config")
	if err != nil {
		s.cleanup()
		return err
	}
	s.configFile = configFile

	if _, err := s.configFile.WriteString(config); err != nil {
		s.cleanup()
		return fmt.Errorf("%w: %w", ErrFailedToWriteConfig, err)
	}

	resultFile, err := createTempFile("result")
	if err != nil {
		s.cleanup()
		return err
	}
	s.resultFile = resultFile

	command = attachParams(slices.Clone(command), s.configFile.Name(), s.resultFile.Name())
	slog.Debug("no siema eniu odpalam", "gameServerArgs", s.cmd.Args)
	s.cmd = exec.Command(command[0], command[1:]...)
	s.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:   true,
		Pdeathsig: syscall.SIGKILL,
	}

	if err := s.cmd.Start(); err != nil {
		s.cleanup()
		return fmt.Errorf("%w: %w", ErrGameServerStartFailed, err)
	}
	s.pgid = s.cmd.Process.Pid
	s.started = true

	return nil
}

func (s *GameServer) Stop(ctx context.Context) error {
	// s.pgid == 0 means that process is already gone
	if !s.started {
		return ErrGameServerNotStarted
	}
	defer s.cleanup()

	s.startWait()

	if err := s.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		slog.Warn("failed to send SIGTERM", "pid", s.cmd.Process.Pid, "err", err)
	}

	select {
	case <-s.waitChannel:
		// kill remaining children
		if err := syscall.Kill(-s.pgid, syscall.SIGKILL); err != nil {
			slog.Warn("failed to kill remaining children", "pgid", s.pgid, "err", err)
		}
		return nil
	case <-ctx.Done():
		if err := syscall.Kill(-s.pgid, syscall.SIGKILL); err != nil {
			slog.Warn("failed to kill remaining children", "pgid", s.pgid, "err", err)
		}
		<-s.waitChannel
		return ctx.Err()
	}
}

// Note: function does not stop cmd, always run Stop function manually
func (s *GameServer) GetResult(ctx context.Context) ([]byte, error) {
	if !s.started {
		return []byte{}, ErrGameServerNotStarted
	}

	if err := s.wait(ctx); err != nil {
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

func (s *GameServer) wait(ctx context.Context) error {
	if !s.started {
		return ErrGameServerNotStarted
	}

	s.startWait()
	select {
	case err := <-s.waitChannel:
		s.waitChannel <- err
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// start wait goroutine and create waitChannel
//
// Note: use this instead of s.cmd.Wait() and only when cmd has started
func (s *GameServer) startWait() {
	if s.waitChannel == nil {
		s.waitChannel = make(chan error, 1)
		go func() {
			s.waitChannel <- s.cmd.Wait()
		}()
	}
}

func (s *GameServer) cleanup() {
	if s.configFile != nil {
		if err := s.configFile.Close(); err != nil {
			slog.Warn("failed to close config file", "err", err)
		}
		if err := os.Remove(s.configFile.Name()); err != nil {
			slog.Warn("failed to remove config file", "err", err)
		}
		s.configFile = nil
	}
	if s.resultFile != nil {
		if err := s.resultFile.Close(); err != nil {
			slog.Warn("failed to close result file", "err", err)
		}
		if err := os.Remove(s.resultFile.Name()); err != nil {
			slog.Warn("failed to remove result file", "err", err)
		}
		s.resultFile = nil
	}
	if s.waitChannel != nil {
		close(s.waitChannel)
		s.waitChannel = nil
	}
	s.cmd = nil
	s.pgid = 0
	s.started = false
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
