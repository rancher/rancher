package clusters

import (
	"fmt"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	baseline         = "baseline"
	externalAws      = "external-aws"
	etcdRole         = "etcd-role"
	controlPlaneRole = "control-plane-role"
	workerRole       = "worker-role"

	externalCloudProviderString = "cloud-provider=external"
	kubeletArgKey               = "kubelet-arg"
	kubeletAPIServerArgKey      = "kubeapi-server-arg"
	kubeControllerManagerArgKey = "kube-controller-manager-arg"
	cloudProviderAnnotationName = "cloud-provider-name"
	disableCloudController      = "disable-cloud-controller"
	protectKernelDefaults       = "protect-kernel-defaults"
	localcluster                = "fleet-local/local"
	rancherRestricted           = "rancher-restricted"
	rke1HardenedGID             = 52034
	rke1HardenedUID             = 52034
)

// CreateRancherBaselinePSACT creates custom PSACT called rancher-baseline which sets each PSS to baseline.
func CreateRancherBaselinePSACT(client *rancher.Client, psact string) error {
	_, err := client.Steve.SteveType(clusters.PodSecurityAdmissionSteveResoureType).ByID(psact)
	if err == nil {
		return err
	}

	template := &v3.PodSecurityAdmissionConfigurationTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: psact,
		},
		Description: "This is a custom baseline Pod Security Admission Configuration Template. " +
			"It defines a minimally restrictive policy which prevents known privilege escalations. " +
			"This policy contains namespace level exemptions for Rancher components.",
		Configuration: v3.PodSecurityAdmissionConfigurationTemplateSpec{
			Defaults: v3.PodSecurityAdmissionConfigurationTemplateDefaults{
				Enforce: baseline,
				Audit:   baseline,
				Warn:    baseline,
			},
			Exemptions: v3.PodSecurityAdmissionConfigurationTemplateExemptions{
				Usernames:      []string{},
				RuntimeClasses: []string{},
				Namespaces: []string{
					"ingress-nginx",
					"kube-system",
					"cattle-system",
					"cattle-epinio-system",
					"cattle-fleet-system",
					"longhorn-system",
					"cattle-neuvector-system",
					"cattle-monitoring-system",
					"rancher-alerting-drivers",
					"cis-operator-system",
					"cattle-csp-adapter-system",
					"cattle-externalip-system",
					"cattle-gatekeeper-system",
					"istio-system",
					"cattle-istio-system",
					"cattle-logging-system",
					"cattle-windows-gmsa-system",
					"cattle-sriov-system",
					"cattle-ui-plugin-system",
					"tigera-operator",
				},
			},
		},
	}

	_, err = client.Steve.SteveType(clusters.PodSecurityAdmissionSteveResoureType).Create(template)
	if err != nil {
		return err
	}

	return nil
}

// NewRKE1ClusterConfig is a constructor for a v3.Cluster object, to be used by the rancher.Client.Provisioning client.
func NewRKE1ClusterConfig(clusterName string, client *rancher.Client, clustersConfig *ClusterConfig) *management.Cluster {
	backupConfigEnabled := true
	criDockerBool := false
	if clustersConfig.CRIDockerd {
		criDockerBool = true
	}
	newConfig := &management.Cluster{
		DockerRootDir: "/var/lib/docker",
		LocalClusterAuthEndpoint: &management.LocalClusterAuthEndpoint{
			Enabled: true,
		},
		Name: clusterName,
		RancherKubernetesEngineConfig: &management.RancherKubernetesEngineConfig{
			DNS: &management.DNSConfig{
				Provider: "coredns",
				Options: map[string]string{
					"stubDomains": "cluster.local",
				},
			},
			EnableCRIDockerd: &criDockerBool,
			Ingress: &management.IngressConfig{
				Provider: "nginx",
			},
			Monitoring: &management.MonitoringConfig{
				Provider: "metrics-server",
			},
			Network: &management.NetworkConfig{
				Plugin:  clustersConfig.CNI,
				MTU:     0,
				Options: map[string]string{},
			},
			Services: &management.RKEConfigServices{
				Etcd: &management.ETCDService{
					BackupConfig: &management.BackupConfig{
						Enabled:       &backupConfigEnabled,
						IntervalHours: 12,
						Retention:     6,
						SafeTimestamp: true,
						Timeout:       120,
					},
				},
			},
			Version: clustersConfig.KubernetesVersion,
		},
	}
	newConfig.ClusterAgentDeploymentCustomization = clustersConfig.ClusterAgent
	newConfig.FleetAgentDeploymentCustomization = clustersConfig.FleetAgent
	newConfig.AgentEnvVars = clustersConfig.AgentEnvVarsRKE1

	if clustersConfig.Registries != nil {
		if clustersConfig.Registries.RKE1Registries != nil {
			newConfig.RancherKubernetesEngineConfig.PrivateRegistries = clustersConfig.Registries.RKE1Registries
			for _, registry := range clustersConfig.Registries.RKE1Registries {
				if registry.ECRCredentialPlugin != nil {
					awsAccessKeyID := fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", registry.ECRCredentialPlugin.AwsAccessKeyID)
					awsSecretAccessKey := fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", registry.ECRCredentialPlugin.AwsSecretAccessKey)
					extraEnv := []string{awsAccessKeyID, awsSecretAccessKey}
					newConfig.RancherKubernetesEngineConfig.Services = &management.RKEConfigServices{
						Kubelet: &management.KubeletService{
							ExtraEnv: extraEnv,
						},
					}
					break
				}
			}
		}
	}

	if clustersConfig.CloudProvider != "" {
		newConfig.RancherKubernetesEngineConfig.CloudProvider = &management.CloudProvider{
			Name: clustersConfig.CloudProvider,
		}
		if clustersConfig.CloudProvider == externalAws {
			trueBoolean := true
			newConfig.RancherKubernetesEngineConfig.CloudProvider.UseInstanceMetadataHostname = &trueBoolean
		}
	}

	if clustersConfig.ETCDRKE1 != nil {
		newConfig.RancherKubernetesEngineConfig.Services.Etcd = clustersConfig.ETCDRKE1
	}

	if clustersConfig.PSACT != "" {
		newConfig.DefaultPodSecurityAdmissionConfigurationTemplateName = clustersConfig.PSACT
	}

	if clustersConfig.AgentEnvVars != nil {
		newConfig.AgentEnvVars = clustersConfig.AgentEnvVarsRKE1
	}

	return newConfig
}

// UpdateRKE1ClusterConfig is a constructor for a v3.Cluster object, to be used by the rancher.Client.Provisioning client.
func UpdateRKE1ClusterConfig(clusterName string, client *rancher.Client, clustersConfig *ClusterConfig) *management.Cluster {
	newConfig := &management.Cluster{
		Name: clusterName,
		RancherKubernetesEngineConfig: &management.RancherKubernetesEngineConfig{
			Network: &management.NetworkConfig{
				Plugin: clustersConfig.CNI,
			},
			Version: clustersConfig.KubernetesVersion,
		},
	}

	newConfig.ClusterAgentDeploymentCustomization = clustersConfig.ClusterAgent
	newConfig.FleetAgentDeploymentCustomization = clustersConfig.FleetAgent
	newConfig.AgentEnvVars = clustersConfig.AgentEnvVarsRKE1

	if clustersConfig.Registries != nil {
		if clustersConfig.Registries.RKE1Registries != nil {
			newConfig.RancherKubernetesEngineConfig.PrivateRegistries = clustersConfig.Registries.RKE1Registries
			for _, registry := range clustersConfig.Registries.RKE1Registries {
				if registry.ECRCredentialPlugin != nil {
					awsAccessKeyID := fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", registry.ECRCredentialPlugin.AwsAccessKeyID)
					awsSecretAccessKey := fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", registry.ECRCredentialPlugin.AwsSecretAccessKey)
					extraEnv := []string{awsAccessKeyID, awsSecretAccessKey}
					newConfig.RancherKubernetesEngineConfig.Services = &management.RKEConfigServices{
						Kubelet: &management.KubeletService{
							ExtraEnv: extraEnv,
						},
					}

					break
				}
			}
		}
	}

	if clustersConfig.CloudProvider != "" {
		newConfig.RancherKubernetesEngineConfig.CloudProvider = &management.CloudProvider{
			Name: clustersConfig.CloudProvider,
		}
		if clustersConfig.CloudProvider == externalAws {
			trueBoolean := true
			newConfig.RancherKubernetesEngineConfig.CloudProvider.UseInstanceMetadataHostname = &trueBoolean
		}
	}

	if clustersConfig.ETCDRKE1 != nil {
		newConfig.RancherKubernetesEngineConfig.Services.Etcd = clustersConfig.ETCDRKE1
	}

	if clustersConfig.AgentEnvVars != nil {
		newConfig.AgentEnvVars = clustersConfig.AgentEnvVarsRKE1
	}

	if clustersConfig.PSACT != "" {
		newConfig.DefaultPodSecurityAdmissionConfigurationTemplateName = clustersConfig.PSACT
	}

	return newConfig
}

// NewK3SRKE2ClusterConfig is a constructor for a apisV1.Cluster object, to be used by the rancher.Client.Provisioning client.
func NewK3SRKE2ClusterConfig(clusterName, namespace string, clustersConfig *ClusterConfig, machinePools []apisV1.RKEMachinePool, cloudCredentialSecretName string) *apisV1.Cluster {
	typeMeta := metav1.TypeMeta{
		Kind:       "Cluster",
		APIVersion: "provisioning.cattle.io/v1",
	}

	//metav1.ObjectMeta
	objectMeta := metav1.ObjectMeta{
		Name:      clusterName,
		Namespace: namespace,
	}

	etcd := &rkev1.ETCD{
		SnapshotRetention:    5,
		SnapshotScheduleCron: "0 */5 * * *",
	}
	if clustersConfig.ETCD != nil {
		etcd = clustersConfig.ETCD
	}

	chartValuesMap := rkev1.GenericMap{
		Data: map[string]interface{}{},
	}
	chartAdditionalManifest := ""
	if clustersConfig.AddOnConfig != nil {
		if clustersConfig.AddOnConfig.ChartValues != nil {
			chartValuesMap = *clustersConfig.AddOnConfig.ChartValues
		}
		chartAdditionalManifest = clustersConfig.AddOnConfig.AdditionalManifest
	}

	machineGlobalConfigMap := rkev1.GenericMap{
		Data: map[string]interface{}{
			"cni":                 clustersConfig.CNI,
			"disable-kube-proxy":  false,
			"etcd-expose-metrics": false,
			"profile":             nil,
		},
	}
	machineSelectorConfigs := []rkev1.RKESystemConfig{}
	if clustersConfig.Advanced != nil {
		if clustersConfig.Advanced.MachineGlobalConfig != nil {
			for k, v := range clustersConfig.Advanced.MachineGlobalConfig.Data {
				machineGlobalConfigMap.Data[k] = v
			}
		}

		if clustersConfig.Advanced.MachineSelectors != nil {
			machineSelectorConfigs = *clustersConfig.Advanced.MachineSelectors
		}
	}

	localClusterAuthEndpoint := rkev1.LocalClusterAuthEndpoint{
		CACerts: "",
		Enabled: false,
		FQDN:    "",
	}
	if clustersConfig.Networking != nil {
		if clustersConfig.Networking.LocalClusterAuthEndpoint != nil {
			localClusterAuthEndpoint = *clustersConfig.Networking.LocalClusterAuthEndpoint
		}
	}

	upgradeStrategy := rkev1.ClusterUpgradeStrategy{
		ControlPlaneConcurrency:  "10%",
		ControlPlaneDrainOptions: rkev1.DrainOptions{},
		WorkerConcurrency:        "10%",
		WorkerDrainOptions:       rkev1.DrainOptions{},
	}
	if clustersConfig.UpgradeStrategy != nil {
		upgradeStrategy = *clustersConfig.UpgradeStrategy
	}

	clusterAgentDeploymentCustomization := &apisV1.AgentDeploymentCustomization{}
	if clustersConfig.ClusterAgent != nil {
		clusterAgentOverrides := ResourceConfigHelper(clustersConfig.ClusterAgent.OverrideResourceRequirements)
		clusterAgentDeploymentCustomization.OverrideResourceRequirements = clusterAgentOverrides
		v1ClusterTolerations := []corev1.Toleration{}
		for _, t := range clustersConfig.ClusterAgent.AppendTolerations {
			v1ClusterTolerations = append(v1ClusterTolerations, corev1.Toleration{
				Key:      t.Key,
				Operator: corev1.TolerationOperator(t.Operator),
				Value:    t.Value,
				Effect:   corev1.TaintEffect(t.Effect),
			})
		}
		clusterAgentDeploymentCustomization.AppendTolerations = v1ClusterTolerations
		clusterAgentDeploymentCustomization.OverrideAffinity = AgentAffinityConfigHelper(clustersConfig.ClusterAgent.OverrideAffinity)
	}

	fleetAgentDeploymentCustomization := &apisV1.AgentDeploymentCustomization{}
	if clustersConfig.FleetAgent != nil {
		fleetAgentOverrides := ResourceConfigHelper(clustersConfig.FleetAgent.OverrideResourceRequirements)
		fleetAgentDeploymentCustomization.OverrideResourceRequirements = fleetAgentOverrides
		v1FleetTolerations := []corev1.Toleration{}
		for _, t := range clustersConfig.FleetAgent.AppendTolerations {
			v1FleetTolerations = append(v1FleetTolerations, corev1.Toleration{
				Key:      t.Key,
				Operator: corev1.TolerationOperator(t.Operator),
				Value:    t.Value,
				Effect:   corev1.TaintEffect(t.Effect),
			})
		}
		fleetAgentDeploymentCustomization.AppendTolerations = v1FleetTolerations
		fleetAgentDeploymentCustomization.OverrideAffinity = AgentAffinityConfigHelper(clustersConfig.FleetAgent.OverrideAffinity)
	}
	var registries *rkev1.Registry
	if clustersConfig.Registries != nil {
		registries = clustersConfig.Registries.RKE2Registries
	}

	if clustersConfig.CloudProvider == provisioninginput.AWSProviderName.String() {
		machineSelectorConfigs = append(machineSelectorConfigs, OutOfTreeSystemConfig(clustersConfig.CloudProvider)...)
	} else if strings.Contains(clustersConfig.CloudProvider, "-in-tree") {
		machineSelectorConfigs = append(machineSelectorConfigs, InTreeSystemConfig(strings.Split(clustersConfig.CloudProvider, "-in-tree")[0])...)
	}

	if clustersConfig.CloudProvider == provisioninginput.VsphereCloudProviderName.String() {
		machineSelectorConfigs = append(machineSelectorConfigs,
			RKESystemConfigTemplate(map[string]interface{}{
				cloudProviderAnnotationName: provisioninginput.VsphereCloudProviderName.String(),
				protectKernelDefaults:       false,
			},
				nil),
		)
	}

	rkeSpecCommon := rkev1.RKEClusterSpecCommon{
		UpgradeStrategy:       upgradeStrategy,
		ChartValues:           chartValuesMap,
		MachineGlobalConfig:   machineGlobalConfigMap,
		MachineSelectorConfig: machineSelectorConfigs,
		AdditionalManifest:    chartAdditionalManifest,
		Registries:            registries,
		ETCD:                  etcd,
	}
	rkeConfig := &apisV1.RKEConfig{
		RKEClusterSpecCommon: rkeSpecCommon,
		MachinePools:         machinePools,
	}

	agentEnvVars := []rkev1.EnvVar{}

	spec := apisV1.ClusterSpec{
		CloudCredentialSecretName:           cloudCredentialSecretName,
		KubernetesVersion:                   clustersConfig.KubernetesVersion,
		LocalClusterAuthEndpoint:            localClusterAuthEndpoint,
		RKEConfig:                           rkeConfig,
		ClusterAgentDeploymentCustomization: clusterAgentDeploymentCustomization,
		FleetAgentDeploymentCustomization:   fleetAgentDeploymentCustomization,
		AgentEnvVars:                        agentEnvVars,
	}

	if clustersConfig.AgentEnvVars != nil {
		spec.AgentEnvVars = clustersConfig.AgentEnvVars
	}

	if clustersConfig.PSACT != "" {
		spec.DefaultPodSecurityAdmissionConfigurationTemplateName = clustersConfig.PSACT
	}

	v1Cluster := &apisV1.Cluster{
		TypeMeta:   typeMeta,
		ObjectMeta: objectMeta,
		Spec:       spec,
	}

	return v1Cluster
}

// UpdateK3SRKE2ClusterConfig is a constructor for a apisV1.Cluster object, to be used by the rancher.Client.Provisioning client.
func UpdateK3SRKE2ClusterConfig(cluster *v1.SteveAPIObject, clustersConfig *ClusterConfig) *v1.SteveAPIObject {
	clusterSpec := &provv1.ClusterSpec{}
	err := steveV1.ConvertToK8sType(cluster.Spec, clusterSpec)
	if err != nil {
		return nil
	}

	if clustersConfig.ETCD != nil {
		clusterSpec.RKEConfig.ETCD = clustersConfig.ETCD
	}

	if clustersConfig.KubernetesVersion != "" {
		clusterSpec.KubernetesVersion = clustersConfig.KubernetesVersion
	}

	if clustersConfig.CNI != "" {
		clusterSpec.RKEConfig.MachineGlobalConfig.Data["cni"] = clustersConfig.CNI
	}

	if clustersConfig.AddOnConfig != nil {
		if clustersConfig.AddOnConfig.ChartValues != nil {
			clusterSpec.RKEConfig.ChartValues = *clustersConfig.AddOnConfig.ChartValues
		}
		clusterSpec.RKEConfig.AdditionalManifest = clustersConfig.AddOnConfig.AdditionalManifest
	}

	if clustersConfig.Advanced != nil {
		if clustersConfig.Advanced.MachineGlobalConfig != nil {
			for k, v := range clustersConfig.Advanced.MachineGlobalConfig.Data {
				clusterSpec.RKEConfig.MachineGlobalConfig.Data[k] = v
			}
		}

		if clustersConfig.Advanced.MachineSelectors != nil {
			clusterSpec.RKEConfig.MachineSelectorConfig = *clustersConfig.Advanced.MachineSelectors
		}
	}

	if clustersConfig.Networking != nil {
		if clustersConfig.Networking.LocalClusterAuthEndpoint != nil {
			clusterSpec.LocalClusterAuthEndpoint = *clustersConfig.Networking.LocalClusterAuthEndpoint
		}
	}

	if clustersConfig.UpgradeStrategy != nil {
		clusterSpec.RKEConfig.UpgradeStrategy = *clustersConfig.UpgradeStrategy
	}

	if clustersConfig.ClusterAgent != nil {
		clusterAgentOverrides := ResourceConfigHelper(clustersConfig.ClusterAgent.OverrideResourceRequirements)
		clusterSpec.ClusterAgentDeploymentCustomization.OverrideResourceRequirements = clusterAgentOverrides
		v1ClusterTolerations := []corev1.Toleration{}
		for _, t := range clustersConfig.ClusterAgent.AppendTolerations {
			v1ClusterTolerations = append(v1ClusterTolerations, corev1.Toleration{
				Key:      t.Key,
				Operator: corev1.TolerationOperator(t.Operator),
				Value:    t.Value,
				Effect:   corev1.TaintEffect(t.Effect),
			})
		}
		clusterSpec.ClusterAgentDeploymentCustomization.AppendTolerations = v1ClusterTolerations
		clusterSpec.ClusterAgentDeploymentCustomization.OverrideAffinity = AgentAffinityConfigHelper(clustersConfig.ClusterAgent.OverrideAffinity)
	}

	if clustersConfig.FleetAgent != nil {
		fleetAgentOverrides := ResourceConfigHelper(clustersConfig.FleetAgent.OverrideResourceRequirements)
		clusterSpec.ClusterAgentDeploymentCustomization.OverrideResourceRequirements = fleetAgentOverrides
		v1FleetTolerations := []corev1.Toleration{}
		for _, t := range clustersConfig.FleetAgent.AppendTolerations {
			v1FleetTolerations = append(v1FleetTolerations, corev1.Toleration{
				Key:      t.Key,
				Operator: corev1.TolerationOperator(t.Operator),
				Value:    t.Value,
				Effect:   corev1.TaintEffect(t.Effect),
			})
		}
		clusterSpec.FleetAgentDeploymentCustomization.AppendTolerations = v1FleetTolerations
		clusterSpec.FleetAgentDeploymentCustomization.OverrideAffinity = AgentAffinityConfigHelper(clustersConfig.FleetAgent.OverrideAffinity)
	}

	if clustersConfig.Registries != nil {
		clusterSpec.RKEConfig.Registries = clustersConfig.Registries.RKE2Registries
	}

	if clustersConfig.CloudProvider == provisioninginput.AWSProviderName.String() {
		clusterSpec.RKEConfig.MachineSelectorConfig = append(clusterSpec.RKEConfig.MachineSelectorConfig, OutOfTreeSystemConfig(clustersConfig.CloudProvider)...)
	} else if strings.Contains(clustersConfig.CloudProvider, "-in-tree") {
		clusterSpec.RKEConfig.MachineSelectorConfig = append(clusterSpec.RKEConfig.MachineSelectorConfig, InTreeSystemConfig(strings.Split(clustersConfig.CloudProvider, "-in-tree")[0])...)
	}

	if clustersConfig.CloudProvider == provisioninginput.VsphereCloudProviderName.String() {
		clusterSpec.RKEConfig.MachineSelectorConfig = append(clusterSpec.RKEConfig.MachineSelectorConfig,
			RKESystemConfigTemplate(map[string]interface{}{
				cloudProviderAnnotationName: provisioninginput.VsphereCloudProviderName.String(),
				protectKernelDefaults:       false,
			},
				nil),
		)
	}

	if clustersConfig.AgentEnvVars != nil {
		clusterSpec.AgentEnvVars = clustersConfig.AgentEnvVars
	}

	if clustersConfig.PSACT != "" {
		clusterSpec.DefaultPodSecurityAdmissionConfigurationTemplateName = clustersConfig.PSACT
	}

	cluster.Spec = clusterSpec

	return cluster
}

// ResourceConfigHelper is a "helper" function that is used to convert the management.ResourceRequirements struct
// to a corev1.ResourceRequirements struct.
func ResourceConfigHelper(advancedClusterResourceRequirements *management.ResourceRequirements) *corev1.ResourceRequirements {
	agentOverrides := corev1.ResourceRequirements{}
	agentOverrides.Limits = corev1.ResourceList{}
	agentOverrides.Requests = corev1.ResourceList{}
	if advancedClusterResourceRequirements.Limits[string(corev1.ResourceCPU)] != "" {
		agentOverrides.Limits[corev1.ResourceCPU] = resource.MustParse(advancedClusterResourceRequirements.Limits[string(corev1.ResourceCPU)])
	}
	if advancedClusterResourceRequirements.Limits[string(corev1.ResourceMemory)] != "" {
		agentOverrides.Limits[corev1.ResourceMemory] = resource.MustParse(advancedClusterResourceRequirements.Limits[string(corev1.ResourceMemory)])
	}
	if advancedClusterResourceRequirements.Requests[string(corev1.ResourceCPU)] != "" {
		agentOverrides.Requests[corev1.ResourceCPU] = resource.MustParse(advancedClusterResourceRequirements.Requests[string(corev1.ResourceCPU)])
	}
	if advancedClusterResourceRequirements.Requests[string(corev1.ResourceMemory)] != "" {
		agentOverrides.Requests[corev1.ResourceMemory] = resource.MustParse(advancedClusterResourceRequirements.Requests[string(corev1.ResourceMemory)])
	}
	return &agentOverrides
}

// AgentAffinityConfigHelper is a "helper" function that converts a management.Affinity struct and returns a corev1.Affinity struct.
func AgentAffinityConfigHelper(advancedClusterAffinity *management.Affinity) *corev1.Affinity {
	agentAffinity := &corev1.Affinity{}
	if advancedClusterAffinity.NodeAffinity != nil {
		agentAffinity.NodeAffinity = &corev1.NodeAffinity{}
		if advancedClusterAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
			agentAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{}
			agentAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = []corev1.NodeSelectorTerm{}
			for _, term := range advancedClusterAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
				agentMatchExpressions := []corev1.NodeSelectorRequirement{}
				if term.MatchExpressions != nil {
					for _, match := range term.MatchExpressions {
						newMatchExpression := corev1.NodeSelectorRequirement{}
						newMatchExpression.Key = match.Key
						newMatchExpression.Operator = corev1.NodeSelectorOperator(match.Operator)
						newMatchExpression.Values = match.Values
						agentMatchExpressions = append(agentMatchExpressions, newMatchExpression)
					}
				}
				agentMatchFields := []corev1.NodeSelectorRequirement{}
				if term.MatchFields != nil {
					for _, match := range term.MatchFields {
						newMatchExpression := corev1.NodeSelectorRequirement{}
						newMatchExpression.Key = match.Key
						newMatchExpression.Operator = corev1.NodeSelectorOperator(match.Operator)
						newMatchExpression.Values = match.Values
						agentMatchFields = append(agentMatchFields, newMatchExpression)
					}
				}
				agentAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = append(agentAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms, corev1.NodeSelectorTerm{
					MatchExpressions: agentMatchExpressions,
					MatchFields:      agentMatchFields,
				})
			}
		}
		if advancedClusterAffinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution != nil {
			agentAffinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = []corev1.PreferredSchedulingTerm{}
			for _, preferred := range advancedClusterAffinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
				termPreferences := corev1.NodeSelectorTerm{}
				if preferred.Preference.MatchExpressions != nil {
					termPreferences.MatchExpressions = []corev1.NodeSelectorRequirement{}
					for _, match := range preferred.Preference.MatchExpressions {
						newMatchExpression := corev1.NodeSelectorRequirement{}
						newMatchExpression.Key = match.Key
						newMatchExpression.Operator = corev1.NodeSelectorOperator(match.Operator)
						newMatchExpression.Values = match.Values
						termPreferences.MatchExpressions = append(termPreferences.MatchExpressions, newMatchExpression)
					}
				}
				if preferred.Preference.MatchFields != nil {
					termPreferences.MatchFields = []corev1.NodeSelectorRequirement{}
					for _, match := range preferred.Preference.MatchFields {
						newMatchExpression := corev1.NodeSelectorRequirement{}
						newMatchExpression.Key = match.Key
						newMatchExpression.Operator = corev1.NodeSelectorOperator(match.Operator)
						newMatchExpression.Values = match.Values
						termPreferences.MatchFields = append(termPreferences.MatchFields, newMatchExpression)
					}
				}
				agentAffinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(agentAffinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution, corev1.PreferredSchedulingTerm{
					Weight:     int32(preferred.Weight),
					Preference: termPreferences,
				})
			}
		}
	}
	if advancedClusterAffinity.PodAffinity != nil {
		agentAffinity.PodAffinity = &corev1.PodAffinity{}
		if advancedClusterAffinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
			agentAffinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution = []corev1.PodAffinityTerm{}
			for _, term := range advancedClusterAffinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
				matchExpressions := []metav1.LabelSelectorRequirement{}
				if term.LabelSelector != nil {
					for _, expression := range term.LabelSelector.MatchExpressions {
						newExpression := metav1.LabelSelectorRequirement{}
						newExpression.Key = expression.Key
						newExpression.Operator = metav1.LabelSelectorOperator(expression.Operator)
						newExpression.Values = expression.Values
						matchExpressions = append(matchExpressions, newExpression)
					}
				}
				matchNamespaces := metav1.LabelSelector{}
				if term.NamespaceSelector != nil {
					if term.NamespaceSelector.MatchLabels != nil {
						matchNamespaces.MatchLabels = term.NamespaceSelector.MatchLabels
					}
					for _, expression := range term.NamespaceSelector.MatchExpressions {
						newExpression := metav1.LabelSelectorRequirement{}
						newExpression.Key = expression.Key
						newExpression.Operator = metav1.LabelSelectorOperator(expression.Operator)
						newExpression.Values = expression.Values
						matchNamespaces.MatchExpressions = append(matchNamespaces.MatchExpressions, newExpression)
					}
				}
				newAffinityTerms := corev1.PodAffinityTerm{
					TopologyKey: term.TopologyKey,
				}
				if len(term.Namespaces) > 0 {
					newAffinityTerms.Namespaces = term.Namespaces
				}
				if term.LabelSelector != nil {
					newAffinityTerms.LabelSelector = &metav1.LabelSelector{}
					if term.LabelSelector.MatchLabels != nil {
						newAffinityTerms.LabelSelector.MatchLabels = term.LabelSelector.MatchLabels
					}
					if len(matchExpressions) > 0 {
						newAffinityTerms.LabelSelector.MatchExpressions = matchExpressions
					}
				}
				if matchNamespaces.MatchLabels != nil || len(matchNamespaces.MatchExpressions) > 0 {
					newAffinityTerms.NamespaceSelector = &matchNamespaces
				}
				agentAffinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(agentAffinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution, newAffinityTerms)
			}
		}
		if advancedClusterAffinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution != nil {
			agentAffinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution = []corev1.WeightedPodAffinityTerm{}
			for _, preferred := range advancedClusterAffinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
				matchExpressions := []metav1.LabelSelectorRequirement{}
				if preferred.PodAffinityTerm.LabelSelector != nil {
					for _, expression := range preferred.PodAffinityTerm.LabelSelector.MatchExpressions {
						newExpression := metav1.LabelSelectorRequirement{}
						newExpression.Key = expression.Key
						newExpression.Operator = metav1.LabelSelectorOperator(expression.Operator)
						newExpression.Values = expression.Values
						matchExpressions = append(matchExpressions, newExpression)
					}
				}
				matchNamespaces := metav1.LabelSelector{}
				if preferred.PodAffinityTerm.NamespaceSelector != nil {
					if preferred.PodAffinityTerm.NamespaceSelector.MatchLabels == nil {
						matchNamespaces.MatchLabels = preferred.PodAffinityTerm.NamespaceSelector.MatchLabels
					}
					for _, expression := range preferred.PodAffinityTerm.NamespaceSelector.MatchExpressions {
						newExpression := metav1.LabelSelectorRequirement{}
						newExpression.Key = expression.Key
						newExpression.Operator = metav1.LabelSelectorOperator(expression.Operator)
						newExpression.Values = expression.Values
						matchNamespaces.MatchExpressions = append(matchNamespaces.MatchExpressions, newExpression)
					}
				}
				newAffinityTerms := corev1.WeightedPodAffinityTerm{
					Weight: int32(preferred.Weight),
					PodAffinityTerm: corev1.PodAffinityTerm{
						TopologyKey: preferred.PodAffinityTerm.TopologyKey,
					},
				}
				// add in optional variables if they exist
				if preferred.PodAffinityTerm.Namespaces != nil {
					newAffinityTerms.PodAffinityTerm.Namespaces = preferred.PodAffinityTerm.Namespaces
				}
				if matchNamespaces.MatchLabels != nil || matchNamespaces.MatchExpressions != nil {
					newAffinityTerms.PodAffinityTerm.NamespaceSelector = &matchNamespaces
				}
				if preferred.PodAffinityTerm.LabelSelector != nil {
					newAffinityTerms.PodAffinityTerm.LabelSelector = &metav1.LabelSelector{}
					if preferred.PodAffinityTerm.LabelSelector.MatchLabels != nil {
						newAffinityTerms.PodAffinityTerm.LabelSelector.MatchLabels = preferred.PodAffinityTerm.LabelSelector.MatchLabels
					}
					if preferred.PodAffinityTerm.LabelSelector.MatchExpressions != nil {
						newAffinityTerms.PodAffinityTerm.LabelSelector.MatchExpressions = matchExpressions
					}
				}
				agentAffinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(agentAffinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution, newAffinityTerms)
			}
		}
	}
	if advancedClusterAffinity.PodAntiAffinity != nil {
		agentAffinity.PodAntiAffinity = &corev1.PodAntiAffinity{}
		if advancedClusterAffinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
			agentAffinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = []corev1.PodAffinityTerm{}
			for _, term := range advancedClusterAffinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
				matchExpressions := []metav1.LabelSelectorRequirement{}
				if term.LabelSelector != nil {
					for _, expression := range term.LabelSelector.MatchExpressions {
						newExpression := metav1.LabelSelectorRequirement{}
						newExpression.Key = expression.Key
						newExpression.Operator = metav1.LabelSelectorOperator(expression.Operator)
						newExpression.Values = expression.Values
						matchExpressions = append(matchExpressions, newExpression)
					}
				}
				matchNamespaces := metav1.LabelSelector{}
				if term.NamespaceSelector != nil {
					if term.NamespaceSelector.MatchLabels != nil {
						matchNamespaces.MatchLabels = term.NamespaceSelector.MatchLabels
					}
					for _, expression := range term.NamespaceSelector.MatchExpressions {
						newExpression := metav1.LabelSelectorRequirement{}
						newExpression.Key = expression.Key
						newExpression.Operator = metav1.LabelSelectorOperator(expression.Operator)
						newExpression.Values = expression.Values
						matchNamespaces.MatchExpressions = append(matchNamespaces.MatchExpressions, newExpression)
					}
				}
				newAffinityTerms := corev1.PodAffinityTerm{
					TopologyKey: term.TopologyKey,
				}
				if len(term.Namespaces) > 0 {
					newAffinityTerms.Namespaces = term.Namespaces
				}
				if term.LabelSelector != nil {
					newAffinityTerms.LabelSelector = &metav1.LabelSelector{}
					if term.LabelSelector.MatchLabels != nil {
						newAffinityTerms.LabelSelector.MatchLabels = term.LabelSelector.MatchLabels
					}
					if len(matchExpressions) > 0 {
						newAffinityTerms.LabelSelector.MatchExpressions = matchExpressions
					}
				}
				if matchNamespaces.MatchLabels != nil || len(matchNamespaces.MatchExpressions) > 0 {
					newAffinityTerms.NamespaceSelector = &matchNamespaces
				}
				agentAffinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(agentAffinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution, newAffinityTerms)
			}
		}
		if advancedClusterAffinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution != nil {
			agentAffinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution = []corev1.WeightedPodAffinityTerm{}
			for _, preferred := range advancedClusterAffinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
				matchExpressions := []metav1.LabelSelectorRequirement{}
				if preferred.PodAffinityTerm.LabelSelector != nil {
					for _, expression := range preferred.PodAffinityTerm.LabelSelector.MatchExpressions {
						newExpression := metav1.LabelSelectorRequirement{}
						newExpression.Key = expression.Key
						newExpression.Operator = metav1.LabelSelectorOperator(expression.Operator)
						newExpression.Values = expression.Values
						matchExpressions = append(matchExpressions, newExpression)
					}
				}
				matchNamespaces := metav1.LabelSelector{}
				if preferred.PodAffinityTerm.NamespaceSelector != nil {
					if preferred.PodAffinityTerm.NamespaceSelector.MatchLabels == nil {
						matchNamespaces.MatchLabels = preferred.PodAffinityTerm.NamespaceSelector.MatchLabels
					}
					for _, expression := range preferred.PodAffinityTerm.NamespaceSelector.MatchExpressions {
						newExpression := metav1.LabelSelectorRequirement{}
						newExpression.Key = expression.Key
						newExpression.Operator = metav1.LabelSelectorOperator(expression.Operator)
						newExpression.Values = expression.Values
						matchNamespaces.MatchExpressions = append(matchNamespaces.MatchExpressions, newExpression)
					}
				}
				newAffinityTerms := corev1.WeightedPodAffinityTerm{
					Weight: int32(preferred.Weight),
					PodAffinityTerm: corev1.PodAffinityTerm{
						TopologyKey: preferred.PodAffinityTerm.TopologyKey,
					},
				}
				// add in optional variables if they exist
				if preferred.PodAffinityTerm.Namespaces != nil {
					newAffinityTerms.PodAffinityTerm.Namespaces = preferred.PodAffinityTerm.Namespaces
				}
				if matchNamespaces.MatchLabels != nil || matchNamespaces.MatchExpressions != nil {
					newAffinityTerms.PodAffinityTerm.NamespaceSelector = &matchNamespaces
				}
				if preferred.PodAffinityTerm.LabelSelector != nil {
					newAffinityTerms.PodAffinityTerm.LabelSelector = &metav1.LabelSelector{}
					if preferred.PodAffinityTerm.LabelSelector.MatchLabels != nil {
						newAffinityTerms.PodAffinityTerm.LabelSelector.MatchLabels = preferred.PodAffinityTerm.LabelSelector.MatchLabels
					}
					if preferred.PodAffinityTerm.LabelSelector.MatchExpressions != nil {
						newAffinityTerms.PodAffinityTerm.LabelSelector.MatchExpressions = matchExpressions
					}
				}
				agentAffinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(agentAffinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution, newAffinityTerms)
			}
		}
	}
	return agentAffinity
}

// HardenK3SClusterConfig is a function that modifies the cluster configuration to be hardened according to the CIS benchmark.
func HardenK3SClusterConfig(clusterName, namespace string, clustersConfig *ClusterConfig, machinePools []apisV1.RKEMachinePool, cloudCredentialSecretName string) *apisV1.Cluster {
	v1Cluster := NewK3SRKE2ClusterConfig(clusterName, namespace, clustersConfig, machinePools, cloudCredentialSecretName)
	v1Cluster.Spec.DefaultPodSecurityAdmissionConfigurationTemplateName = rancherRestricted

	v1Cluster.Spec.RKEConfig.MachineGlobalConfig.Data["kube-apiserver-arg"] = []string{
		"audit-policy-file=/var/lib/rancher/k3s/server/audit.yaml",
		"audit-log-path=/var/lib/rancher/k3s/server/logs/audit.log",
		"audit-log-maxage=30",
		"audit-log-maxbackup=10",
		"audit-log-maxsize=100",
		"request-timeout=300s",
		"service-account-lookup=true",
	}

	v1Cluster.Spec.RKEConfig.MachineSelectorConfig = []rkev1.RKESystemConfig{
		{
			Config: rkev1.GenericMap{
				Data: map[string]interface{}{
					"kubelet-arg": []string{
						"make-iptables-util-chains=true",
					},
					protectKernelDefaults: true,
				},
			},
		},
	}

	return v1Cluster
}

// HardenRKE1ClusterConfig is a function that modifies the cluster configuration to be hardened according to the CIS benchmark.
func HardenRKE1ClusterConfig(client *rancher.Client, clusterName string, clustersConfig *ClusterConfig) *management.Cluster {
	cluster := NewRKE1ClusterConfig(clusterName, client, clustersConfig)

	cluster.DefaultPodSecurityAdmissionConfigurationTemplateName = rancherRestricted
	cluster.RancherKubernetesEngineConfig.Services.Etcd.GID = rke1HardenedGID
	cluster.RancherKubernetesEngineConfig.Services.Etcd.UID = rke1HardenedUID

	return cluster
}

// HardenRKE2ClusterConfig is a function that modifies the cluster configuration to be hardened according to the CIS benchmark.
func HardenRKE2ClusterConfig(clusterName, namespace string, clustersConfig *ClusterConfig, machinePools []apisV1.RKEMachinePool, cloudCredentialSecretName string) *apisV1.Cluster {
	v1Cluster := NewK3SRKE2ClusterConfig(clusterName, namespace, clustersConfig, machinePools, cloudCredentialSecretName)
	v1Cluster.Spec.DefaultPodSecurityAdmissionConfigurationTemplateName = rancherRestricted

	v1Cluster.Spec.RKEConfig.MachineSelectorConfig = []rkev1.RKESystemConfig{
		{
			Config: rkev1.GenericMap{
				Data: map[string]interface{}{
					"profile":             "cis",
					protectKernelDefaults: true,
				},
			},
		},
	}

	return v1Cluster
}

// CheckServiceAccountTokenSecret verifies if a serviceAccountTokenSecret exists or not in the cluster.
func CheckServiceAccountTokenSecret(client *rancher.Client, clusterName string) (success bool, err error) {
	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	if err != nil {
		return false, err
	}

	cluster, err := client.Management.Cluster.ByID(clusterID)
	if err != nil {
		return false, err
	}

	if cluster.ServiceAccountTokenSecret == "" {
		logrus.Warn("warning: serviceAccountTokenSecret does not exist in this cluster!")
		return false, nil
	}

	logrus.Infof("serviceAccountTokenSecret in this cluster is: %s", cluster.ServiceAccountTokenSecret)
	return true, nil
}

// OutOfTreeSystemConfig constructs the proper rkeSystemConfig slice for enabling the aws cloud provider
// out-of-tree services
func OutOfTreeSystemConfig(providerName string) (rkeConfig []rkev1.RKESystemConfig) {
	roles := []string{etcdRole, controlPlaneRole, workerRole}

	for _, role := range roles {
		selector := &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"rke.cattle.io/" + role: "true",
			},
		}
		configData := map[string]interface{}{}

		configData[kubeletArgKey] = []string{externalCloudProviderString}

		if role == controlPlaneRole {
			configData[kubeletAPIServerArgKey] = []string{externalCloudProviderString}
			configData[kubeControllerManagerArgKey] = []string{externalCloudProviderString}
		}

		if role == workerRole || role == controlPlaneRole {
			configData[disableCloudController] = true
		}

		rkeConfig = append(rkeConfig, RKESystemConfigTemplate(configData, selector))
	}

	configData := map[string]interface{}{
		cloudProviderAnnotationName: providerName,
		protectKernelDefaults:       false,
	}

	rkeConfig = append(rkeConfig, RKESystemConfigTemplate(configData, nil))
	return
}

// InTreeSystemConfig constructs the proper rkeSystemConfig slice for enabling cloud provider
// in-tree services.
// Vsphere deprecated 1.21+
// AWS deprecated 1.27+
// Azure deprecated 1.28+
func InTreeSystemConfig(providerName string) (rkeConfig []rkev1.RKESystemConfig) {
	configData := map[string]interface{}{
		cloudProviderAnnotationName: providerName,
		protectKernelDefaults:       false,
	}
	rkeConfig = append(rkeConfig, RKESystemConfigTemplate(configData, nil))
	return
}

// RKESYstemConfigTemplate constructs an RKESystemConfig object given config data and a selector
func RKESystemConfigTemplate(config map[string]interface{}, selector *metav1.LabelSelector) rkev1.RKESystemConfig {
	return rkev1.RKESystemConfig{
		Config: rkev1.GenericMap{
			Data: config,
		},
		MachineLabelSelector: selector,
	}
}
