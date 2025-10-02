package settings

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsRelease(t *testing.T) {
	inputs := map[string]bool{
		"dev":         false,
		"master-head": false,
		"master":      false,
		"v2.5.2":      true,
		"v2":          true,
		"v2.0":        true,
		"v2.x":        true,
		"v2.5-head":   false,
		"2.5":         false,
		"2.5-head":    false,
	}
	a := assert.New(t)
	for key, value := range inputs {
		if err := ServerVersion.Set(key); err != nil {
			t.Errorf("Encountered error while setting temp version: %v\n", err)
		}
		result := IsRelease()
		a.Equal(value, result, fmt.Sprintf("Expected value [%t] for key [%s]. Got value [%t]", value, key, result))
	}
}

func TestGetRancherVersion(t *testing.T) {
	inputs := map[string]string{
		"dev-version":    RancherVersionDev,
		"master-version": RancherVersionDev,
		"version-head":   RancherVersionDev,
		"v2.7.X":         "2.7.X",
		"2.7.X":          "2.7.X",
	}

	for key, value := range inputs {
		err := ServerVersion.Set(key)
		assert.NoError(t, err)
		result := GetRancherVersion()
		assert.Equal(t, value, result)
	}
}

func TestIsReleaseServerVersion(t *testing.T) {
	tests := []struct {
		name          string
		serverVersion string
		want          bool
	}{
		{
			"Normal SemVer",
			"v2.13.99",
			true,
		},
		{
			"Normal SemVer wo v prefix",
			"2.13.99",
			true,
		},
		{
			"Prerelease head",
			"v2.12-head",
			false,
		},
		{
			"Prerelease main",
			"head-main",
			false,
		},
		{
			"Dev build",
			"dev-someGitHash",
			false,
		},
		{
			"Empty version",
			"",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, IsReleaseServerVersion(tt.serverVersion), "IsReleaseServerVersion(%v)", tt.serverVersion)
		})
	}
}
