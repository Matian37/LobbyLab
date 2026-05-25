package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		fail     bool
		expected []string
	}{
		{name: "too few arguments", args: []string{"x"}, fail: true},
		{name: "too many arguments", args: []string{"x", "./x", "x"}, fail: true},
		{name: "parsing error", args: []string{"/", "/\\"}, fail: true},
		{
			name:     "too few arguments",
			args:     []string{"/", "./x --arg 1 -e 4"},
			fail:     false,
			expected: []string{"./x", "--arg", "1", "-e", "4"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := ParseArgs(test.args)

			if test.fail {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expected, res)
			}
		})
	}
}
