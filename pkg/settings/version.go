package settings

import "strings"

// IsRelease returns true if the running server is a released version of rancher.
func IsRelease() bool {
	return !strings.Contains(ServerVersion.Get(), "head") && releasePattern.MatchString(ServerVersion.Get())
}

func IsReleaseServerVersion(serverVersion string) bool {
	if strings.HasPrefix(serverVersion, "dev") ||
		strings.HasPrefix(serverVersion, "master") ||
		serverVersion == "" ||
		strings.HasSuffix(serverVersion, "-head") ||
		strings.HasSuffix(serverVersion, "-main") {
		return false
	}
	return true
}

// GetRancherVersion will return the stored server version without the 'v' prefix.
func GetRancherVersion() string {
	rancherVersion := ServerVersion.Get()
	if !IsReleaseServerVersion(rancherVersion) {
		return RancherVersionDev
	}
	return strings.TrimPrefix(rancherVersion, "v")
}
