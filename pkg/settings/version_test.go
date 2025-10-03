package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testExpectations struct {
	serverVersionHasReleasePrefixExcludesHead bool
	getRancherVersion                         string
	isVersionRelease                          bool
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
			RancherVersionDev,
			false,
		},
	},
	{
		"New Style Alpha",
		"v2.13.3-alpha.1",
		testExpectations{
			true,
			RancherVersionDev,
			false,
		},
	},
	{
		"RC Build (old)",
		"v2.12.3-rc1",
		testExpectations{
			true,
			RancherVersionDev,
			false,
		},
	},
	{
		"RC Build (new)",
		"v2.12.3-rc.1",
		testExpectations{
			true,
			RancherVersionDev,
			false,
		},
	},
	{
		"Stable Build",
		"v2.12.3",
		testExpectations{
			true,
			"2.12.3",
			true,
		},
	},
	{
		"Hotfix Build",
		"v2.12.0-hotfix-b112.1",
		testExpectations{
			true,
			RancherVersionDev,
			false,
		},
	},
	{
		"Dev IDE",
		"dev",
		testExpectations{
			false,
			RancherVersionDev,
			false,
		},
	},
	{
		"Dev Build/Head Images",
		"v2.12-207d1eaa2-head",
		testExpectations{
			false,
			RancherVersionDev,
			false,
		},
	},
	{
		"Manual Build Large Patch wo v prefix",
		"2.13.9999",
		testExpectations{
			false,
			"2.13.9999",
			true,
		},
	},
	{
		"Patch with x",
		"v2.7.x",
		testExpectations{
			true,
			RancherVersionDev,
			false,
		},
	},
	{
		"Branch release prefix",
		"v2.x",
		testExpectations{
			true,
			RancherVersionDev,
			false,
		},
	},
	{
		"Branch release prefix w prerelease",
		"v3.x-something",
		testExpectations{
			true,
			RancherVersionDev,
			false,
		},
	},
	{
		"Branch release head",
		"v3.x-head",
		testExpectations{
			false,
			RancherVersionDev,
			false,
		},
	},
}

func Test_ServerVersionHasReleasePrefixExcludesHead(t *testing.T) {
	asserts := assert.New(t)
	for _, tt := range exampleRancherVersions {
		t.Run(tt.name, func(t *testing.T) {
			err := ServerVersion.Set(tt.version)
			asserts.NoError(err)
			result := ServerVersionHasReleasePrefixExcludesHead()
			asserts.Equal(
				tt.expectations.serverVersionHasReleasePrefixExcludesHead,
				result,
				"Expected value [%t] for key [%s]. Got value [%t]",
				tt.expectations.serverVersionHasReleasePrefixExcludesHead, tt.version, result,
			)
		})
	}
}

func Test_GetRancherVersion(t *testing.T) {
	for _, tt := range exampleRancherVersions {
		t.Run(tt.name, func(t *testing.T) {
			err := ServerVersion.Set(tt.version)
			assert.NoError(t, err)
			result := GetRancherVersion()
			assert.Equal(t, tt.expectations.getRancherVersion, result, "Expected value [%s] for key [%s]. Got value [%s]", tt.expectations.getRancherVersion, tt.version, result)
		})
	}
}

func Test_IsVersionRelease(t *testing.T) {
	for _, tt := range exampleRancherVersions {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.expectations.isVersionRelease, IsVersionRelease(tt.version), "IsVersionRelease(%v)", tt.version)
		})
	}
}

func Test_ServerVersionOrFallback(t *testing.T) {
	for _, tt := range exampleRancherVersions {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.expectations.getRancherVersion, ServerVersionOrFallback(tt.version), "ServerVersionOrFallback(%s)", tt.version)
		})
	}
}
