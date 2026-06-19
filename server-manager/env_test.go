package main

import (
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
		{"GAME_SERVER_CLIENT_PORTS", "80"},
		{"NATS_URI", "nats://localhost:4222"},
		{"PUBLIC_HOST", "127.0.0.1"},
	} {
		skip := false
		for _, x := range except {
			if e.key == x {
				skip = true
				break
			}
		}
		if !skip {
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
		ClientPorts: network.PortSet{
			network.MustParsePort("80"): {},
		},
		BrokerURI:  "nats://localhost:4222",
		PublicHost: "127.0.0.1",
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

func TestIsSubset(t *testing.T) {
	type StrSet map[string]struct{}

	tests := []struct {
		name     string
		sub      StrSet
		super    StrSet
		expected bool
	}{
		{
			name:     "full match",
			sub:      StrSet{"a": {}, "b": {}},
			super:    StrSet{"a": {}, "b": {}},
			expected: true,
		},
		{
			name:     "partial match",
			sub:      StrSet{"b": {}},
			super:    StrSet{"a": {}, "b": {}},
			expected: true,
		},
		{
			name:     "sub additional key",
			sub:      StrSet{"a": {}, "b": {}, "c": {}},
			super:    StrSet{"a": {}, "b": {}},
			expected: false,
		},
		{
			name:     "empty super",
			sub:      StrSet{"a": {}},
			super:    StrSet{},
			expected: false,
		},
		{
			name:     "empty sub",
			sub:      StrSet{},
			super:    StrSet{"a": {}},
			expected: true,
		},
		{
			name:     "both empty",
			sub:      StrSet{},
			super:    StrSet{},
			expected: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, isSubset(test.sub, test.super), test.expected)
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
				override: "GAME_SERVER_CLIENT_PORTS",
				value:    "abc",
				err:      ErrInvalidPortString,
			},
			{
				name:     "client ports not subset of expose ports",
				override: "GAME_SERVER_CLIENT_PORTS",
				value:    "1,2,3,4,5",
				err:      ErrClientPortsNotSubset,
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

	t.Run("missing required field", func(t *testing.T) {
		for _, field := range []string{
			"GAME_SERVER_IMAGE",
			"GAME_SERVER_COUNT",
			"GAME_SERVER_EXPOSE_PORTS",
			"GAME_SERVER_CLIENT_PORTS",
			"NATS_URI",
			"PUBLIC_HOST",
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
			"GAME_SERVER_CLIENT_PORTS",
			"NATS_URI",
			"PUBLIC_HOST",
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
