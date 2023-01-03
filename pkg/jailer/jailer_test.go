package jailer

import (
	"os"
	"testing"

	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
)

func Test_WhitelistEnvvars(t *testing.T) {
	settings.WhitelistEnvironmentVars.Set("ENVVAR_FOO")
	defer os.Unsetenv("ENVVAR_FOO")

	type testCase struct {
		input    []string
		expected []string
		envValue string
	}

	testCases := []testCase{
		// Base case, nothing goes in, no env set, nothing comes out
		{
			envValue: "",
			input:    []string{},
			expected: []string{},
		},
		{
			envValue: "BAR",
			input:    []string{},
			expected: []string{"ENVVAR_FOO=BAR"},
		},
		{
			envValue: "ALPHA",
			input:    []string{"BAZ=FOOBAZ"},
			expected: []string{"BAZ=FOOBAZ", "ENVVAR_FOO=ALPHA"},
		},
	}

	for _, tc := range testCases {
		os.Setenv("ENVVAR_FOO", tc.envValue)
		envs := getWhitelistedEnvVars(tc.input)
		assert.Equal(t, envs, tc.expected)
	}

}
