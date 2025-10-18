package semver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testExpectations struct {
	isDevOrPrerelease bool
	isRC              bool
	hasReleasePrefix  bool
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
			false,
			true,
		},
	},
	{
		"New Style Alpha",
		"v2.13.3-alpha.1",
		testExpectations{
			true,
			false,
			true,
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
			false,
			false,
			true,
		},
	},
	{
		"Hotfix Build",
		"v2.12.0-hotfix-b112.1",
		testExpectations{
			true,
			false,
			true,
		},
	},
	{
		"Dev IDE",
		"dev",
		testExpectations{
			true,
			false,
			false,
		},
	},
	{
		"Dev Build/Head Images",
		"v2.12-207d1eaa2-head",
		testExpectations{
			true,
			false,
			true,
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
