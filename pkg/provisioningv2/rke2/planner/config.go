package planner

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/rancher/rancher/pkg/controllers/management/secretmigrator"

	v1 "k8s.io/api/core/v1"

	"github.com/rancher/norman/types/values"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	"github.com/rancher/rancher/pkg/nodeconfig"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/data/convert"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/kv"
	"github.com/rancher/wrangler/pkg/yaml"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

func (p *Planner) addETCD(config map[string]interface{}, controlPlane *rkev1.RKEControlPlane, entry *planEntry) (result []plan.File, _ error) {
	if !isEtcd(entry) || controlPlane.Spec.ETCD == nil {
		return nil, nil
	}

	if controlPlane.Spec.ETCD.DisableSnapshots {
		config["etcd-disable-snapshots"] = true
	}
	if controlPlane.Spec.ETCD.SnapshotRetention > 0 {
		config["etcd-snapshot-retention"] = controlPlane.Spec.ETCD.SnapshotRetention
	}
	if controlPlane.Spec.ETCD.SnapshotScheduleCron != "" {
		config["etcd-snapshot-schedule-cron"] = controlPlane.Spec.ETCD.SnapshotScheduleCron
	}

	args, _, files, err := p.etcdS3Args.ToArgs(controlPlane.Spec.ETCD.S3, controlPlane, "etcd-", false)
	if err != nil {
		return nil, err
	}
	for _, arg := range args {
		k, v := kv.Split(arg, "=")
		k = strings.TrimPrefix(k, "--")
		if v == "" {
			config[k] = true
		} else {
			config[k] = v
		}
	}
	result = files

	return
}

func addDefaults(config map[string]interface{}, controlPlane *rkev1.RKEControlPlane) {
	if rke2.GetRuntime(controlPlane.Spec.KubernetesVersion) == rke2.RuntimeRKE2 {
		config["cni"] = "calico"
	}
}

func addUserConfig(config map[string]interface{}, controlPlane *rkev1.RKEControlPlane, entry *planEntry) error {
	for k, v := range controlPlane.Spec.MachineGlobalConfig.Data {
		config[k] = v
	}

	for _, opts := range controlPlane.Spec.MachineSelectorConfig {
		sel, err := metav1.LabelSelectorAsSelector(opts.MachineLabelSelector)
		if err != nil {
			return err
		}
		if opts.MachineLabelSelector == nil || sel.Matches(labels.Set(entry.Machine.Labels)) {
			for k, v := range opts.Config.Data {
				config[k] = v
			}
		}
	}

	filterConfigData(config, controlPlane, entry)
	return nil
}

func addRoleConfig(config map[string]interface{}, controlPlane *rkev1.RKEControlPlane, entry *planEntry, initNode bool, joinServer string) {
	runtime := rke2.GetRuntime(controlPlane.Spec.KubernetesVersion)
	if initNode {
		if runtime == rke2.RuntimeK3S {
			config["cluster-init"] = true
		}
	} else if joinServer != "" {
		// it's very important that the joinServer param isn't used on the initNode. The init node is special
		// because it will be evaluated twice, first with joinServer = "" and then with joinServer == self.
		// If we use the joinServer param then we will get different nodePlan and cause issues.
		config["server"] = joinServer
	}

	if IsOnlyEtcd(entry) {
		config["disable-scheduler"] = true
		config["disable-apiserver"] = true
		config["disable-controller-manager"] = true
	} else if isOnlyControlPlane(entry) {
		config["disable-etcd"] = true
	}

	if sdr := settings.SystemDefaultRegistry.Get(); sdr != "" && !isOnlyWorker(entry) {
		// only pass the global system-default-registry if we have not defined a different registry for system-images within the UI.
		// registries.yaml should take precedence over the global default registry.
		clusterRegistries := controlPlane.Spec.RKEClusterSpecCommon.Registries
		if clusterRegistries == nil || (clusterRegistries != nil && len(clusterRegistries.Configs) == 0) {
			config["system-default-registry"] = sdr
		}
	}

	// If this is a control-plane node, then we need to set arguments/(and for RKE2, volume mounts) to allow probes
	// to run.
	if isControlPlane(entry) {
		logrus.Debug("addRoleConfig rendering arguments and mounts for kube-controller-manager")
		certDirArg, certDirMount := renderArgAndMount(config[KubeControllerManagerArg], config[KubeControllerManagerExtraMount], runtime, DefaultKubeControllerManagerDefaultSecurePort, DefaultKubeControllerManagerCertDir)
		config[KubeControllerManagerArg] = certDirArg
		if runtime == rke2.RuntimeRKE2 {
			config[KubeControllerManagerExtraMount] = certDirMount
		}

		logrus.Debug("addRoleConfig rendering arguments and mounts for kube-scheduler")
		certDirArg, certDirMount = renderArgAndMount(config[KubeSchedulerArg], config[KubeSchedulerExtraMount], runtime, DefaultKubeSchedulerDefaultSecurePort, DefaultKubeSchedulerCertDir)
		config[KubeSchedulerArg] = certDirArg
		if runtime == rke2.RuntimeRKE2 {
			config[KubeSchedulerExtraMount] = certDirMount
		}
	}

	if nodeName := entry.Metadata.Labels[rke2.NodeNameLabel]; nodeName != "" {
		config["node-name"] = nodeName
	}
}

func addLocalClusterAuthenticationEndpointConfig(config map[string]interface{}, controlPlane *rkev1.RKEControlPlane, entry *planEntry) {
	if isOnlyWorker(entry) || !controlPlane.Spec.LocalClusterAuthEndpoint.Enabled {
		return
	}

	authFile := fmt.Sprintf(authnWebhookFileName, rke2.GetRuntime(controlPlane.Spec.KubernetesVersion))
	config["kube-apiserver-arg"] = append(convert.ToStringSlice(config["kube-apiserver-arg"]),
		fmt.Sprintf("authentication-token-webhook-config-file=%s", authFile))
}

func addLocalClusterAuthenticationEndpointFile(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane, entry *planEntry) plan.NodePlan {
	if isOnlyWorker(entry) || !controlPlane.Spec.LocalClusterAuthEndpoint.Enabled {
		return nodePlan
	}

	authFile := fmt.Sprintf(authnWebhookFileName, rke2.GetRuntime(controlPlane.Spec.KubernetesVersion))
	nodePlan.Files = append(nodePlan.Files, plan.File{
		Content: base64.StdEncoding.EncodeToString(AuthnWebhook),
		Path:    authFile,
	})

	return nodePlan
}

func (p *Planner) addManifests(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane, entry *planEntry) (plan.NodePlan, error) {
	files, err := p.getControlPlaneManifests(controlPlane, entry)
	if err != nil {
		return nodePlan, err
	}
	nodePlan.Files = append(nodePlan.Files, files...)

	return nodePlan, nil
}

func isVSphereProvider(controlPlane *rkev1.RKEControlPlane, entry *planEntry) (bool, error) {
	data := map[string]interface{}{}
	if err := addUserConfig(data, controlPlane, entry); err != nil {
		return false, err
	}
	return data["cloud-provider-name"] == "rancher-vsphere", nil
}

func addVSphereCharts(controlPlane *rkev1.RKEControlPlane, entry *planEntry) (map[string]interface{}, error) {
	if isVSphere, err := isVSphereProvider(controlPlane, entry); err != nil {
		return nil, err
	} else if isVSphere && controlPlane.Spec.ChartValues.Data["rancher-vsphere-csi"] == nil {
		// ensure we have this chart config so that the global.cattle.clusterId is set
		newData := controlPlane.Spec.ChartValues.DeepCopy()
		if newData.Data == nil {
			newData.Data = map[string]interface{}{}
		}
		newData.Data["rancher-vsphere-csi"] = map[string]interface{}{}
		return newData.Data, nil
	}

	return controlPlane.Spec.ChartValues.Data, nil
}

func (p *Planner) addChartConfigs(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane, entry *planEntry) (plan.NodePlan, error) {
	if isOnlyWorker(entry) {
		return nodePlan, nil
	}

	chartValues, err := addVSphereCharts(controlPlane, entry)
	if err != nil {
		return nodePlan, err
	}

	var chartConfigs []runtime.Object
	for _, chart := range rke2.SortedKeys(chartValues) {
		valuesMap := convert.ToMapInterface(chartValues[chart])
		if valuesMap == nil {
			valuesMap = map[string]interface{}{}
		}
		data.PutValue(valuesMap, controlPlane.Spec.ManagementClusterName, "global", "cattle", "clusterId")

		data, err := json.Marshal(valuesMap)
		if err != nil {
			return plan.NodePlan{}, err
		}

		chartConfigs = append(chartConfigs, &helmChartConfig{
			TypeMeta: metav1.TypeMeta{
				Kind:       "HelmChartConfig",
				APIVersion: "helm.cattle.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      chart,
				Namespace: "kube-system",
			},
			Spec: helmChartConfigSpec{
				ValuesContent: string(data),
			},
		})
	}
	contents, err := yaml.ToBytes(chartConfigs)
	if err != nil {
		return plan.NodePlan{}, err
	}

	nodePlan.Files = append(nodePlan.Files, plan.File{
		Content: base64.StdEncoding.EncodeToString(contents),
		Path:    fmt.Sprintf("/var/lib/rancher/%s/server/manifests/rancher/managed-chart-config.yaml", rke2.GetRuntime(controlPlane.Spec.KubernetesVersion)),
		Dynamic: true,
	})

	return nodePlan, nil
}

func addOtherFiles(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane, entry *planEntry) (plan.NodePlan, error) {
	nodePlan = addLocalClusterAuthenticationEndpointFile(nodePlan, controlPlane, entry)
	return nodePlan, nil
}

func restartStamp(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane, image string) string {
	restartStamp := sha256.New()
	restartStamp.Write([]byte(strconv.Itoa(controlPlane.Spec.ProvisionGeneration)))
	restartStamp.Write([]byte(image))
	for _, file := range nodePlan.Files {
		if file.Dynamic {
			continue
		}
		restartStamp.Write([]byte(file.Path))
		restartStamp.Write([]byte(file.Content))
	}
	restartStamp.Write([]byte(strconv.FormatInt(controlPlane.Status.ConfigGeneration, 10)))
	return hex.EncodeToString(restartStamp.Sum(nil))
}

func addToken(config map[string]interface{}, entry *planEntry, tokensSecret plan.Secret) {
	if tokensSecret.ServerToken == "" {
		return
	}
	if isOnlyWorker(entry) {
		config["token"] = tokensSecret.AgentToken
	} else {
		config["token"] = tokensSecret.ServerToken
		config["agent-token"] = tokensSecret.AgentToken
	}
}

func addAddresses(secrets corecontrollers.SecretCache, config map[string]interface{}, entry *planEntry) error {
	internalIPAddress := entry.Metadata.Annotations[rke2.InternalAddressAnnotation]
	ipAddress := entry.Metadata.Annotations[rke2.AddressAnnotation]
	internalAddressProvided, addressProvided := internalIPAddress != "", ipAddress != ""

	// If this is a provisioned node (not a custom node), then get the IP addresses from the machine driver config.
	if entry.Machine.Spec.InfrastructureRef.APIVersion == rke2.RKEMachineAPIVersion && (!internalAddressProvided || !addressProvided) {
		secret, err := secrets.Get(entry.Machine.Spec.InfrastructureRef.Namespace, rke2.MachineStateSecretName(entry.Machine.Spec.InfrastructureRef.Name))
		if apierrors.IsNotFound(err) || (secret != nil && len(secret.Data["extractedConfig"]) == 0) {
			return errIgnore(fmt.Sprintf("waiting for machine %s/%s driver config to be saved", entry.Machine.Namespace, entry.Machine.Name))
		} else if err != nil {
			return fmt.Errorf("error getting machine state secret for machine %s/%s: %w", entry.Machine.Namespace, entry.Machine.Name, err)
		}

		driverConfig, err := nodeconfig.ExtractConfigJSON(base64.StdEncoding.EncodeToString(secret.Data["extractedConfig"]))
		if err != nil || len(driverConfig) == 0 {
			return fmt.Errorf("error getting machine state JSON for machine %s/%s: %w", entry.Machine.Namespace, entry.Machine.Name, err)
		}

		if !addressProvided {
			ipAddress = convert.ToString(values.GetValueN(driverConfig, "Driver", "IPAddress"))
		}
		if !internalAddressProvided {
			internalIPAddress = convert.ToString(values.GetValueN(driverConfig, "Driver", "PrivateIPAddress"))
		}
	}

	setNodeExternalIP := ipAddress != "" && internalIPAddress != "" && ipAddress != internalIPAddress

	if setNodeExternalIP && !isOnlyWorker(entry) {
		config["advertise-address"] = internalIPAddress
		config["tls-san"] = append(convert.ToStringSlice(config["tls-san"]), ipAddress)
	}

	if internalIPAddress != "" {
		config["node-ip"] = append(convert.ToStringSlice(config["node-ip"]), internalIPAddress)
	}

	// Cloud provider, if set, will handle external IP
	if convert.ToString(config["cloud-provider-name"]) == "" && (addressProvided || setNodeExternalIP) {
		config["node-external-ip"] = append(convert.ToStringSlice(config["node-external-ip"]), ipAddress)
	}

	return nil
}

func addLabels(config map[string]interface{}, entry *planEntry) error {
	var labels []string
	if data := entry.Metadata.Annotations[rke2.LabelsAnnotation]; data != "" {
		labelMap := map[string]string{}
		if err := json.Unmarshal([]byte(data), &labelMap); err != nil {
			return err
		}
		for k, v := range labelMap {
			labels = append(labels, fmt.Sprintf("%s=%s", k, v))
		}
	}

	labels = append(labels, rke2.MachineUIDLabel+"="+string(entry.Machine.UID))
	sort.Strings(labels)
	if len(labels) > 0 {
		config["node-label"] = labels
	}
	return nil
}

func addTaints(config map[string]interface{}, entry *planEntry, runtime string) error {
	var (
		taintString []string
	)

	taints, err := getTaints(entry, runtime)
	if err != nil {
		return err
	}

	for _, taint := range taints {
		if taint.Key != "" {
			taintString = append(taintString, taint.ToString())
		}
	}

	sort.Strings(taintString)
	config["node-taint"] = taintString

	return nil
}

// retrieveClusterAuthorizedSecret accepts a secret and a cluster name, and checks if a cluster is authorized to use the secret
// by looking at the 'v2prov-secret-authorized-for-cluster' annotation and determining if it is equal to the cluster name.
// if the cluster is authorized to use the secret, the contents of the 'credential' key are returned as a byte slice
func retrieveClusterAuthorizedSecret(secret *v1.Secret, clusterName string) ([]byte, error) {
	specifiedClusterName, ownerFound := secret.Annotations[secretmigrator.AuthorizedSecretAnnotation]
	if !ownerFound || specifiedClusterName != clusterName {
		return nil, fmt.Errorf("the secret 'secret://%s:%s' provided within the cloud-provider-config does not belong to cluster '%s'", secret.Namespace, secret.Name, clusterName)
	}

	secretContent, configFound := secret.Data["credential"]
	if !configFound {
		return nil, fmt.Errorf("the cloud-provider-config specified a secret, but no config could be found within the secret 'secret://%s:%s'", secret.Namespace, secret.Name)
	}
	return secretContent, nil
}

func checkForSecretFormat(configValue string) (bool, string, string, error) {
	if strings.HasPrefix(configValue, "secret://") {
		configValue = strings.ReplaceAll(configValue, "secret://", "")
		namespaceAndName := strings.Split(configValue, ":")
		if len(namespaceAndName) != 2 || namespaceAndName[0] == "" || namespaceAndName[1] == "" {
			return true, "", "", fmt.Errorf("provided value for cloud-provider-config secret is malformed, must be of the format secret://namespace:name")
		}
		return true, namespaceAndName[0], namespaceAndName[1], nil
	}
	return false, "", "", nil
}

// configFile renders the full path to a config file based on the passed in filename and controlPlane
// If the desired filename does not have a defined path template in the `filePaths` map, the function will fall back
// to rendering a filepath based on `/var/lib/rancher/%s/etc/config-files/%s` where the first %s is the runtime and
// second %s is the filename.
func configFile(controlPlane *rkev1.RKEControlPlane, filename string) string {
	if path := filePaths[filename]; path != "" {
		if strings.Contains(path, "%s") {
			return fmt.Sprintf(path, rke2.GetRuntime(controlPlane.Spec.KubernetesVersion))
		}
		return path
	}
	return fmt.Sprintf("/var/lib/rancher/%s/etc/config-files/%s",
		rke2.GetRuntime(controlPlane.Spec.KubernetesVersion), filename)
}

func (p *Planner) addConfigFile(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane, entry *planEntry, tokensSecret plan.Secret,
	initNode bool, joinServer string) (plan.NodePlan, map[string]interface{}, error) {
	config := map[string]interface{}{}

	addDefaults(config, controlPlane)

	// Must call addUserConfig first because it will filter out non-kdm data
	if err := addUserConfig(config, controlPlane, entry); err != nil {
		return nodePlan, config, err
	}

	files, err := p.addRegistryConfig(config, controlPlane)
	if err != nil {
		return nodePlan, config, err
	}
	nodePlan.Files = append(nodePlan.Files, files...)

	files, err = p.addETCD(config, controlPlane, entry)
	if err != nil {
		return nodePlan, config, err
	}
	nodePlan.Files = append(nodePlan.Files, files...)

	addRoleConfig(config, controlPlane, entry, initNode, joinServer)
	addLocalClusterAuthenticationEndpointConfig(config, controlPlane, entry)
	addToken(config, entry, tokensSecret)
	if err := addAddresses(p.secretCache, config, entry); err != nil {
		return nodePlan, config, err
	}
	if err := addLabels(config, entry); err != nil {
		return nodePlan, config, err
	}

	runtime := rke2.GetRuntime(controlPlane.Spec.KubernetesVersion)
	if err := addTaints(config, entry, runtime); err != nil {
		return nodePlan, config, err
	}

	for _, fileParam := range fileParams {
		content, ok := config[fileParam]
		if !ok {
			continue
		}

		if fileParam == "cloud-provider-config" {
			isSecretFormat, namespace, name, err := checkForSecretFormat(convert.ToString(content))
			if err != nil {
				// provided secret for cloud-provider-config does not follow the format of
				// secret://namespace:name
				return nodePlan, config, err
			}
			if isSecretFormat {
				secret, err := p.secretCache.Get(namespace, name)
				if err != nil {
					return nodePlan, config, err
				}

				secretContent, err := retrieveClusterAuthorizedSecret(secret, controlPlane.Name)
				if err != nil {
					return nodePlan, config, err
				}

				filePath := configFile(controlPlane, fileParam)
				config[fileParam] = filePath
				nodePlan.Files = append(nodePlan.Files, plan.File{
					Content: base64.StdEncoding.EncodeToString(secretContent),
					Path:    filePath,
				})
				continue
			}
		}

		filePath := configFile(controlPlane, fileParam)
		config[fileParam] = filePath

		nodePlan.Files = append(nodePlan.Files, plan.File{
			Content: base64.StdEncoding.EncodeToString([]byte(convert.ToString(content))),
			Path:    filePath,
		})
	}

	PruneEmpty(config)

	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nodePlan, config, err
	}

	nodePlan.Files = append(nodePlan.Files, plan.File{
		Content: base64.StdEncoding.EncodeToString(configData),
		Path:    fmt.Sprintf(ConfigYamlFileName, runtime),
	})

	return nodePlan, config, nil
}
