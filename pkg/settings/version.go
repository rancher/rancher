package settings

import (
	"strings"

	"github.com/rancher/rancher/pkg/version/semver"
)

// ServerVersionHasReleasePrefixExcludesHead returns true if the running server has the release prefix `v` and doesn't include `head`.
// This function is primarily used by the UI to know when to use local (to rancher image) assets or web assets.
func ServerVersionHasReleasePrefixExcludesHead() bool {
	serverVersion := ServerVersion.Get()
	version := semver.Version(serverVersion)
	return version.HasReleasePrefix() && !strings.Contains(serverVersion, "head")
}

func IsVersionRelease(version string) bool {
	semVer := semver.Version(version)
	return !semVer.IsDevOrPrerelease()
}

// ServerVersionOrFallback verifies the input is a release semver and returns it (without v prefix) or the RancherVersionDev value.
func ServerVersionOrFallback(version string) string {
	if !IsVersionRelease(version) {
		return RancherVersionDev
	}
	return strings.TrimPrefix(version, "v")
}

// GetRancherVersion will return the stored server version without the 'v' prefix (or the fallback value).
func GetRancherVersion() string {
	return ServerVersionOrFallback(ServerVersion.Get())
}
