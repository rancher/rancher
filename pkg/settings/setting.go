package settings

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/rancher/types/apis/management.cattle.io/v3"
)

var (
	settings       = map[string]Setting{}
	provider       Provider
	InjectDefaults string

	AgentImage                      = NewSetting("agent-image", "rancher/rancher-agent:master")
	AuthImage                       = NewSetting("auth-image", "rancher/kube-api-auth:v0.1.1")
	WindowsAgentImage               = NewSetting("windows-agent-image", "rancher/rancher-agent:master-nanoserver-1803")
	CACerts                         = NewSetting("cacerts", "")
	CLIURLDarwin                    = NewSetting("cli-url-darwin", "https://releases.rancher.com/cli/v1.0.0-alpha8/rancher-darwin-amd64-v1.0.0-alpha8.tar.gz")
	CLIURLLinux                     = NewSetting("cli-url-linux", "https://releases.rancher.com/cli/v1.0.0-alpha8/rancher-linux-amd64-v1.0.0-alpha8.tar.gz")
	CLIURLWindows                   = NewSetting("cli-url-windows", "https://releases.rancher.com/cli/v1.0.0-alpha8/rancher-windows-386-v1.0.0-alpha8.zip")
	EngineInstallURL                = NewSetting("engine-install-url", "https://releases.rancher.com/install-docker/18.09.sh")
	EngineISOURL                    = NewSetting("engine-iso-url", "https://releases.rancher.com/os/latest/rancheros-vmware.iso")
	EngineNewestVersion             = NewSetting("engine-newest-version", "v17.12.0")
	EngineSupportedRange            = NewSetting("engine-supported-range", "~v1.11.2 || ~v1.12.0 || ~v1.13.0 || ~v17.03.0 || ~v17.06.0 || ~v17.09.0 || ~v18.06.0 || ~v18.09.0")
	FirstLogin                      = NewSetting("first-login", "true")
	HelmVersion                     = NewSetting("helm-version", "dev")
	IngressIPDomain                 = NewSetting("ingress-ip-domain", "xip.io")
	InstallUUID                     = NewSetting("install-uuid", "")
	KubernetesVersion               = NewSetting("k8s-version", v3.DefaultK8s)
	KubernetesVersionToSystemImages = NewSetting("k8s-version-to-images", getSystemImages())
	MachineVersion                  = NewSetting("machine-version", "dev")
	Namespace                       = NewSetting("namespace", os.Getenv("CATTLE_NAMESPACE"))
	PeerServices                    = NewSetting("peer-service", os.Getenv("CATTLE_PEER_SERVICE"))
	RDNSServerBaseURL               = NewSetting("rdns-base-url", "https://api.lb.rancher.cloud/v1")
	RkeVersion                      = NewSetting("rke-version", "")
	ServerImage                     = NewSetting("server-image", "rancher/rancher")
	ServerURL                       = NewSetting("server-url", "")
	ServerVersion                   = NewSetting("server-version", "dev")
	SystemDefaultRegistry           = NewSetting("system-default-registry", "")
	SystemNamespaces                = NewSetting("system-namespaces", "kube-system,kube-public,cattle-system,cattle-alerting,cattle-logging,cattle-pipeline,cattle-prometheus,ingress-nginx,cattle-global-data")
	TelemetryOpt                    = NewSetting("telemetry-opt", "prompt")
	UIFeedBackForm                  = NewSetting("ui-feedback-form", "")
	UIIndex                         = NewSetting("ui-index", "https://releases.rancher.com/ui/latest2/index.html")
	UIPath                          = NewSetting("ui-path", "")
	UIPL                            = NewSetting("ui-pl", "rancher")
	UIKubernetesSupportedVersions   = NewSetting("ui-k8s-supported-versions-range", ">= 1.11.0 <=1.13.x")
	UIKubernetesDefaultVersion      = NewSetting("ui-k8s-default-version-range", "<=1.13.x")
	WhitelistDomain                 = NewSetting("whitelist-domain", "forums.rancher.com")
	SystemMonitoringCatalogID       = NewSetting("system-monitoring-catalog-id", "catalog://?catalog=system-library&template=rancher-monitoring&version=0.0.2")
	AuthUserInfoResyncCron          = NewSetting("auth-user-info-resync-cron", "0 0 * * *")
	AuthUserInfoMaxAgeSeconds       = NewSetting("auth-user-info-max-age-seconds", "3600") // 1 hour
	APIUIVersion                    = NewSetting("api-ui-version", "1.1.6")                // Please update the CATTLE_API_UI_VERSION in package/Dockerfile when updating the version here.
)

func init() {
	if InjectDefaults == "" {
		return
	}
	defaults := map[string]string{}
	if err := json.Unmarshal([]byte(InjectDefaults), &defaults); err != nil {
		return
	}
	for name, defaultValue := range defaults {
		value, ok := settings[name]
		if !ok {
			continue
		}
		value.Default = defaultValue
		settings[name] = value
	}
}

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

func GetEnvKey(key string) string {
	return "CATTLE_" + strings.ToUpper(strings.Replace(key, "-", "_", -1))
}
