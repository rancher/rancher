package systeminfo

import (
	"net/url"

	"github.com/pborman/uuid"
	"github.com/rancher/rancher/pkg/scc/util"
	"github.com/rancher/rancher/pkg/settings"
	coreVersion "github.com/rancher/rancher/pkg/version"
)

const (
	RancherProductIdentifier = "rancher"
	RancherCPUArch           = "unknown"
)

type InfoProvider struct {
	RancherUuid uuid.UUID
	ClusterUuid uuid.UUID
}

func NewInfoProvider(rancherUuid uuid.UUID, clusterUuid uuid.UUID) *InfoProvider {

	return &InfoProvider{
		RancherUuid: rancherUuid,
		ClusterUuid: clusterUuid,
	}
}

// GetVersion returns a version number for the systeminfo provider
func (i *InfoProvider) GetVersion() string {
	var version string
	version = coreVersion.Version
	if util.VersionIsDevBuild() {
		// TODO: maybe SCC devs can give us a static dev version?
		version = "2.10.3"
	}

	return version
}

// GetProductIdentifier returns a triple of product ID, version and CPU architecture
func (i *InfoProvider) GetProductIdentifier() (string, string, string) {
	// Rancher always returns "rancher" as product, and "unknown" as the architecture
	// The CPU architecture must match what SCC has product codes for; unless SCC adds other arches we always return unknown.
	// It is unlikely SCC should add these as that would require customers purchasing different RegCodes to run Rancher on arm64 and amd64.
	// In turn, that would lead to complications like "should Arm run Ranchers allow x86 downstream clusters?"
	return RancherProductIdentifier, i.GetVersion(), RancherCPUArch
}

// ServerUrl returns the Rancher server URL
func ServerUrl() string {
	return settings.ServerURL.Get()
}

// ServerHostname returns the hostname of the Rancher server URL
func ServerHostname() string {
	serverUrl := settings.ServerURL.Get()
	if serverUrl == "" {
		return ""
	}
	parsed, _ := url.Parse(serverUrl)
	return parsed.Host
}

func IsServerUrlReady() bool {
	return ServerHostname() != ""
}
