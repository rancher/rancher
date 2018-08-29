package settings

import (
	"encoding/json"

	"github.com/rancher/types/apis/management.cattle.io/v3"
)

var (
	settings = map[string]Setting{}
	provider Provider

	AgentImage                      = NewSetting("agent-image", "rancher/rancher-agent:master")
	WindowsAgentImage               = NewSetting("windows-agent-image", "rancher/rancher-agent:master-nanoserver-1803")
	CACerts                         = NewSetting("cacerts", "")
	CLIURLDarwin                    = NewSetting("cli-url-darwin", "https://releases.rancher.com/cli/v1.0.0-alpha8/rancher-darwin-amd64-v1.0.0-alpha8.tar.gz")
	CLIURLLinux                     = NewSetting("cli-url-linux", "https://releases.rancher.com/cli/v1.0.0-alpha8/rancher-linux-amd64-v1.0.0-alpha8.tar.gz")
	CLIURLWindows                   = NewSetting("cli-url-windows", "https://releases.rancher.com/cli/v1.0.0-alpha8/rancher-windows-386-v1.0.0-alpha8.zip")
	ClusterDefaults                 = NewSetting("cluster-defaults", "")
	EngineInstallURL                = NewSetting("engine-install-url", "https://releases.rancher.com/install-docker/17.03.sh")
	EngineISOURL                    = NewSetting("engine-iso-url", "https://releases.rancher.com/os/latest/rancheros-vmware.iso")
	EngineNewestVersion             = NewSetting("engine-newest-version", "v17.12.0")
	EngineSupportedRange            = NewSetting("engine-supported-range", "~v1.11.2 || ~v1.12.0 || ~v1.13.0 || ~v17.03.0")
	FirstLogin                      = NewSetting("first-login", "true")
	HelmVersion                     = NewSetting("helm-version", "dev")
	IngressIPDomain                 = NewSetting("ingress-ip-domain", "xip.io")
	InstallUUID                     = NewSetting("install-uuid", "")
	KubernetesVersion               = NewSetting("k8s-version", v3.DefaultK8s)
	KubernetesVersionToSystemImages = NewSetting("k8s-version-to-images", getSystemImages())
	MachineVersion                  = NewSetting("machine-version", "dev")
	Namespace                       = NewSetting("namespace", "cattle-system")
	PeerServices                    = NewSetting("peer-service", "rancher")
	RDNSServerBaseURL               = NewSetting("rdns-base-url", "https://api.lb.rancher.cloud/v1")
	ServerImage                     = NewSetting("server-image", "rancher/rancher")
	ServerURL                       = NewSetting("server-url", "")
	ServerVersion                   = NewSetting("server-version", "dev")
	SystemDefaultRegistry           = NewSetting("system-default-registry", "")
	SystemNamespaces                = NewSetting("system-namespaces", "kube-system,kube-public,cattle-system,cattle-alerting,cattle-logging,cattle-pipeline,ingress-nginx")
	TelemetryOpt                    = NewSetting("telemetry-opt", "prompt")
	UIFeedBackForm                  = NewSetting("ui-feedback-form", "")
	UIIndex                         = NewSetting("ui-index", "https://releases.rancher.com/ui/latest2/index.html")
	UIPath                          = NewSetting("ui-path", "")
	UIPL                            = NewSetting("ui-pl", "rancher")
	WhitelistDomain                 = NewSetting("whitelist-domain", "forums.rancher.com")
)

type Provider interface {
	Get(name string) string
	Set(name, value string) error
	SetIfUnset(name, value string) error
	SetAll(settings map[string]Setting) error
}

type Setting struct {
	Name     string
	Default  string
	ReadOnly bool
}

func (s Setting) SetIfUnset(value string) error {
	if provider == nil {
		return s.Set(value)
	}
	return provider.SetIfUnset(s.Name, value)
}

func (s Setting) Set(value string) error {
	if provider == nil {
		s, ok := settings[s.Name]
		if ok {
			s.Default = value
			settings[s.Name] = s
		}
	} else {
		return provider.Set(s.Name, value)
	}
	return nil
}

func (s Setting) Get() string {
	if provider == nil {
		s := settings[s.Name]
		return s.Default
	}
	return provider.Get(s.Name)
}

func SetProvider(p Provider) error {
	if err := p.SetAll(settings); err != nil {
		return err
	}
	provider = p
	return nil
}

func NewSetting(name, def string) Setting {
	s := Setting{
		Name:    name,
		Default: def,
	}
	settings[s.Name] = s
	return s
}

func getSystemImages() string {
	newMap := map[string]interface{}{}
	for k := range v3.K8sVersionToRKESystemImages {
		newMap[k] = nil
	}

	data, err := json.Marshal(newMap)
	if err != nil {
		return ""
	}
	return string(data)
}
