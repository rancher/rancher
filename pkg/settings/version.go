package settings

import "strings"

// IsRelease returns true if the running server is a released version of rancher.
func IsRelease() bool {
	return !strings.Contains(ServerVersion.Get(), "head") && releasePattern.MatchString(ServerVersion.Get())
}

func IsVersionRelease(version string) bool {
	if strings.HasPrefix(version, "dev") ||
		strings.HasPrefix(version, "master") ||
		version == "" ||
		strings.HasSuffix(version, "-head") ||
		strings.HasSuffix(version, "-main") {
		return false
	}
	return true
}

// GetRancherVersion will return the stored server version without the 'v' prefix.
func GetRancherVersion() string {
	rancherVersion := ServerVersion.Get()
	if !IsVersionRelease(rancherVersion) {
		return RancherVersionDev
	}
	return strings.TrimPrefix(rancherVersion, "v")
}
