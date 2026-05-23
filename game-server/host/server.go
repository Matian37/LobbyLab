package main

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	ErrGameServerNoConfig       = errors.New("game server has no config")
	ErrGameServerAlreadyClosed  = errors.New("game server already closed")
	ErrGameServerAlreadyStarted = errors.New("game server already started")
)

type GameServer struct {
	started     bool
	closed      bool
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

	if s.closed {
		return ErrGameServerAlreadyClosed
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
	if s.closed {
		return ErrGameServerAlreadyClosed
	}
	defer s.cleanup()

	s.startWait()

	s.cmd.Process.Signal(syscall.SIGTERM)

	select {
	case <-s.waitChannel:
		// kill remaining children
		syscall.Kill(-s.pgid, syscall.SIGKILL)
		s.closed = true
		return nil
	case <-ctx.Done():
		syscall.Kill(-s.pgid, syscall.SIGKILL)
		<-s.waitChannel
		s.closed = true
		return ctx.Err()
	}
}

func (s *GameServer) Wait(ctx context.Context) error {
	if !s.started {
		return ErrGameServerNotStarted
	}
	if s.closed {
		return ErrGameServerAlreadyClosed
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

func (s *GameServer) GetResult(ctx context.Context) ([]byte, error) {
	if !s.started {
		return []byte{}, ErrGameServerNotStarted
	}
	if s.closed {
		return []byte{}, ErrGameServerAlreadyClosed
	}

	if err := s.Wait(ctx); err != nil {
		return []byte{}, err
	}
	defer s.Stop(ctx)

	s.resultFile.Seek(0, 0)
	result, err := io.ReadAll(s.resultFile)
	if err != nil {
		return []byte{}, err
	}
	return result, nil
}

// start wait goroutine and create waitChannel
//
// Note: use this instead of s.cmd.Wait() and only when cmd has started
func (s *GameServer) startWait() error {
	if s.waitChannel == nil {
		s.waitChannel = make(chan error, 1)
		go func() {
			s.waitChannel <- s.cmd.Wait()
		}()
	}
	return nil
}

func (s *GameServer) cleanup() {
	if s.configFile != nil {
		s.configFile.Close()
		os.Remove(s.configFile.Name())
		s.configFile = nil
	}
	if s.resultFile != nil {
		s.resultFile.Close()
		os.Remove(s.resultFile.Name())
		s.resultFile = nil
	}
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
