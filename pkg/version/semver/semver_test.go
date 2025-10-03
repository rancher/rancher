package semver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testExpectations struct {
	isDevOrPrerelease      bool
	isRC                   bool
	hasReleasePrefix       bool
	hasBranchReleasePrefix bool
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
			false,
		},
	},
	{
		"New Style Alpha",
		"v2.13.3-alpha.1",
		testExpectations{
			true,
			false,
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
			false,
		},
	},
	{
		"RC Build (new)",
		"v2.12.3-rc.1",
		testExpectations{
			true,
			true,
			true,
			false,
		},
	},
	{
		"Stable Build",
		"v2.12.3",
		testExpectations{
			false,
			false,
			true,
			false,
		},
	},
	{
		"Hotfix Build",
		"v2.12.0-hotfix-b112.1",
		testExpectations{
			true,
			false,
			true,
			false,
		},
	},
	{
		"Dev IDE",
		"dev",
		testExpectations{
			true,
			false,
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
			false,
		},
	},
	{
		"Patch with x",
		"v2.7.x",
		testExpectations{
			true,
			false,
			true,
			false,
		},
	},
	{
		"Branch release prefix",
		"v2.x",
		testExpectations{
			true,
			false,
			true,
			true,
		},
	},
	{
		"Branch release prefix w prerelease",
		"v3.x-something",
		testExpectations{
			true,
			false,
			true,
			true,
		},
	},
	{
		"Branch release head",
		"v3.x-head",
		testExpectations{
			true,
			false,
			true,
			true,
		},
	},
}

func Test_IsDevOrPrerelease(t *testing.T) {
	t.Parallel()
	asserts := assert.New(t)

	for _, tt := range exampleRancherVersions {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			testVersion := Version(tt.version)
			asserts.Equal(tt.expectations.isDevOrPrerelease, testVersion.IsDevOrPrerelease())
		})
	}
}

func TestVersion_IsRC(t *testing.T) {
	for _, tt := range exampleRancherVersions {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			version := Version(tt.version)
			assert.Equalf(t, tt.expectations.isRC, version.IsRC(), "Version(%s).IsRC()", tt.version)
		})
	}
}

func TestVersion_HasReleasePrefix(t *testing.T) {
	for _, tt := range exampleRancherVersions {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			version := Version(tt.version)
			assert.Equalf(t, tt.expectations.hasReleasePrefix, version.HasReleasePrefix(), "Version(%s).HasReleasePrefix()", tt.version)
		})
	}
}

func Test_HasBranchReleasePrefix(t *testing.T) {
	for _, tt := range exampleRancherVersions {
		t.Run(tt.name, func(t *testing.T) {
			version := Version(tt.version)
			assert.Equalf(t, tt.expectations.hasBranchReleasePrefix, version.HasBranchReleasePrefix(), "Version(%s).HasBranchReleasePrefix()", tt.version)
		})
	}
}
