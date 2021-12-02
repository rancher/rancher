package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	authsettings "github.com/rancher/rancher/pkg/auth/settings"
	fleetconst "github.com/rancher/rancher/pkg/fleet"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

var (
	releasePattern = regexp.MustCompile("^v[0-9]")
	settings       = map[string]Setting{}
	provider       Provider
	InjectDefaults string

	AgentImage                        = NewSetting("agent-image", "rancher/rancher-agent:master-head")
	AgentRolloutTimeout               = NewSetting("agent-rollout-timeout", "300s")
	AgentRolloutWait                  = NewSetting("agent-rollout-wait", "true")
	AuthImage                         = NewSetting("auth-image", v32.ToolsSystemImages.AuthSystemImages.KubeAPIAuth)
	AuthTokenMaxTTLMinutes            = NewSetting("auth-token-max-ttl-minutes", "0") // never expire
	AuthorizationCacheTTLSeconds      = NewSetting("authorization-cache-ttl-seconds", "10")
	AuthorizationDenyCacheTTLSeconds  = NewSetting("authorization-deny-cache-ttl-seconds", "10")
	AzureGroupCacheSize               = NewSetting("azure-group-cache-size", "10000")
	CACerts                           = NewSetting("cacerts", "")
	CLIURLDarwin                      = NewSetting("cli-url-darwin", "https://releases.rancher.com/cli/v1.0.0-alpha8/rancher-darwin-amd64-v1.0.0-alpha8.tar.gz")
	CLIURLLinux                       = NewSetting("cli-url-linux", "https://releases.rancher.com/cli/v1.0.0-alpha8/rancher-linux-amd64-v1.0.0-alpha8.tar.gz")
	CLIURLWindows                     = NewSetting("cli-url-windows", "https://releases.rancher.com/cli/v1.0.0-alpha8/rancher-windows-386-v1.0.0-alpha8.zip")
	ClusterControllerStartCount       = NewSetting("cluster-controller-start-count", "50")
	EngineInstallURL                  = NewSetting("engine-install-url", "https://releases.rancher.com/install-docker/20.10.sh")
	EngineISOURL                      = NewSetting("engine-iso-url", "https://releases.rancher.com/os/latest/rancheros-vmware.iso")
	EngineNewestVersion               = NewSetting("engine-newest-version", "v17.12.0")
	EngineSupportedRange              = NewSetting("engine-supported-range", "~v1.11.2 || ~v1.12.0 || ~v1.13.0 || ~v17.03.0 || ~v17.06.0 || ~v17.09.0 || ~v18.06.0 || ~v18.09.0 || ~v19.03.0 || ~v20.10.0 ")
	FirstLogin                        = NewSetting("first-login", "true")
	GlobalRegistryEnabled             = NewSetting("global-registry-enabled", "false")
	GithubProxyAPIURL                 = NewSetting("github-proxy-api-url", "https://api.github.com")
	HelmVersion                       = NewSetting("helm-version", "dev")
	HelmMaxHistory                    = NewSetting("helm-max-history", "10")
	IngressIPDomain                   = NewSetting("ingress-ip-domain", "sslip.io")
	InstallUUID                       = NewSetting("install-uuid", "")
	InternalServerURL                 = NewSetting("internal-server-url", "")
	InternalCACerts                   = NewSetting("internal-cacerts", "")
	IsRKE                             = NewSetting("is-rke", "")
	JailerTimeout                     = NewSetting("jailer-timeout", "60")
	KubeconfigGenerateToken           = NewSetting("kubeconfig-generate-token", "true")
	KubeconfigTokenTTLMinutes         = NewSetting("kubeconfig-token-ttl-minutes", "960") // 16 hours
	KubernetesVersion                 = NewSetting("k8s-version", "")
	KubernetesVersionToServiceOptions = NewSetting("k8s-version-to-service-options", "")
	KubernetesVersionToSystemImages   = NewSetting("k8s-version-to-images", "")
	KubernetesVersionsCurrent         = NewSetting("k8s-versions-current", "")
	KubernetesVersionsDeprecated      = NewSetting("k8s-versions-deprecated", "")
	KDMBranch                         = NewSetting("kdm-branch", "dev-v2.6-1.22")
	MachineVersion                    = NewSetting("machine-version", "dev")
	Namespace                         = NewSetting("namespace", os.Getenv("CATTLE_NAMESPACE"))
	PasswordMinLength                 = NewSetting("password-min-length", "12")
	PeerServices                      = NewSetting("peer-service", os.Getenv("CATTLE_PEER_SERVICE"))
	RDNSServerBaseURL                 = NewSetting("rdns-base-url", "https://api.lb.rancher.cloud/v1")
	RkeVersion                        = NewSetting("rke-version", "")
	RkeMetadataConfig                 = NewSetting("rke-metadata-config", getMetadataConfig())
	ServerImage                       = NewSetting("server-image", "rancher/rancher")
	ServerURL                         = NewSetting("server-url", "")
	ServerVersion                     = NewSetting("server-version", "dev")
	SystemAgentVersion                = NewSetting("system-agent-version", "")
	WinsAgentVersion                  = NewSetting("wins-agent-version", "")
	SystemAgentInstallScript          = NewSetting("system-agent-install-script", "https://raw.githubusercontent.com/rancher/system-agent/main/install.sh")
	WindowsRke2InstallScript          = NewSetting("windows-rke2-install-script", "https://raw.githubusercontent.com/rancher/wins/main/install.ps1")
	SystemAgentInstallerImage         = NewSetting("system-agent-installer-image", "rancher/system-agent-installer-")
	SystemAgentUpgradeImage           = NewSetting("system-agent-upgrade-image", "")
	SystemDefaultRegistry             = NewSetting("system-default-registry", "")
	SystemNamespaces                  = NewSetting("system-namespaces", "kube-system,kube-public,cattle-system,cattle-alerting,cattle-logging,cattle-pipeline,cattle-prometheus,ingress-nginx,cattle-global-data,cattle-istio,kube-node-lease,cert-manager,cattle-global-nt,security-scan,cattle-fleet-system,cattle-fleet-local-system,calico-system,tigera-operator,cattle-impersonation-system,rancher-operator-system")
	TelemetryOpt                      = NewSetting("telemetry-opt", "")
	TLSMinVersion                     = NewSetting("tls-min-version", "1.2")
	TLSCiphers                        = NewSetting("tls-ciphers", "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305")
	UIBanners                         = NewSetting("ui-banners", "{}")
	UIBrand                           = NewSetting("ui-brand", "")
	UIDefaultLanding                  = NewSetting("ui-default-landing", "vue")
	UIFeedBackForm                    = NewSetting("ui-feedback-form", "")
	UIIndex                           = NewSetting("ui-index", "https://releases.rancher.com/ui/latest2/index.html")
	UIPath                            = NewSetting("ui-path", "/usr/share/rancher/ui")
	UIDashboardIndex                  = NewSetting("ui-dashboard-index", "https://releases.rancher.com/dashboard/latest/index.html")
	UIDashboardPath                   = NewSetting("ui-dashboard-path", "/usr/share/rancher/ui-dashboard")
	UIPreferred                       = NewSetting("ui-preferred", "vue")
	UIOfflinePreferred                = NewSetting("ui-offline-preferred", "dynamic")
	UIIssues                          = NewSetting("ui-issues", "")
	UIPL                              = NewSetting("ui-pl", "rancher")
	UICommunityLinks                  = NewSetting("ui-community-links", "true")
	UIKubernetesSupportedVersions     = NewSetting("ui-k8s-supported-versions-range", ">= 1.11.0 <=1.14.x")
	UIKubernetesDefaultVersion        = NewSetting("ui-k8s-default-version-range", "<=1.14.x")
	WhitelistDomain                   = NewSetting("whitelist-domain", "forums.rancher.com")
	WhitelistEnvironmentVars          = NewSetting("whitelist-envvars", "HTTP_PROXY,HTTPS_PROXY,NO_PROXY")
	AuthUserInfoResyncCron            = NewSetting("auth-user-info-resync-cron", "0 0 * * *")
	AuthUserSessionTTLMinutes         = NewSetting("auth-user-session-ttl-minutes", "960")   // 16 hours
	AuthUserInfoMaxAgeSeconds         = NewSetting("auth-user-info-max-age-seconds", "3600") // 1 hour
	APIUIVersion                      = NewSetting("api-ui-version", "1.1.6")                // Please update the CATTLE_API_UI_VERSION in package/Dockerfile when updating the version here.
	RotateCertsIfExpiringInDays       = NewSetting("rotate-certs-if-expiring-in-days", "7")  // 7 days
	ClusterTemplateEnforcement        = NewSetting("cluster-template-enforcement", "false")
	InitialDockerRootDir              = NewSetting("initial-docker-root-dir", "/var/lib/docker")
	SystemCatalog                     = NewSetting("system-catalog", "external") // Options are 'external' or 'bundled'
	ChartDefaultBranch                = NewSetting("chart-default-branch", "dev-v2.6")
	PartnerChartDefaultBranch         = NewSetting("partner-chart-default-branch", "main")
	RKE2ChartDefaultBranch            = NewSetting("rke2-chart-default-branch", "main")
	FleetDefaultWorkspaceName         = NewSetting("fleet-default-workspace-name", fleetconst.ClustersDefaultNamespace) // fleetWorkspaceName to assign to clusters with none
	ShellImage                        = NewSetting("shell-image", "rancher/shell:v0.1.14")
	IgnoreNodeName                    = NewSetting("ignore-node-name", "") // nodes to ignore when syncing v1.node to v3.node
	NoDefaultAdmin                    = NewSetting("no-default-admin", "")
	RestrictedDefaultAdmin            = NewSetting("restricted-default-admin", "false") // When bootstrapping the admin for the first time, give them the global role restricted-admin
	AKSUpstreamRefresh                = NewSetting("aks-refresh", "300")
	EKSUpstreamRefreshCron            = NewSetting("eks-refresh-cron", "*/5 * * * *") // EKSUpstreamRefreshCron is deprecated and will be replaced by EKSUpstreamRefresh
	EKSUpstreamRefresh                = NewSetting("eks-refresh", "300")
	GKEUpstreamRefresh                = NewSetting("gke-refresh", "300")
	HideLocalCluster                  = NewSetting("hide-local-cluster", "false")
	MachineProvisionImage             = NewSetting("machine-provision-image", "rancher/machine:v0.15.0-rancher73")
	SystemFeatureChartRefreshSeconds  = NewSetting("system-feature-chart-refresh-seconds", "900")

	FleetMinVersion          = NewSetting("fleet-min-version", "")
	RancherWebhookMinVersion = NewSetting("rancher-webhook-min-version", "")
)

func FullShellImage() string {
	return PrefixPrivateRegistry(ShellImage.Get())
}

func PrefixPrivateRegistry(image string) string {
	private := SystemDefaultRegistry.Get()
	if private == "" {
		return image
	}
	return private + "/" + image
}

func IsRelease() bool {
	return !strings.Contains(ServerVersion.Get(), "head") && releasePattern.MatchString(ServerVersion.Get())
}

func init() {
	// setup auth setting
	authsettings.AuthUserInfoResyncCron = AuthUserInfoResyncCron
	authsettings.AuthUserSessionTTLMinutes = AuthUserSessionTTLMinutes
	authsettings.AuthUserInfoMaxAgeSeconds = AuthUserInfoMaxAgeSeconds
	authsettings.FirstLogin = FirstLogin

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

func (s Setting) GetInt() int {
	v := s.Get()
	i, err := strconv.Atoi(v)
	if err == nil {
		return i
	}
	logrus.Errorf("failed to parse setting %s=%s as int: %v", s.Name, v, err)
	i, err = strconv.Atoi(s.Default)
	if err != nil {
		return 0
	}
	return i
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

func GetEnvKey(key string) string {
	return "CATTLE_" + strings.ToUpper(strings.Replace(key, "-", "_", -1))
}

func getMetadataConfig() string {
	branch := KDMBranch.Get()
	data := map[string]interface{}{
		"url":                      fmt.Sprintf("https://releases.rancher.com/kontainer-driver-metadata/%s/data.json", branch),
		"refresh-interval-minutes": "1440",
	}
	ans, err := json.Marshal(data)
	if err != nil {
		logrus.Errorf("error getting metadata config %v", err)
		return ""
	}
	return string(ans)
}

// GetSettingByID returns a setting that is stored with the given id
func GetSettingByID(id string) string {
	if provider == nil {
		s := settings[id]
		return s.Default
	}
	return provider.Get(id)
}

func DefaultAgentSettings() []Setting {
	return []Setting{
		ServerVersion,
		InstallUUID,
		IngressIPDomain,
	}
}

func DefaultAgentSettingsAsEnvVars() []v1.EnvVar {
	defaultAgentSettings := DefaultAgentSettings()
	envVars := make([]v1.EnvVar, 0, len(defaultAgentSettings))

	for _, s := range defaultAgentSettings {
		envVars = append(envVars, v1.EnvVar{
			Name:  GetEnvKey(s.Name),
			Value: s.Get(),
		})
	}

	return envVars
}
