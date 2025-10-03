package semver

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsDev(t *testing.T) {
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

func TestVersion_IsRC(t *testing.T) {
	tests := []struct {
		name string
		v    Version
		want bool
	}{
		{
			"Dev IDE",
			Version("dev"),
			false,
		},
		{
			"Release Version",
			Version("v2.13.99"),
			false,
		},
		{
			"Release Version wo v prefix",
			Version("2.13.99"),
			false,
		},
		{
			"RC version",
			Version("2.13.99-rc.1"),
			true,
		},
		{
			"Non-RC Prerelease version",
			Version("2.13.99-alpha.1"),
			false,
		},
		{
			"Dev Build/Head Images",
			"v2.12-207d1eaa2-head",
			false,
		},
		{
			"Very big RC version",
			Version("2.13.99-rc.92654891"),
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, tt.v.IsRC(), "IsRC()")
		})
	}
}

func TestVersion_HasReleasePrefix(t *testing.T) {
	tests := []struct {
		name string
		v    Version
		want bool
	}{
		{
			"Dev IDE",
			"dev",
			false,
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
			true,
		},
		{
			"Manual Override",
			"2.13.99",
			false,
		},
		{
			"Patch with x",
			"v2.7.x",
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, tt.v.HasReleasePrefix(), "HasReleasePrefix()")
		})
	}
}

func TestHasBranchPrefix(t *testing.T) {
	tests := map[string]bool{
		"":                      false,
		"dev-version":           false,
		"master-version":        false,
		"version-head":          false,
		"v2.12-dev-someGitHash": false,
		"v2.7.X":                false,
		"2.7.X":                 false,
		"v2.7.0":                false,
		"2.7.0":                 false,
		"v3.x-something":        true,
		"v2.x":                  true,
	}

	for input, expected := range tests {
		t.Run(fmt.Sprintf("%s => %s", input, strconv.FormatBool(expected)), func(t *testing.T) {
			version := Version(input)
			assert.Equalf(t, expected, version.HasBranchPrefix(), "Version(%s).HasBranchPrefix()", input)
		})
	}
}
