package internal

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogger(t *testing.T) {
	t.Run("emits hostname and message", func(t *testing.T) {
		cfg := Config{WorkerID: "test-host", LogLevel: slog.LevelInfo}

		var buf bytes.Buffer
		logger := NewLogger(&buf, cfg)

		logger.Info("test message", "extra_key", "extra_val")

		var parsed map[string]any
		err := json.Unmarshal(buf.Bytes(), &parsed)
		require.NoError(t, err)

		assert.Equal(t, "test-host", parsed["workerID"])
		assert.Equal(t, "test message", parsed["msg"])
		assert.Equal(t, "extra_val", parsed["extra_key"])
	})

	t.Run("respects log level", func(t *testing.T) {
		cfg := Config{WorkerID: "test-host", LogLevel: slog.LevelInfo}

		var buf bytes.Buffer
		logger := NewLogger(&buf, cfg)

		logger.Debug("debug message")
		logger.Info("info message")

		lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
		require.Len(t, lines, 1)

		var parsed map[string]any
		err := json.Unmarshal(lines[0], &parsed)
		require.NoError(t, err)
		assert.Equal(t, "info message", parsed["msg"])
	})
}
