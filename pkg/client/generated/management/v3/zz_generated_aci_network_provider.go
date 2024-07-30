package client

const (
	AciNetworkProviderType                                      = "aciNetworkProvider"
	AciNetworkProviderFieldAEP                                  = "aep"
	AciNetworkProviderFieldAciContainersControllerMemoryLimit   = "aciContainersControllerMemoryLimit"
	AciNetworkProviderFieldAciContainersControllerMemoryRequest = "aciContainersControllerMemoryRequest"
	AciNetworkProviderFieldAciContainersHostMemoryLimit         = "aciContainersHostMemoryLimit"
	AciNetworkProviderFieldAciContainersHostMemoryRequest       = "aciContainersHostMemoryRequest"
	AciNetworkProviderFieldAciContainersMemoryLimit             = "aciContainersMemoryLimit"
	AciNetworkProviderFieldAciContainersMemoryRequest           = "aciContainersMemoryRequest"
	AciNetworkProviderFieldAciMultipod                          = "aciMultipod"
	AciNetworkProviderFieldAciMultipodUbuntu                    = "aciMultipodUbuntu"
	AciNetworkProviderFieldAddExternalContractToDefaultEpg      = "addExternalContractToDefaultEpg"
	AciNetworkProviderFieldAddExternalSubnetsToRdconfig         = "addExternalSubnetsToRdconfig"
	AciNetworkProviderFieldApicConnectionRetryLimit             = "apicConnectionRetryLimit"
	AciNetworkProviderFieldApicHosts                            = "apicHosts"
	AciNetworkProviderFieldApicRefreshTickerAdjust              = "apicRefreshTickerAdjust"
	AciNetworkProviderFieldApicRefreshTime                      = "apicRefreshTime"
	AciNetworkProviderFieldApicSubscriptionDelay                = "apicSubscriptionDelay"
	AciNetworkProviderFieldApicUserCrt                          = "apicUserCrt"
	AciNetworkProviderFieldApicUserKey                          = "apicUserKey"
	AciNetworkProviderFieldApicUserName                         = "apicUserName"
	AciNetworkProviderFieldCApic                                = "capic"
	AciNetworkProviderFieldControllerLogLevel                   = "controllerLogLevel"
	AciNetworkProviderFieldDhcpDelay                            = "dhcpDelay"
	AciNetworkProviderFieldDhcpRenewMaxRetryCount               = "dhcpRenewMaxRetryCount"
	AciNetworkProviderFieldDisableHppRendering                  = "disableHppRendering"
	AciNetworkProviderFieldDisablePeriodicSnatGlobalInfoSync    = "disablePeriodicSnatGlobalInfoSync"
	AciNetworkProviderFieldDisableWaitForNetwork                = "disableWaitForNetwork"
	AciNetworkProviderFieldDropLogDisableEvents                 = "dropLogDisableEvents"
	AciNetworkProviderFieldDropLogEnable                        = "dropLogEnable"
	AciNetworkProviderFieldDurationWaitForNetwork               = "durationWaitForNetwork"
	AciNetworkProviderFieldDynamicExternalSubnet                = "externDynamic"
	AciNetworkProviderFieldEnableEndpointSlice                  = "enableEndpointSlice"
	AciNetworkProviderFieldEnableOpflexAgentReconnect           = "enableOpflexAgentReconnect"
	AciNetworkProviderFieldEncapType                            = "encapType"
	AciNetworkProviderFieldEpRegistry                           = "epRegistry"
	AciNetworkProviderFieldGbpPodSubnet                         = "gbpPodSubnet"
	AciNetworkProviderFieldHostAgentLogLevel                    = "hostAgentLogLevel"
	AciNetworkProviderFieldHppOptimization                      = "hppOptimization"
	AciNetworkProviderFieldImagePullPolicy                      = "imagePullPolicy"
	AciNetworkProviderFieldImagePullSecret                      = "imagePullSecret"
	AciNetworkProviderFieldInfraVlan                            = "infraVlan"
	AciNetworkProviderFieldInstallIstio                         = "installIstio"
	AciNetworkProviderFieldIstioProfile                         = "istioProfile"
	AciNetworkProviderFieldKafkaBrokers                         = "kafkaBrokers"
	AciNetworkProviderFieldKafkaClientCrt                       = "kafkaClientCrt"
	AciNetworkProviderFieldKafkaClientKey                       = "kafkaClientKey"
	AciNetworkProviderFieldKubeAPIVlan                          = "kubeApiVlan"
	AciNetworkProviderFieldL3Out                                = "l3out"
	AciNetworkProviderFieldL3OutExternalNetworks                = "l3outExternalNetworks"
	AciNetworkProviderFieldMTUHeadRoom                          = "mtuHeadRoom"
	AciNetworkProviderFieldMaxNodesSvcGraph                     = "maxNodesSvcGraph"
	AciNetworkProviderFieldMcastDaemonMemoryLimit               = "mcastDaemonMemoryLimit"
	AciNetworkProviderFieldMcastDaemonMemoryRequest             = "mcastDaemonMemoryRequest"
	AciNetworkProviderFieldMcastRangeEnd                        = "mcastRangeEnd"
	AciNetworkProviderFieldMcastRangeStart                      = "mcastRangeStart"
	AciNetworkProviderFieldMultusDisable                        = "multusDisable"
	AciNetworkProviderFieldNoPriorityClass                      = "noPriorityClass"
	AciNetworkProviderFieldNoWaitForServiceEpReadiness          = "noWaitForServiceEpReadiness"
	AciNetworkProviderFieldNodePodIfEnable                      = "nodePodIfEnable"
	AciNetworkProviderFieldNodeSnatRedirectExclude              = "nodeSnatRedirectExclude"
	AciNetworkProviderFieldNodeSubnet                           = "nodeSubnet"
	AciNetworkProviderFieldOVSMemoryLimit                       = "ovsMemoryLimit"
	AciNetworkProviderFieldOVSMemoryRequest                     = "ovsMemoryRequest"
	AciNetworkProviderFieldOpflexAgentLogLevel                  = "opflexLogLevel"
	AciNetworkProviderFieldOpflexAgentMemoryLimit               = "opflexAgentMemoryLimit"
	AciNetworkProviderFieldOpflexAgentMemoryRequest             = "opflexAgentMemoryRequest"
	AciNetworkProviderFieldOpflexAgentOpflexAsyncjsonEnabled    = "opflexAgentOpflexAsyncjsonEnabled"
	AciNetworkProviderFieldOpflexAgentOvsAsyncjsonEnabled       = "opflexAgentOvsAsyncjsonEnabled"
	AciNetworkProviderFieldOpflexAgentPolicyRetryDelayTimer     = "opflexAgentPolicyRetryDelayTimer"
	AciNetworkProviderFieldOpflexAgentStatistics                = "opflexAgentStatistics"
	AciNetworkProviderFieldOpflexClientSSL                      = "opflexClientSsl"
	AciNetworkProviderFieldOpflexDeviceDeleteTimeout            = "opflexDeviceDeleteTimeout"
	AciNetworkProviderFieldOpflexDeviceReconnectWaitTimeout     = "opflexDeviceReconnectWaitTimeout"
	AciNetworkProviderFieldOpflexMode                           = "opflexMode"
	AciNetworkProviderFieldOpflexOpensslCompat                  = "opflexOpensslCompat"
	AciNetworkProviderFieldOpflexServerPort                     = "opflexServerPort"
	AciNetworkProviderFieldOpflexStartupEnabled                 = "opflexStartupEnabled"
	AciNetworkProviderFieldOpflexStartupPolicyDuration          = "opflexStartupPolicyDuration"
	AciNetworkProviderFieldOpflexStartupResolveAftConn          = "opflexStartupResolveAftConn"
	AciNetworkProviderFieldOpflexSwitchSyncDelay                = "opflexSwitchSyncDelay"
	AciNetworkProviderFieldOpflexSwitchSyncDynamic              = "opflexSwitchSyncDynamic"
	AciNetworkProviderFieldOverlayVRFName                       = "overlayVrfName"
	AciNetworkProviderFieldPBRTrackingNonSnat                   = "pbrTrackingNonSnat"
	AciNetworkProviderFieldPodSubnetChunkSize                   = "podSubnetChunkSize"
	AciNetworkProviderFieldRunGbpContainer                      = "runGbpContainer"
	AciNetworkProviderFieldRunOpflexServerContainer             = "runOpflexServerContainer"
	AciNetworkProviderFieldServiceGraphEndpointAddDelay         = "serviceGraphEndpointAddDelay"
	AciNetworkProviderFieldServiceGraphEndpointAddServices      = "serviceGraphEndpointAddServices"
	AciNetworkProviderFieldServiceGraphSubnet                   = "nodeSvcSubnet"
	AciNetworkProviderFieldServiceMonitorInterval               = "serviceMonitorInterval"
	AciNetworkProviderFieldServiceVlan                          = "serviceVlan"
	AciNetworkProviderFieldSleepTimeSnatGlobalInfoSync          = "sleepTimeSnatGlobalInfoSync"
	AciNetworkProviderFieldSnatContractScope                    = "snatContractScope"
	AciNetworkProviderFieldSnatNamespace                        = "snatNamespace"
	AciNetworkProviderFieldSnatPortRangeEnd                     = "snatPortRangeEnd"
	AciNetworkProviderFieldSnatPortRangeStart                   = "snatPortRangeStart"
	AciNetworkProviderFieldSnatPortsPerNode                     = "snatPortsPerNode"
	AciNetworkProviderFieldSriovEnable                          = "sriovEnable"
	AciNetworkProviderFieldStaticExternalSubnet                 = "externStatic"
	AciNetworkProviderFieldSubnetDomainName                     = "subnetDomainName"
	AciNetworkProviderFieldSystemIdentifier                     = "systemId"
	AciNetworkProviderFieldTaintNotReadyNode                    = "taintNotReadyNode"
	AciNetworkProviderFieldTenant                               = "tenant"
	AciNetworkProviderFieldToken                                = "token"
	AciNetworkProviderFieldTolerationSeconds                    = "tolerationSeconds"
	AciNetworkProviderFieldUseAciAnywhereCRD                    = "useAciAnywhereCrd"
	AciNetworkProviderFieldUseAciCniPriorityClass               = "useAciCniPriorityClass"
	AciNetworkProviderFieldUseClusterRole                       = "useClusterRole"
	AciNetworkProviderFieldUseHostNetnsVolume                   = "useHostNetnsVolume"
	AciNetworkProviderFieldUseOpflexServerVolume                = "useOpflexServerVolume"
	AciNetworkProviderFieldUsePrivilegedContainer               = "usePrivilegedContainer"
	AciNetworkProviderFieldUseSystemNodePriorityClass           = "useSystemNodePriorityClass"
	AciNetworkProviderFieldVRFName                              = "vrfName"
	AciNetworkProviderFieldVRFTenant                            = "vrfTenant"
	AciNetworkProviderFieldVmmController                        = "vmmController"
	AciNetworkProviderFieldVmmDomain                            = "vmmDomain"
)

type AciNetworkProvider struct {
	AEP                                  string              `json:"aep,omitempty" yaml:"aep,omitempty"`
	AciContainersControllerMemoryLimit   string              `json:"aciContainersControllerMemoryLimit,omitempty" yaml:"aciContainersControllerMemoryLimit,omitempty"`
	AciContainersControllerMemoryRequest string              `json:"aciContainersControllerMemoryRequest,omitempty" yaml:"aciContainersControllerMemoryRequest,omitempty"`
	AciContainersHostMemoryLimit         string              `json:"aciContainersHostMemoryLimit,omitempty" yaml:"aciContainersHostMemoryLimit,omitempty"`
	AciContainersHostMemoryRequest       string              `json:"aciContainersHostMemoryRequest,omitempty" yaml:"aciContainersHostMemoryRequest,omitempty"`
	AciContainersMemoryLimit             string              `json:"aciContainersMemoryLimit,omitempty" yaml:"aciContainersMemoryLimit,omitempty"`
	AciContainersMemoryRequest           string              `json:"aciContainersMemoryRequest,omitempty" yaml:"aciContainersMemoryRequest,omitempty"`
	AciMultipod                          string              `json:"aciMultipod,omitempty" yaml:"aciMultipod,omitempty"`
	AciMultipodUbuntu                    string              `json:"aciMultipodUbuntu,omitempty" yaml:"aciMultipodUbuntu,omitempty"`
	AddExternalContractToDefaultEpg      string              `json:"addExternalContractToDefaultEpg,omitempty" yaml:"addExternalContractToDefaultEpg,omitempty"`
	AddExternalSubnetsToRdconfig         string              `json:"addExternalSubnetsToRdconfig,omitempty" yaml:"addExternalSubnetsToRdconfig,omitempty"`
	ApicConnectionRetryLimit             string              `json:"apicConnectionRetryLimit,omitempty" yaml:"apicConnectionRetryLimit,omitempty"`
	ApicHosts                            []string            `json:"apicHosts,omitempty" yaml:"apicHosts,omitempty"`
	ApicRefreshTickerAdjust              string              `json:"apicRefreshTickerAdjust,omitempty" yaml:"apicRefreshTickerAdjust,omitempty"`
	ApicRefreshTime                      string              `json:"apicRefreshTime,omitempty" yaml:"apicRefreshTime,omitempty"`
	ApicSubscriptionDelay                string              `json:"apicSubscriptionDelay,omitempty" yaml:"apicSubscriptionDelay,omitempty"`
	ApicUserCrt                          string              `json:"apicUserCrt,omitempty" yaml:"apicUserCrt,omitempty"`
	ApicUserKey                          string              `json:"apicUserKey,omitempty" yaml:"apicUserKey,omitempty"`
	ApicUserName                         string              `json:"apicUserName,omitempty" yaml:"apicUserName,omitempty"`
	CApic                                string              `json:"capic,omitempty" yaml:"capic,omitempty"`
	ControllerLogLevel                   string              `json:"controllerLogLevel,omitempty" yaml:"controllerLogLevel,omitempty"`
	DhcpDelay                            string              `json:"dhcpDelay,omitempty" yaml:"dhcpDelay,omitempty"`
	DhcpRenewMaxRetryCount               string              `json:"dhcpRenewMaxRetryCount,omitempty" yaml:"dhcpRenewMaxRetryCount,omitempty"`
	DisableHppRendering                  string              `json:"disableHppRendering,omitempty" yaml:"disableHppRendering,omitempty"`
	DisablePeriodicSnatGlobalInfoSync    string              `json:"disablePeriodicSnatGlobalInfoSync,omitempty" yaml:"disablePeriodicSnatGlobalInfoSync,omitempty"`
	DisableWaitForNetwork                string              `json:"disableWaitForNetwork,omitempty" yaml:"disableWaitForNetwork,omitempty"`
	DropLogDisableEvents                 string              `json:"dropLogDisableEvents,omitempty" yaml:"dropLogDisableEvents,omitempty"`
	DropLogEnable                        string              `json:"dropLogEnable,omitempty" yaml:"dropLogEnable,omitempty"`
	DurationWaitForNetwork               string              `json:"durationWaitForNetwork,omitempty" yaml:"durationWaitForNetwork,omitempty"`
	DynamicExternalSubnet                string              `json:"externDynamic,omitempty" yaml:"externDynamic,omitempty"`
	EnableEndpointSlice                  string              `json:"enableEndpointSlice,omitempty" yaml:"enableEndpointSlice,omitempty"`
	EnableOpflexAgentReconnect           string              `json:"enableOpflexAgentReconnect,omitempty" yaml:"enableOpflexAgentReconnect,omitempty"`
	EncapType                            string              `json:"encapType,omitempty" yaml:"encapType,omitempty"`
	EpRegistry                           string              `json:"epRegistry,omitempty" yaml:"epRegistry,omitempty"`
	GbpPodSubnet                         string              `json:"gbpPodSubnet,omitempty" yaml:"gbpPodSubnet,omitempty"`
	HostAgentLogLevel                    string              `json:"hostAgentLogLevel,omitempty" yaml:"hostAgentLogLevel,omitempty"`
	HppOptimization                      string              `json:"hppOptimization,omitempty" yaml:"hppOptimization,omitempty"`
	ImagePullPolicy                      string              `json:"imagePullPolicy,omitempty" yaml:"imagePullPolicy,omitempty"`
	ImagePullSecret                      string              `json:"imagePullSecret,omitempty" yaml:"imagePullSecret,omitempty"`
	InfraVlan                            string              `json:"infraVlan,omitempty" yaml:"infraVlan,omitempty"`
	InstallIstio                         string              `json:"installIstio,omitempty" yaml:"installIstio,omitempty"`
	IstioProfile                         string              `json:"istioProfile,omitempty" yaml:"istioProfile,omitempty"`
	KafkaBrokers                         []string            `json:"kafkaBrokers,omitempty" yaml:"kafkaBrokers,omitempty"`
	KafkaClientCrt                       string              `json:"kafkaClientCrt,omitempty" yaml:"kafkaClientCrt,omitempty"`
	KafkaClientKey                       string              `json:"kafkaClientKey,omitempty" yaml:"kafkaClientKey,omitempty"`
	KubeAPIVlan                          string              `json:"kubeApiVlan,omitempty" yaml:"kubeApiVlan,omitempty"`
	L3Out                                string              `json:"l3out,omitempty" yaml:"l3out,omitempty"`
	L3OutExternalNetworks                []string            `json:"l3outExternalNetworks,omitempty" yaml:"l3outExternalNetworks,omitempty"`
	MTUHeadRoom                          string              `json:"mtuHeadRoom,omitempty" yaml:"mtuHeadRoom,omitempty"`
	MaxNodesSvcGraph                     string              `json:"maxNodesSvcGraph,omitempty" yaml:"maxNodesSvcGraph,omitempty"`
	McastDaemonMemoryLimit               string              `json:"mcastDaemonMemoryLimit,omitempty" yaml:"mcastDaemonMemoryLimit,omitempty"`
	McastDaemonMemoryRequest             string              `json:"mcastDaemonMemoryRequest,omitempty" yaml:"mcastDaemonMemoryRequest,omitempty"`
	McastRangeEnd                        string              `json:"mcastRangeEnd,omitempty" yaml:"mcastRangeEnd,omitempty"`
	McastRangeStart                      string              `json:"mcastRangeStart,omitempty" yaml:"mcastRangeStart,omitempty"`
	MultusDisable                        string              `json:"multusDisable,omitempty" yaml:"multusDisable,omitempty"`
	NoPriorityClass                      string              `json:"noPriorityClass,omitempty" yaml:"noPriorityClass,omitempty"`
	NoWaitForServiceEpReadiness          string              `json:"noWaitForServiceEpReadiness,omitempty" yaml:"noWaitForServiceEpReadiness,omitempty"`
	NodePodIfEnable                      string              `json:"nodePodIfEnable,omitempty" yaml:"nodePodIfEnable,omitempty"`
	NodeSnatRedirectExclude              []map[string]string `json:"nodeSnatRedirectExclude,omitempty" yaml:"nodeSnatRedirectExclude,omitempty"`
	NodeSubnet                           string              `json:"nodeSubnet,omitempty" yaml:"nodeSubnet,omitempty"`
	OVSMemoryLimit                       string              `json:"ovsMemoryLimit,omitempty" yaml:"ovsMemoryLimit,omitempty"`
	OVSMemoryRequest                     string              `json:"ovsMemoryRequest,omitempty" yaml:"ovsMemoryRequest,omitempty"`
	OpflexAgentLogLevel                  string              `json:"opflexLogLevel,omitempty" yaml:"opflexLogLevel,omitempty"`
	OpflexAgentMemoryLimit               string              `json:"opflexAgentMemoryLimit,omitempty" yaml:"opflexAgentMemoryLimit,omitempty"`
	OpflexAgentMemoryRequest             string              `json:"opflexAgentMemoryRequest,omitempty" yaml:"opflexAgentMemoryRequest,omitempty"`
	OpflexAgentOpflexAsyncjsonEnabled    string              `json:"opflexAgentOpflexAsyncjsonEnabled,omitempty" yaml:"opflexAgentOpflexAsyncjsonEnabled,omitempty"`
	OpflexAgentOvsAsyncjsonEnabled       string              `json:"opflexAgentOvsAsyncjsonEnabled,omitempty" yaml:"opflexAgentOvsAsyncjsonEnabled,omitempty"`
	OpflexAgentPolicyRetryDelayTimer     string              `json:"opflexAgentPolicyRetryDelayTimer,omitempty" yaml:"opflexAgentPolicyRetryDelayTimer,omitempty"`
	OpflexAgentStatistics                string              `json:"opflexAgentStatistics,omitempty" yaml:"opflexAgentStatistics,omitempty"`
	OpflexClientSSL                      string              `json:"opflexClientSsl,omitempty" yaml:"opflexClientSsl,omitempty"`
	OpflexDeviceDeleteTimeout            string              `json:"opflexDeviceDeleteTimeout,omitempty" yaml:"opflexDeviceDeleteTimeout,omitempty"`
	OpflexDeviceReconnectWaitTimeout     string              `json:"opflexDeviceReconnectWaitTimeout,omitempty" yaml:"opflexDeviceReconnectWaitTimeout,omitempty"`
	OpflexMode                           string              `json:"opflexMode,omitempty" yaml:"opflexMode,omitempty"`
	OpflexOpensslCompat                  string              `json:"opflexOpensslCompat,omitempty" yaml:"opflexOpensslCompat,omitempty"`
	OpflexServerPort                     string              `json:"opflexServerPort,omitempty" yaml:"opflexServerPort,omitempty"`
	OpflexStartupEnabled                 string              `json:"opflexStartupEnabled,omitempty" yaml:"opflexStartupEnabled,omitempty"`
	OpflexStartupPolicyDuration          string              `json:"opflexStartupPolicyDuration,omitempty" yaml:"opflexStartupPolicyDuration,omitempty"`
	OpflexStartupResolveAftConn          string              `json:"opflexStartupResolveAftConn,omitempty" yaml:"opflexStartupResolveAftConn,omitempty"`
	OpflexSwitchSyncDelay                string              `json:"opflexSwitchSyncDelay,omitempty" yaml:"opflexSwitchSyncDelay,omitempty"`
	OpflexSwitchSyncDynamic              string              `json:"opflexSwitchSyncDynamic,omitempty" yaml:"opflexSwitchSyncDynamic,omitempty"`
	OverlayVRFName                       string              `json:"overlayVrfName,omitempty" yaml:"overlayVrfName,omitempty"`
	PBRTrackingNonSnat                   string              `json:"pbrTrackingNonSnat,omitempty" yaml:"pbrTrackingNonSnat,omitempty"`
	PodSubnetChunkSize                   string              `json:"podSubnetChunkSize,omitempty" yaml:"podSubnetChunkSize,omitempty"`
	RunGbpContainer                      string              `json:"runGbpContainer,omitempty" yaml:"runGbpContainer,omitempty"`
	RunOpflexServerContainer             string              `json:"runOpflexServerContainer,omitempty" yaml:"runOpflexServerContainer,omitempty"`
	ServiceGraphEndpointAddDelay         string              `json:"serviceGraphEndpointAddDelay,omitempty" yaml:"serviceGraphEndpointAddDelay,omitempty"`
	ServiceGraphEndpointAddServices      []map[string]string `json:"serviceGraphEndpointAddServices,omitempty" yaml:"serviceGraphEndpointAddServices,omitempty"`
	ServiceGraphSubnet                   string              `json:"nodeSvcSubnet,omitempty" yaml:"nodeSvcSubnet,omitempty"`
	ServiceMonitorInterval               string              `json:"serviceMonitorInterval,omitempty" yaml:"serviceMonitorInterval,omitempty"`
	ServiceVlan                          string              `json:"serviceVlan,omitempty" yaml:"serviceVlan,omitempty"`
	SleepTimeSnatGlobalInfoSync          string              `json:"sleepTimeSnatGlobalInfoSync,omitempty" yaml:"sleepTimeSnatGlobalInfoSync,omitempty"`
	SnatContractScope                    string              `json:"snatContractScope,omitempty" yaml:"snatContractScope,omitempty"`
	SnatNamespace                        string              `json:"snatNamespace,omitempty" yaml:"snatNamespace,omitempty"`
	SnatPortRangeEnd                     string              `json:"snatPortRangeEnd,omitempty" yaml:"snatPortRangeEnd,omitempty"`
	SnatPortRangeStart                   string              `json:"snatPortRangeStart,omitempty" yaml:"snatPortRangeStart,omitempty"`
	SnatPortsPerNode                     string              `json:"snatPortsPerNode,omitempty" yaml:"snatPortsPerNode,omitempty"`
	SriovEnable                          string              `json:"sriovEnable,omitempty" yaml:"sriovEnable,omitempty"`
	StaticExternalSubnet                 string              `json:"externStatic,omitempty" yaml:"externStatic,omitempty"`
	SubnetDomainName                     string              `json:"subnetDomainName,omitempty" yaml:"subnetDomainName,omitempty"`
	SystemIdentifier                     string              `json:"systemId,omitempty" yaml:"systemId,omitempty"`
	TaintNotReadyNode                    string              `json:"taintNotReadyNode,omitempty" yaml:"taintNotReadyNode,omitempty"`
	Tenant                               string              `json:"tenant,omitempty" yaml:"tenant,omitempty"`
	Token                                string              `json:"token,omitempty" yaml:"token,omitempty"`
	TolerationSeconds                    string              `json:"tolerationSeconds,omitempty" yaml:"tolerationSeconds,omitempty"`
	UseAciAnywhereCRD                    string              `json:"useAciAnywhereCrd,omitempty" yaml:"useAciAnywhereCrd,omitempty"`
	UseAciCniPriorityClass               string              `json:"useAciCniPriorityClass,omitempty" yaml:"useAciCniPriorityClass,omitempty"`
	UseClusterRole                       string              `json:"useClusterRole,omitempty" yaml:"useClusterRole,omitempty"`
	UseHostNetnsVolume                   string              `json:"useHostNetnsVolume,omitempty" yaml:"useHostNetnsVolume,omitempty"`
	UseOpflexServerVolume                string              `json:"useOpflexServerVolume,omitempty" yaml:"useOpflexServerVolume,omitempty"`
	UsePrivilegedContainer               string              `json:"usePrivilegedContainer,omitempty" yaml:"usePrivilegedContainer,omitempty"`
	UseSystemNodePriorityClass           string              `json:"useSystemNodePriorityClass,omitempty" yaml:"useSystemNodePriorityClass,omitempty"`
	VRFName                              string              `json:"vrfName,omitempty" yaml:"vrfName,omitempty"`
	VRFTenant                            string              `json:"vrfTenant,omitempty" yaml:"vrfTenant,omitempty"`
	VmmController                        string              `json:"vmmController,omitempty" yaml:"vmmController,omitempty"`
	VmmDomain                            string              `json:"vmmDomain,omitempty" yaml:"vmmDomain,omitempty"`
}
