package clusters

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/api/scheme"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/defaults"
	"github.com/rancher/rancher/tests/framework/extensions/provisioninginput"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/wrangler/pkg/summary"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	active                               = "active"
	baseline                             = "baseline"
	externalAws                          = "external-aws"
	FleetSteveResourceType               = "fleet.cattle.io.cluster"
	PodSecurityAdmissionSteveResoureType = "management.cattle.io.podsecurityadmissionconfigurationtemplate"
	ProvisioningSteveResourceType        = "provisioning.cattle.io.cluster"

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

	clusterStateUpgrading    = "upgrading" // For imported RKE2 and K3s clusters
	clusterStateUpdating     = "updating"  // For all clusters except imported K3s and RKE2
	clusterErrorStateMessage = "cluster is in error state"
)

// GetV1ProvisioningClusterByName is a helper function that returns the cluster ID by name
func GetV1ProvisioningClusterByName(client *rancher.Client, clusterName string) (string, error) {
	clusterList, err := client.Steve.SteveType(ProvisioningSteveResourceType).List(nil)
	if err != nil {
		return "", err
	}

	for _, cluster := range clusterList.Data {
		if cluster.Name == clusterName {
			return cluster.ID, nil
		}
	}

	return "", nil
}

// GetClusterIDByName is a helper function that returns the cluster ID by name
func GetClusterIDByName(client *rancher.Client, clusterName string) (string, error) {
	clusterList, err := client.Management.Cluster.List(&types.ListOpts{})
	if err != nil {
		return "", err
	}

	for _, cluster := range clusterList.Data {
		if cluster.Name == clusterName {
			return cluster.ID, nil
		}
	}

	return "", nil
}

// GetClusterNameByID is a helper function that returns the cluster ID by name
func GetClusterNameByID(client *rancher.Client, clusterID string) (string, error) {
	clusterList, err := client.Management.Cluster.List(&types.ListOpts{})
	if err != nil {
		return "", err
	}

	for _, cluster := range clusterList.Data {
		if cluster.ID == clusterID {
			return cluster.Name, nil
		}
	}

	return "", nil
}

// IsProvisioningClusterReady is basic check function that would be used for the wait.WatchWait func in pkg/wait.
// This functions just waits until a cluster becomes ready.
func IsProvisioningClusterReady(event watch.Event) (ready bool, err error) {
	cluster := event.Object.(*apisV1.Cluster)
	var updated bool
	ready = cluster.Status.Ready
	for _, condition := range cluster.Status.Conditions {
		if condition.Type == "Updated" && condition.Status == corev1.ConditionTrue {
			updated = true
			logrus.Infof("Cluster status is active!")
		}
	}

	return ready && updated, nil
}

// IsHostedProvisioningClusterReady is basic check function that would be used for the wait.WatchWait func in pkg/wait.
// This functions just waits until a hosted cluster becomes ready.
func IsHostedProvisioningClusterReady(event watch.Event) (ready bool, err error) {
	clusterUnstructured := event.Object.(*unstructured.Unstructured)
	cluster := &v3.Cluster{}
	err = scheme.Scheme.Convert(clusterUnstructured, cluster, clusterUnstructured.GroupVersionKind())
	if err != nil {
		return false, err
	}
	for _, cond := range cluster.Status.Conditions {
		if cond.Type == "Ready" && cond.Status == "True" {
			logrus.Infof("Cluster status is active!")
			return true, nil
		}
	}

	return false, nil
}

// CheckServiceAccountTokenSecret verifies if a serviceAccountTokenSecret exists or not in the cluster.
func CheckServiceAccountTokenSecret(client *rancher.Client, clusterName string) (success bool, err error) {
	clusterID, err := GetClusterIDByName(client, clusterName)
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

// CreateRancherBaselinePSACT creates custom PSACT called rancher-baseline which sets each PSS to baseline.
func CreateRancherBaselinePSACT(client *rancher.Client, psact string) error {
	_, err := client.Steve.SteveType(PodSecurityAdmissionSteveResoureType).ByID(psact)
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

	_, err = client.Steve.SteveType(PodSecurityAdmissionSteveResoureType).Create(template)
	if err != nil {
		return err
	}

	return nil
}

// NewRKE1lusterConfig is a constructor for a v3.Cluster object, to be used by the rancher.Client.Provisioning client.
func NewRKE1ClusterConfig(clusterName string, client *rancher.Client, clustersConfig *ClusterConfig) *management.Cluster {
	backupConfigEnabled := true
	criDockerBool := false
	if clustersConfig.CRIDockerd {
		criDockerBool = true
	}
	newConfig := &management.Cluster{
		DockerRootDir:           "/var/lib/docker",
		EnableClusterAlerting:   false,
		EnableClusterMonitoring: false,
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
		machineSelectorConfigs = append(machineSelectorConfigs, awsOutOfTreeSystemConfig()...)
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

	spec := apisV1.ClusterSpec{
		CloudCredentialSecretName:           cloudCredentialSecretName,
		KubernetesVersion:                   clustersConfig.KubernetesVersion,
		LocalClusterAuthEndpoint:            localClusterAuthEndpoint,
		RKEConfig:                           rkeConfig,
		ClusterAgentDeploymentCustomization: clusterAgentDeploymentCustomization,
		FleetAgentDeploymentCustomization:   fleetAgentDeploymentCustomization,
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

// awsOutOfTreeSystemConfig constructs the proper rkeSystemConfig slice for enabling the aws cloud provider
// out-of-tree services
func awsOutOfTreeSystemConfig() (rkeConfig []rkev1.RKESystemConfig) {
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
		cloudProviderAnnotationName: provisioninginput.AWSProviderName,
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

// HardenK3SClusterConfig is a constructor for a apisV1.Cluster object, to be used by the rancher.Client.Provisioning client.
func HardenK3SClusterConfig(clusterName, namespace string, clustersConfig *ClusterConfig, machinePools []apisV1.RKEMachinePool, cloudCredentialSecretName string) *apisV1.Cluster {
	v1Cluster := NewK3SRKE2ClusterConfig(clusterName, namespace, clustersConfig, machinePools, cloudCredentialSecretName)

	if clustersConfig.KubernetesVersion <= string(provisioninginput.PSPKubeVersionLimit) {
		v1Cluster.Spec.RKEConfig.MachineGlobalConfig.Data["kube-apiserver-arg"] = []string{
			"enable-admission-plugins=NodeRestriction,PodSecurityPolicy,ServiceAccount",
			"audit-policy-file=/var/lib/rancher/k3s/server/audit.yaml",
			"audit-log-path=/var/lib/rancher/k3s/server/logs/audit.log",
			"audit-log-maxage=30",
			"audit-log-maxbackup=10",
			"audit-log-maxsize=100",
			"request-timeout=300s",
			"service-account-lookup=true",
		}
	} else {
		v1Cluster.Spec.RKEConfig.MachineGlobalConfig.Data["kube-apiserver-arg"] = []string{
			"admission-control-config-file=/var/lib/rancher/k3s/server/psa.yaml",
			"audit-policy-file=/var/lib/rancher/k3s/server/audit.yaml",
			"audit-log-path=/var/lib/rancher/k3s/server/logs/audit.log",
			"audit-log-maxage=30",
			"audit-log-maxbackup=10",
			"audit-log-maxsize=100",
			"request-timeout=300s",
			"service-account-lookup=true",
		}
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

// HardenRKE2ClusterConfig is a constructor for a apisV1.Cluster object, to be used by the rancher.Client.Provisioning client.
func HardenRKE2ClusterConfig(clusterName, namespace string, clustersConfig *ClusterConfig, machinePools []apisV1.RKEMachinePool, cloudCredentialSecretName string) *apisV1.Cluster {
	v1Cluster := NewK3SRKE2ClusterConfig(clusterName, namespace, clustersConfig, machinePools, cloudCredentialSecretName)

	if clustersConfig.KubernetesVersion <= string(provisioninginput.PSPKubeVersionLimit) {
		v1Cluster.Spec.RKEConfig.MachineSelectorConfig = []rkev1.RKESystemConfig{
			{
				Config: rkev1.GenericMap{
					Data: map[string]interface{}{
						"profile":             "cis-1.6",
						protectKernelDefaults: true,
					},
				},
			},
		}
	} else {
		v1Cluster.Spec.RKEConfig.MachineSelectorConfig = []rkev1.RKESystemConfig{
			{
				Config: rkev1.GenericMap{
					Data: map[string]interface{}{
						"profile":                 "cis-1.23",
						"protect-kernel-defaults": true,
					},
				},
			},
		}
	}

	return v1Cluster
}

// CreateRKE1Cluster is a "helper" functions that takes a rancher client, and the rke1 cluster config as parameters. This function
// registers a delete cluster fuction with a wait.WatchWait to ensure the cluster is removed cleanly.
func CreateRKE1Cluster(client *rancher.Client, rke1Cluster *management.Cluster) (*management.Cluster, error) {
	cluster, err := client.Management.Cluster.Create(rke1Cluster)
	if err != nil {
		return nil, err
	}

	err = kwait.Poll(500*time.Millisecond, 2*time.Minute, func() (done bool, err error) {
		client, err = client.ReLogin()
		if err != nil {
			return false, err
		}

		_, err = client.Management.Cluster.ByID(cluster.ID)
		if err != nil {
			return false, nil
		}
		return true, nil
	})

	if err != nil {
		return nil, err
	}

	client.Session.RegisterCleanupFunc(func() error {
		adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
		if err != nil {
			return err
		}

		clusterResp, err := client.Management.Cluster.ByID(cluster.ID)
		if err != nil {
			return err
		}

		client, err = client.ReLogin()
		if err != nil {
			return err
		}

		err = client.Management.Cluster.Delete(clusterResp)
		if err != nil {
			return err
		}

		watchInterface, err := adminClient.GetManagementWatchInterface(management.ClusterType, metav1.ListOptions{
			FieldSelector:  "metadata.name=" + clusterResp.ID,
			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
		})
		if err != nil {
			return err
		}

		return wait.WatchWait(watchInterface, func(event watch.Event) (ready bool, err error) {
			if event.Type == watch.Error {
				return false, fmt.Errorf("there was an error deleting cluster")
			} else if event.Type == watch.Deleted {
				return true, nil
			}
			return false, nil
		})
	})

	return cluster, nil
}

// CreateK3SRKE2Cluster is a "helper" functions that takes a rancher client, and the rke2 cluster config as parameters. This function
// registers a delete cluster fuction with a wait.WatchWait to ensure the cluster is removed cleanly.
func CreateK3SRKE2Cluster(client *rancher.Client, rke2Cluster *apisV1.Cluster) (*v1.SteveAPIObject, error) {
	cluster, err := client.Steve.SteveType(ProvisioningSteveResourceType).Create(rke2Cluster)
	if err != nil {
		return nil, err
	}

	err = kwait.Poll(500*time.Millisecond, 2*time.Minute, func() (done bool, err error) {
		client, err = client.ReLogin()
		if err != nil {
			return false, err
		}

		_, err = client.Steve.SteveType(ProvisioningSteveResourceType).ByID(cluster.ID)
		if err != nil {
			return false, nil
		}

		return true, nil
	})

	if err != nil {
		return nil, err
	}

	client.Session.RegisterCleanupFunc(func() error {
		adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
		if err != nil {
			return err
		}

		provKubeClient, err := adminClient.GetKubeAPIProvisioningClient()
		if err != nil {
			return err
		}

		watchInterface, err := provKubeClient.Clusters(cluster.ObjectMeta.Namespace).Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  "metadata.name=" + cluster.ObjectMeta.Name,
			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
		})

		if err != nil {
			return err
		}

		client, err = client.ReLogin()
		if err != nil {
			return err
		}

		err = client.Steve.SteveType(ProvisioningSteveResourceType).Delete(cluster)
		if err != nil {
			return err
		}

		return wait.WatchWait(watchInterface, func(event watch.Event) (ready bool, err error) {
			cluster := event.Object.(*apisV1.Cluster)
			if event.Type == watch.Error {
				return false, fmt.Errorf("there was an error deleting cluster")
			} else if event.Type == watch.Deleted {
				return true, nil
			} else if cluster == nil {
				return true, nil
			}
			return false, nil
		})
	})

	return cluster, nil
}

// DeleteKE1Cluster is a "helper" functions that takes a rancher client, and the rke1 cluster ID as parameters to delete
// the cluster.
func DeleteRKE1Cluster(client *rancher.Client, clusterID string) error {
	cluster, err := client.Management.Cluster.ByID(clusterID)
	if err != nil {
		return err
	}

	logrus.Infof("Deleting cluster %s...", cluster.Name)
	err = client.Management.Cluster.Delete(cluster)
	if err != nil {
		return err
	}

	return nil
}

// DeleteK3SRKE2Cluster is a "helper" functions that takes a rancher client, and the non-rke1 cluster ID as parameters to delete
// the cluster.
func DeleteK3SRKE2Cluster(client *rancher.Client, clusterID string) error {
	cluster, err := client.Steve.SteveType(ProvisioningSteveResourceType).ByID(clusterID)
	if err != nil {
		return err
	}

	logrus.Infof("Deleting cluster %s...", cluster.Name)
	err = client.Steve.SteveType(ProvisioningSteveResourceType).Delete(cluster)
	if err != nil {
		return err
	}

	return nil
}

// UpdateK3SRKE2Cluster is a "helper" functions that takes a rancher client, old rke2/k3s cluster config, and the new rke2/k3s cluster config as parameters.
func UpdateK3SRKE2Cluster(client *rancher.Client, cluster *v1.SteveAPIObject, updatedCluster *apisV1.Cluster) (*v1.SteveAPIObject, error) {
	updateCluster, err := client.Steve.SteveType(ProvisioningSteveResourceType).ByID(cluster.ID)
	if err != nil {
		return nil, err
	}

	updatedCluster.ObjectMeta.ResourceVersion = updateCluster.ObjectMeta.ResourceVersion

	cluster, err = client.Steve.SteveType(ProvisioningSteveResourceType).Update(cluster, updatedCluster)
	if err != nil {
		return nil, err
	}

	err = kwait.Poll(500*time.Millisecond, 5*time.Minute, func() (done bool, err error) {
		client, err = client.ReLogin()
		if err != nil {
			return false, err
		}

		clusterResp, err := client.Steve.SteveType(ProvisioningSteveResourceType).ByID(cluster.ID)
		if err != nil {
			return false, err
		}

		clusterStatus := &apisV1.ClusterStatus{}
		err = v1.ConvertToK8sType(clusterResp.Status, clusterStatus)
		if err != nil {
			return false, err
		}

		if clusterResp.ObjectMeta.State.Name == active {
			proxyClient, err := client.Steve.ProxyDownstream(clusterStatus.ClusterName)
			if err != nil {
				return false, err
			}

			_, err = proxyClient.SteveType(pods.PodResourceSteveType).List(nil)
			if err != nil {
				return false, nil
			}
			logrus.Infof("Cluster has been successfully updated!")
			return true, nil
		}
		return false, nil
	})

	if err != nil {
		return nil, err
	}

	return cluster, nil
}

// WaitClusterToBeInUpgrade is a helper function that takes a rancher client, and the cluster id as parameters.
// Waits cluster to be in upgrade state.
// Cluster error states that declare control plane is inaccessible and cluster object modified are ignored.
// Same cluster summary information logging is ignored.
func WaitClusterToBeInUpgrade(client *rancher.Client, clusterID string) (err error) {
	var clusterInfo string
	opts := metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterID,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	}

	watchInterface, err := client.GetManagementWatchInterface(management.ClusterType, opts)
	if err != nil {
		return
	}
	checkFuncWaitToBeInUpgrade := func(event watch.Event) (bool, error) {
		clusterUnstructured := event.Object.(*unstructured.Unstructured)
		summarizedCluster := summary.Summarize(clusterUnstructured)

		clusterInfo = logClusterInfoWithChanges(clusterID, clusterInfo, summarizedCluster)

		if summarizedCluster.Transitioning && !summarizedCluster.Error && (summarizedCluster.State == clusterStateUpdating || summarizedCluster.State == clusterStateUpgrading) {
			return true, nil
		} else if summarizedCluster.Error && isClusterInaccessible(summarizedCluster.Message) {
			return false, nil
		} else if summarizedCluster.Error && !isClusterInaccessible(summarizedCluster.Message) {
			return false, errors.Wrap(err, clusterErrorStateMessage)
		}

		return false, nil
	}
	err = wait.WatchWait(watchInterface, checkFuncWaitToBeInUpgrade)
	if err != nil {
		return
	}

	return
}

// WaitClusterUntilUpgrade is a helper function that takes a rancher client, and the cluster id as parameters.
// Waits until cluster is ready.
// Cluster error states that declare control plane is inaccessible and cluster object modified are ignored.
// Same cluster summary information logging is ignored.
func WaitClusterUntilUpgrade(client *rancher.Client, clusterID string) (err error) {
	var clusterInfo string
	opts := metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterID,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	}

	watchInterfaceWaitUpgrade, err := client.GetManagementWatchInterface(management.ClusterType, opts)
	if err != nil {
		return
	}
	checkFuncWaitUpgrade := func(event watch.Event) (bool, error) {
		clusterUnstructured := event.Object.(*unstructured.Unstructured)
		summarizedCluster := summary.Summarize(clusterUnstructured)

		clusterInfo = logClusterInfoWithChanges(clusterID, clusterInfo, summarizedCluster)

		if summarizedCluster.IsReady() {
			return true, nil
		} else if summarizedCluster.Error && isClusterInaccessible(summarizedCluster.Message) {
			return false, nil
		} else if summarizedCluster.Error && !isClusterInaccessible(summarizedCluster.Message) {
			return false, errors.Wrap(err, clusterErrorStateMessage)

		}

		return false, nil
	}

	err = wait.WatchWait(watchInterfaceWaitUpgrade, checkFuncWaitUpgrade)
	if err != nil {
		return err
	}

	return
}

// WaitForClusterToBeUpgraded is a "helper" functions that takes a rancher client, and the cluster id as parameters. This function
// contains two stages. First stage is to wait to be cluster in upgrade state. And the other is to wait until cluster is ready.
// Cluster error states that declare control plane is inaccessible and cluster object modified are ignored.
// Same cluster summary information logging is ignored.
func WaitClusterToBeUpgraded(client *rancher.Client, clusterID string) (err error) {
	err = WaitClusterToBeInUpgrade(client, clusterID)
	if err != nil {
		return err
	}

	err = WaitClusterUntilUpgrade(client, clusterID)
	if err != nil {
		return err
	}

	return
}

func isClusterInaccessible(messages []string) (isInaccessible bool) {
	clusterCPErrorMessage := "Cluster health check failed: Failed to communicate with API server during namespace check" // For GKE
	clusterModifiedErrorMessage := "the object has been modified"                                                        // For provisioning node driver K3s and RKE2

	for _, message := range messages {
		if strings.Contains(message, clusterCPErrorMessage) || strings.Contains(message, clusterModifiedErrorMessage) {
			isInaccessible = true
			break
		}
	}

	return
}

func logClusterInfoWithChanges(clusterID, clusterInfo string, summary summary.Summary) string {
	newClusterInfo := fmt.Sprintf("ClusterID: %v, Message: %v, Error: %v, State: %v, Transitioning: %v", clusterID, summary.Message, summary.Error, summary.State, summary.Transitioning)

	if clusterInfo != newClusterInfo {
		logrus.Infof(newClusterInfo)
		clusterInfo = newClusterInfo
	}

	return clusterInfo
}

// WatchAndWaitForCluster is function that waits for a cluster to go unactive before checking its active state.
func WatchAndWaitForCluster(client *rancher.Client, steveID string) error {
	var clusterResp *v1.SteveAPIObject
	err := kwait.Poll(500*time.Millisecond, 2*time.Minute, func() (done bool, err error) {
		clusterResp, err = client.Steve.SteveType(ProvisioningSteveResourceType).ByID(steveID)
		if err != nil {
			return false, err
		}
		state := clusterResp.ObjectMeta.State.Name
		return state != "active", nil
	})
	if err != nil {
		return err
	}
	logrus.Infof("waiting for cluster to be up.............")

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	if err != nil {
		return err
	}
	kubeProvisioningClient, err := adminClient.GetKubeAPIProvisioningClient()
	if err != nil {
		return err
	}

	result, err := kubeProvisioningClient.Clusters(clusterResp.ObjectMeta.Namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterResp.Name,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return err
	}

	err = wait.WatchWait(result, IsProvisioningClusterReady)
	return err
}

// GetProvisioningClusterByName is a helper function to get cluster object with the cluster name
func GetProvisioningClusterByName(client *rancher.Client, clusterName string, namespace string) (*apisV1.Cluster, *v1.SteveAPIObject, error) {
	clusterObj, err := client.Steve.SteveType(ProvisioningSteveResourceType).ByID(namespace + "/" + clusterName)
	if err != nil {
		return nil, nil, err
	}

	cluster := new(apisV1.Cluster)
	err = v1.ConvertToK8sType(clusterObj, &cluster)
	if err != nil {
		return nil, nil, err
	}

	return cluster, clusterObj, nil
}

// WaitForActiveCluster is a "helper" function that waits for the cluster to reach the active state.
// The function accepts a Rancher client and a cluster ID as parameters.
func WaitForActiveRKE1Cluster(client *rancher.Client, clusterID string) error {
	err := kwait.Poll(500*time.Millisecond, 30*time.Minute, func() (done bool, err error) {
		client, err = client.ReLogin()
		if err != nil {
			return false, err
		}
		clusterResp, err := client.Management.Cluster.ByID(clusterID)
		if err != nil {
			return false, err
		}
		if clusterResp.State == active {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return err
	}
	return nil
}
