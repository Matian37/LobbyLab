package adapters

import (
	"context"
	"os/exec"
)

// Used for running non-blocking wait operations on exec.Cmd.
type CmdWaiter struct {
	Cmd     *exec.Cmd
	resChan chan error
}

// NewCmdWaiter builds waiter for the given command.
// Panics on a nil cmd.
func NewCmdWaiter(cmd *exec.Cmd) *CmdWaiter {
	if cmd == nil {
		panic("cmd provided with nil value")
	}
	return &CmdWaiter{Cmd: cmd}
}

// Waits for cmd to complete and returns its error result.
// This is a blocking call. When completed, the same error will be
// returned every time when called again.
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

// Starts waiting for the process to complete. Does not block.
func (c *CmdWaiter) WaitAsync() {
	if c.resChan != nil {
		return
	}

	c.resChan = make(chan error, 1)
	go func() {
		c.resChan <- c.Cmd.Wait()
	}()
}
