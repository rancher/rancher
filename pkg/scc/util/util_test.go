package util

import (
	coreVersion "github.com/rancher/rancher/pkg/version"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestVersionIsDevBuild(t *testing.T) {
	coreVersion.Version = "dev"
	assert.True(t, VersionIsDevBuild())

	coreVersion.Version = "2.13.2"
	assert.False(t, VersionIsDevBuild())
	coreVersion.Version = "v2.13.2"
	assert.False(t, VersionIsDevBuild())

	coreVersion.Version = "v2.13.2+meta"
	assert.False(t, VersionIsDevBuild())

	coreVersion.Version = "v2.13.2-rc.42"
	assert.True(t, VersionIsDevBuild())

	coreVersion.Version = "v2.13.2+meta-with-hyphen"
	assert.False(t, VersionIsDevBuild())

	coreVersion.Version = "v2.13.2-rc.9999+meta-also"
	assert.True(t, VersionIsDevBuild())
}
