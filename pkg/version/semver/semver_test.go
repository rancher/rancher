package semver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testExpectations struct {
	hasReleasePrefix  bool
	isDevOrPrerelease bool
	isRC              bool
}

type genericRancherExampleCases struct {
	name         string
	version      string
	expectations testExpectations
}

var exampleRancherVersions = []genericRancherExampleCases{
	{
		"Current Alpha",
		"v2.12.3-alpha1",
		testExpectations{
			true,
			true,
			false,
		},
	},
	{
		"New Style Alpha",
		"v2.13.3-alpha.1",
		testExpectations{
			true,
			true,
			false,
		},
	},
	{
		"RC Build (old)",
		"v2.12.3-rc1",
		testExpectations{
			true,
			true,
			true,
		},
	},
	{
		"RC Build (new)",
		"v2.12.3-rc.1",
		testExpectations{
			true,
			true,
			true,
		},
	},
	{
		"Stable Build",
		"v2.12.3",
		testExpectations{
			true,
			false,
			false,
		},
	},
	{
		"Hotfix Build",
		"v2.12.0-hotfix-b112.1",
		testExpectations{
			true,
			true,
			false,
		},
	},
	{
		"Dev IDE",
		"dev",
		testExpectations{
			false,
			true,
			false,
		},
	},
	{
		"Dev Build/Head Images",
		"v2.12-207d1eaa2-head",
		testExpectations{
			true,
			true,
			false,
		},
	},
	{
		"Manual Build Large Patch wo v prefix",
		"2.13.9999",
		testExpectations{
			false,
			false,
			false,
		},
	},
}

func TestVersion_HasReleasePrefix(t *testing.T) {
	t.Parallel()
	for _, tt := range exampleRancherVersions {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			version := Version(tt.version)
			assert.Equalf(t, tt.expectations.hasReleasePrefix, version.HasReleasePrefix(), "Version(%s).HasReleasePrefix()", tt.version)
		})
	}
}

func Test_IsDevOrPrerelease(t *testing.T) {
	t.Parallel()
	for _, tt := range exampleRancherVersions {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			testVersion := Version(tt.version)
			assert.Equal(t, tt.expectations.isDevOrPrerelease, testVersion.IsDevOrPrerelease())
		})
	}
}

func TestVersion_IsRC(t *testing.T) {
	t.Parallel()
	for _, tt := range exampleRancherVersions {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			version := Version(tt.version)
			assert.Equalf(t, tt.expectations.isRC, version.IsRC(), "Version(%s).IsRC()", tt.version)
		})
	}
}
