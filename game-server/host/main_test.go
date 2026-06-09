package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		fail     bool
		expected []string
	}{
		{
			name: "too few arguments",
			args: []string{"x"},
			fail: true,
		},
		{
			name: "too many arguments",
			args: []string{"x", "./x", "x"},
			fail: true,
		},
		{
			name: "unterminated quote",
			args: []string{"/", "./game --name \"unclosed"},
			fail: true,
		},
		{
			name:     "success",
			args:     []string{"/", "./game args"},
			fail:     false,
			expected: []string{"./game", "args"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := ParseArgs(tc.args)

			if tc.fail {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, res)
			}
		})
	}
}

func TestParseArgs_InputNotModified(t *testing.T) {
	original := []string{"/", "./x"}
	result, err := ParseArgs(original)
	require.NoError(t, err)
	assert.Equal(t, []string{"./x"}, result)
	assert.Equal(t, []string{"/", "./x"}, original, "input slice must not be modified")
}

func TestSetupLogger(t *testing.T) {
	originalLogger := slog.Default()
	defer slog.SetDefault(originalLogger)

	var buf bytes.Buffer
	setupLoggerWithWriter(&buf)

	slog.Info("test message", "extra_key", "extra_val")

	var parsed map[string]any
	err := json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)

	expectedHostname, err := os.Hostname()
	if err != nil {
		expectedHostname = "unknown"
	}
	assert.Equal(t, expectedHostname, parsed["hostname"])
	assert.Equal(t, "test message", parsed["msg"])
	assert.Equal(t, "extra_val", parsed["extra_key"])
}
