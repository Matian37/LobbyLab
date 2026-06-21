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
		{"GAME_SERVER_CLIENT_PORTS", "xyz:80"},
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
		ClientPorts: internal.NamedPortSet{
			internal.NamedPort{Name: "xyz", Port: network.MustParsePort("80")}: {},
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

func TestParseNamedPorts(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		inputPorts := map[string]string{"a": "80", "b": "400/udp", "c": "8080/tcp"}
		inputSlice := []string{"a:80", "b:400/udp", "c:8080/tcp"}

		ports, err := parseNamedPorts(inputSlice)
		require.NoError(t, err)

		require.Len(t, ports, len(inputSlice))
		for name, p := range inputPorts {
			_, ok := ports[internal.NamedPort{Name: name, Port: network.MustParsePort(p)}]
			assert.True(t, ok, "missing port %v", p)
		}
	})

	t.Run("missing port name", func(t *testing.T) {
		_, err := parseNamedPorts([]string{"abc"})
		assert.ErrorIs(t, err, ErrPortNameMissing)
	})

	t.Run("invalid port", func(t *testing.T) {
		_, err := parseNamedPorts([]string{"game:abc"})
		assert.ErrorIs(t, err, ErrInvalidPortString)
	})

	t.Run("empty", func(t *testing.T) {
		ports, err := parseNamedPorts([]string{})
		require.NoError(t, err)
		assert.Empty(t, ports)
	})
}

func TestIsPortSubset(t *testing.T) {
	tests := []struct {
		name     string
		sub      internal.NamedPortSet
		super    network.PortSet
		expected bool
	}{
		{
			name: "full match",
			sub: internal.NamedPortSet{
				internal.NamedPort{Name: "a", Port: network.MustParsePort("80")}:     {},
				internal.NamedPort{Name: "b", Port: network.MustParsePort("53/udp")}: {},
			},
			super: network.PortSet{
				network.MustParsePort("80"):     {},
				network.MustParsePort("53/udp"): {},
			},
			expected: true,
		},
		{
			name: "partial match",
			sub: internal.NamedPortSet{
				internal.NamedPort{Name: "b", Port: network.MustParsePort("80")}: {},
			},
			super: network.PortSet{
				network.MustParsePort("80"):     {},
				network.MustParsePort("53/udp"): {},
			},
			expected: true,
		},
		{
			name: "sub additional key",
			sub: internal.NamedPortSet{
				internal.NamedPort{Name: "a", Port: network.MustParsePort("80")}:     {},
				internal.NamedPort{Name: "b", Port: network.MustParsePort("53/udp")}: {},
				internal.NamedPort{Name: "c", Port: network.MustParsePort("8080")}:   {},
			},
			super: network.PortSet{
				network.MustParsePort("80"):     {},
				network.MustParsePort("53/udp"): {},
			},
			expected: false,
		},
		{
			name: "empty super",
			sub: internal.NamedPortSet{
				internal.NamedPort{Name: "a", Port: network.MustParsePort("80")}: {},
			},
			super:    network.PortSet{},
			expected: false,
		},
		{
			name:     "empty sub",
			sub:      internal.NamedPortSet{},
			super:    network.PortSet{network.MustParsePort("80"): {}},
			expected: true,
		},
		{
			name:     "sub and super empty",
			sub:      internal.NamedPortSet{},
			super:    network.PortSet{},
			expected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, isSubset(test.sub, test.super))
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
				value:    "abc:xyz",
				err:      ErrInvalidPortString,
			},
			{
				name:     "missing port name",
				override: "GAME_SERVER_CLIENT_PORTS",
				value:    "abc",
				err:      ErrPortNameMissing,
			},
			{
				name:     "client ports not subset of expose ports",
				override: "GAME_SERVER_CLIENT_PORTS",
				value:    "abc:9999",
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
