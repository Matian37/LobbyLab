package adapters

import (
	"context"
	"os/exec"
)

// Note: this struct is not thread-safe.
type CmdWaiter struct {
	Cmd     *exec.Cmd
	resChan chan error
}

func NewCmdWaiter(cmd *exec.Cmd) *CmdWaiter {
	if cmd == nil {
		panic("cmd provided with nil value")
	}
	return &CmdWaiter{Cmd: cmd}
}

func (c *CmdWaiter) Wait(ctx context.Context) error {
	c.WaitAsync()

	select {
	case err := <-c.resChan:
		c.resChan <- err
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *CmdWaiter) WaitAsync() {
	if c.resChan != nil {
		return
	}

	c.resChan = make(chan error, 1)
	go func() {
		c.resChan <- c.Cmd.Wait()
	}()
}
