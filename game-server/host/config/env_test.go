package config

import (
	"log/slog"
	"os"
	"slices"
	"testing"

	"github.com/caarlos0/env/v11"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setAllEnvExcept(t *testing.T, except ...string) {
	t.Helper()

	for _, e := range []struct{ key, val string }{
		{"NATS_URI", "nats://localhost:4222"},
		{"LOG_LEVEL", "info"},
	} {
		if !slices.Contains(except, e.key) {
			t.Setenv(e.key, e.val)
		}
	}
}

func setArgs(t *testing.T, args []string) {
	t.Helper()

	original := os.Args
	os.Args = args
	t.Cleanup(func() { os.Args = original })
}

func TestParseArgs(t *testing.T) {
	t.Run("too few arguments", func(t *testing.T) {
		_, err := parseArgs([]string{"/bin/server"})
		assert.Error(t, err)
	})

	t.Run("too many arguments", func(t *testing.T) {
		_, err := parseArgs([]string{"/bin/server", "./game", "extra"})
		assert.Error(t, err)
	})

	t.Run("unterminated quote", func(t *testing.T) {
		_, err := parseArgs([]string{"/bin/server", "./game --name \"unclosed"})
		assert.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		res, err := parseArgs([]string{"/bin/server", "./game --name \"my game\""})
		require.NoError(t, err)
		assert.Equal(t, []string{"./game", "--name", "my game"}, res)
	})
}

func TestReadConfig(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		setAllEnvExcept(t)
		setArgs(t, []string{"/bin/server", "./game --name \"my game\""})

		cfg, err := ReadConfig()
		require.NoError(t, err)

		assert.Equal(t, "nats://localhost:4222", cfg.BrokerURI)
		assert.Equal(t, slog.LevelInfo, cfg.LogLevel)
		assert.Equal(t, []string{"./game", "--name", "my game"}, cfg.GameServerArgs)

		hostname, err := os.Hostname()
		require.NoError(t, err)
		assert.Equal(t, hostname, cfg.Hostname)
	})

	t.Run("missing required field", func(t *testing.T) {
		for _, field := range []string{"NATS_URI", "LOG_LEVEL"} {
			t.Run(field, func(t *testing.T) {
				setAllEnvExcept(t, field)
				setArgs(t, []string{"/bin/server", "./game"})

				_, err := ReadConfig()
				require.Error(t, err)
				assert.Equal(t, err.Error(), "env: "+env.VarIsNotSetError{Key: field}.Error())
			})
		}
	})

	t.Run("empty field", func(t *testing.T) {
		for _, field := range []string{"NATS_URI", "LOG_LEVEL"} {
			t.Run(field, func(t *testing.T) {
				setAllEnvExcept(t)
				t.Setenv(field, "")
				setArgs(t, []string{"/bin/server", "./game"})

				_, err := ReadConfig()
				require.Error(t, err)
				assert.Equal(t, err.Error(), "env: "+env.EmptyVarError{Key: field}.Error())
			})
		}
	})

	t.Run("invalid log level", func(t *testing.T) {
		setAllEnvExcept(t)
		t.Setenv("LOG_LEVEL", "not-a-level")
		setArgs(t, []string{"/bin/server", "./game"})

		_, err := ReadConfig()
		assert.ErrorIs(t, err, ErrInvalidLogLevel)
	})
}
