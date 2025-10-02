package settings

import (
	"strings"

	"github.com/rancher/rancher/pkg/version/semver"
)

// IsRelease returns true if the running server is a released version of rancher.
func IsRelease() bool {
	return !strings.Contains(ServerVersion.Get(), "head") && releasePattern.MatchString(ServerVersion.Get())
}

func IsVersionRelease(version string) bool {
	semVer := semver.Version(version)
	return !semVer.IsDev()
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
