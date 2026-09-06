package adapters

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCmdWaiter(t *testing.T) {
	t.Run("nil cmd panics", func(t *testing.T) {
		assert.Panics(t, func() { NewCmdWaiter(nil) })
	})

	t.Run("success", func(t *testing.T) {
		cmd := exec.Command("echo")
		w := NewCmdWaiter(cmd)
		assert.Equal(t, cmd, w.Cmd)
	})
}

func TestCmdWaiter_WaitAsync(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cmd := exec.Command("echo")
		require.NoError(t, cmd.Start())

		w := NewCmdWaiter(cmd)
		w.WaitAsync()
		require.NotNil(t, w.resChan)

		require.NoError(t, w.Wait(context.Background()))
	})

	t.Run("idempotent", func(t *testing.T) {
		cmd := exec.Command("echo")
		require.NoError(t, cmd.Start())

		w := NewCmdWaiter(cmd)
		w.WaitAsync()
		require.NotNil(t, w.resChan)

		ch := w.resChan
		require.NotPanics(t, func() { w.WaitAsync() })
		assert.Equal(t, ch, w.resChan)
	})
}

func TestCmdWaiter_Wait(t *testing.T) {
	t.Run("context cancel", func(t *testing.T) {
		cmd := exec.Command("sleep", "inf")
		require.NoError(t, cmd.Start())

		w := NewCmdWaiter(cmd)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := w.Wait(ctx)
		assert.ErrorIs(t, err, context.Canceled)

		t.Cleanup(func() {
			require.NoError(t, cmd.Process.Kill())
			require.Error(t, cmd.Wait())
		})
	})

	t.Run("success", func(t *testing.T) {
		cmd := exec.Command("sh", "-c", "exit 1")
		require.NoError(t, cmd.Start())

		w := NewCmdWaiter(cmd)
		err := w.Wait(context.Background())

		_, ok := err.(*exec.ExitError)
		assert.True(t, ok)
	})

	t.Run("re-queues value for subsequent Wait", func(t *testing.T) {
		cmd := exec.Command("sh", "-c", "exit 1")
		require.NoError(t, cmd.Start())

		w := NewCmdWaiter(cmd)
		first := w.Wait(context.Background())
		second := w.Wait(context.Background())

		assert.Equal(t, first, second)
	})
}
