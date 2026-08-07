package config

import (
	"log/slog"
	"slices"
	"testing"

	"server-manager/internal"

	"github.com/caarlos0/env/v11"
	"github.com/moby/moby/api/types/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setAllEnvExcept(t *testing.T, except ...string) *internal.EnvConfig {
	t.Helper()
	type entry struct{ key, val string }
	for _, e := range []entry{
		{"GAME_SERVER_IMAGE", "img:latest"},
		{"GAME_SERVER_COUNT", "2"},
		{"GAME_SERVER_EXPOSE_PORTS", "80,443/udp"},
		{"GAME_SERVER_CLIENT_PORT", "80"},
		{"NATS_URI", "nats://localhost:4222"},
		{"NATS_NETWORK_NAME", "network"},
		{"PUBLIC_HOST", "127.0.0.1"},
		{"PLAYERS_PER_ROOM", "2"},
		{"DATABASE_URI", "postgresql://a:b@localhost:5432/c"},
		{"LOG_LEVEL", "info"},
		{"GAME_SERVER_LOG_LEVEL", "debug"},
	} {
		if !slices.Contains(except, e.key) {
			t.Setenv(e.key, e.val)
		}
	}

	return &internal.EnvConfig{
		Image:       "img:latest",
		Workercount: 2,
		ExposePorts: network.PortSet{
			network.MustParsePort("80"):      {},
			network.MustParsePort("443/udp"): {},
		},
		ClientPort:         network.MustParsePort("80"),
		BrokerURI:          "nats://localhost:4222",
		BrokerNetworkName:  "network",
		PublicHost:         "127.0.0.1",
		PlayersPerRoom:     2,
		DatabaseURI:        "postgresql://a:b@localhost:5432/c",
		LogLevel:           slog.LevelInfo,
		GameServerLogLevel: slog.LevelDebug,
	}
}

func TestParsePorts(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		input := []string{"80", "400/udp", "8080/tcp"}
		ports, err := parsePorts(input)
		require.NoError(t, err)
		require.Len(t, ports, len(input))
		for _, p := range input {
			_, ok := ports[network.MustParsePort(p)]
			assert.True(t, ok, "missing port %s", p)
		}
	})

	t.Run("invalid port", func(t *testing.T) {
		_, err := parsePorts([]string{"abc"})
		assert.ErrorIs(t, err, ErrInvalidPortString)
	})

	t.Run("empty", func(t *testing.T) {
		ports, err := parsePorts([]string{})
		require.NoError(t, err)
		assert.Empty(t, ports)
	})
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantLevel slog.Level
		invalid   bool
	}{
		{
			name:      "empty string",
			value:     "",
			wantLevel: 0,
			invalid:   true,
		},
		{
			name:      "invalid log level",
			value:     "a",
			wantLevel: 0,
			invalid:   true,
		},
		{
			name:      "valid log level",
			value:     "debug",
			wantLevel: slog.LevelDebug,
			invalid:   false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := parseLogLevel(test.value)
			assert.Equal(t, res, test.wantLevel)
			assert.Equal(t, err != nil, test.invalid)
		})
	}
}

func TestReadConfig(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		expected := setAllEnvExcept(t)
		cfg, err := ReadConfig()
		require.NoError(t, err)
		assert.Equal(t, expected, cfg)
	})

	t.Run("port error", func(t *testing.T) {
		tests := []struct {
			name     string
			override string
			value    string
			err      error
		}{
			{
				name:     "invalid expose port",
				override: "GAME_SERVER_EXPOSE_PORTS",
				value:    "abc",
				err:      ErrInvalidPortString,
			},
			{
				name:     "invalid client port",
				override: "GAME_SERVER_CLIENT_PORT",
				value:    "abc",
				err:      ErrInvalidPortString,
			},
			{
				name:     "client port not in expose ports",
				override: "GAME_SERVER_CLIENT_PORT",
				value:    "1",
				err:      ErrClientPortNotInExposed,
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				setAllEnvExcept(t)
				t.Setenv(test.override, test.value)
				_, err := ReadConfig()
				assert.ErrorIs(t, err, test.err)
			})
		}
	})

	t.Run("worker count not positive", func(t *testing.T) {
		for _, count := range []string{"0", "-1"} {
			t.Run("count "+count, func(t *testing.T) {
				setAllEnvExcept(t)
				t.Setenv("GAME_SERVER_COUNT", count)
				_, err := ReadConfig()
				assert.ErrorIs(t, err, ErrWorkerCountNotPositive)
			})
		}
	})

	t.Run("playersPerRoom count less than two", func(t *testing.T) {
		for _, count := range []string{"-1", "0", "1"} {
			t.Run("count "+count, func(t *testing.T) {
				setAllEnvExcept(t)
				t.Setenv("PLAYERS_PER_ROOM", count)
				_, err := ReadConfig()
				assert.ErrorIs(t, err, ErrTooFewPlayersPerRoom)
			})
		}
	})

	t.Run("invalid log level", func(t *testing.T) {
		setAllEnvExcept(t)
		t.Setenv("LOG_LEVEL", "a")
		_, err := ReadConfig()
		require.Error(t, err)
	})

	t.Run("invalid game server log level", func(t *testing.T) {
		setAllEnvExcept(t)
		t.Setenv("GAME_SERVER_LOG_LEVEL", "a")
		_, err := ReadConfig()
		require.Error(t, err)
	})

	t.Run("missing required field", func(t *testing.T) {
		for _, field := range []string{
			"GAME_SERVER_IMAGE",
			"GAME_SERVER_COUNT",
			"GAME_SERVER_EXPOSE_PORTS",
			"GAME_SERVER_CLIENT_PORT",
			"NATS_URI",
			"NATS_NETWORK_NAME",
			"PUBLIC_HOST",
			"PLAYERS_PER_ROOM",
			"DATABASE_URI",
			"LOG_LEVEL",
			"GAME_SERVER_LOG_LEVEL",
		} {
			t.Run(field, func(t *testing.T) {
				setAllEnvExcept(t, field)
				_, err := ReadConfig()
				require.Error(t, err)
				assert.Equal(t, err.Error(), "env: "+env.VarIsNotSetError{Key: field}.Error())
			})
		}
	})

	t.Run("empty field", func(t *testing.T) {
		for _, field := range []string{
			"GAME_SERVER_IMAGE",
			"GAME_SERVER_EXPOSE_PORTS",
			"GAME_SERVER_CLIENT_PORT",
			"NATS_URI",
			"NATS_NETWORK_NAME",
			"PUBLIC_HOST",
			"DATABASE_URI",
			"LOG_LEVEL",
			"GAME_SERVER_LOG_LEVEL",
		} {
			t.Run(field, func(t *testing.T) {
				setAllEnvExcept(t)
				t.Setenv(field, "")

				_, err := ReadConfig()
				require.Error(t, err)
				assert.Equal(t, err.Error(), "env: "+env.EmptyVarError{Key: field}.Error())
			})
		}
	})
}
