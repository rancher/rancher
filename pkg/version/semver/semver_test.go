package semver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRancherVersions(t *testing.T) {
	t.Parallel()
	asserts := assert.New(t)

	var tests = []struct {
		name     string
		input    string
		expected bool
	}{
		{
			"Dev IDE",
			"dev",
			true,
		},
		{
			"Dev Build/Head Images",
			"v2.12-207d1eaa2-head",
			true,
		},
		{
			"Alpha Build",
			"v2.12.1-alpha4",
			true,
		},
		{
			"Release",
			"v2.12.1",
			false,
		},
		{
			"Manual Override",
			"2.13.99",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			testVersion := Version(tt.input)
			asserts.Equal(tt.expected, testVersion.IsDev())
		})
	}

}
