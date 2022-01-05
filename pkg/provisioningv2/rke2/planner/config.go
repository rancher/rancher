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

	"github.com/rancher/norman/types/values"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/nodeconfig"
	rancherruntime "github.com/rancher/rancher/pkg/provisioningv2/rke2/runtime"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/data/convert"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/kv"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/rancher/wrangler/pkg/yaml"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

func (p *Planner) addETCD(config map[string]interface{}, controlPlane *rkev1.RKEControlPlane, machine *capi.Machine) (result []plan.File, _ error) {
	if !isEtcd(machine) || controlPlane.Spec.ETCD == nil {
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

	args, _, files, err := p.etcdS3Args.ToArgs(controlPlane.Spec.ETCD.S3, controlPlane)
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

func addDefaults(config map[string]interface{}, controlPlane *rkev1.RKEControlPlane, machine *capi.Machine) {
	if rancherruntime.GetRuntime(controlPlane.Spec.KubernetesVersion) == rancherruntime.RuntimeRKE2 {
		config["cni"] = "calico"
	}
	if settings.SystemDefaultRegistry.Get() != "" {
		config["system-default-registry"] = settings.SystemDefaultRegistry.Get()
	}
}

func addUserConfig(config map[string]interface{}, controlPlane *rkev1.RKEControlPlane, machine *capi.Machine) error {
	for k, v := range controlPlane.Spec.MachineGlobalConfig.Data {
		config[k] = v
	}

	for _, opts := range controlPlane.Spec.MachineSelectorConfig {
		sel, err := metav1.LabelSelectorAsSelector(opts.MachineLabelSelector)
		if err != nil {
			return err
		}
		if opts.MachineLabelSelector == nil || sel.Matches(labels.Set(machine.Labels)) {
			for k, v := range opts.Config.Data {
				config[k] = v
			}
		}
	}

	filterConfigData(config, controlPlane, machine)
	return nil
}

func addRoleConfig(config map[string]interface{}, controlPlane *rkev1.RKEControlPlane, machine *capi.Machine, initNode bool, joinServer string) {
	runtime := rancherruntime.GetRuntime(controlPlane.Spec.KubernetesVersion)
	if initNode {
		if runtime == rancherruntime.RuntimeK3S {
			config["cluster-init"] = true
		}
	} else if joinServer != "" {
		// it's very important that the joinServer param isn't used on the initNode. The init node is special
		// because it will be evaluated twice, first with joinServer = "" and then with joinServer == self.
		// If we use the joinServer param then we will get different nodePlan and cause issues.
		config["server"] = joinServer
	}

	if IsOnlyEtcd(machine) {
		config["disable-scheduler"] = true
		config["disable-apiserver"] = true
		config["disable-controller-manager"] = true
	} else if isOnlyControlPlane(machine) {
		config["disable-etcd"] = true
	}

	// If this is a control-plane node, then we need to set arguments/(and for RKE2, volume mounts) to allow probes
	// to run.
	if isControlPlane(machine) {
		logrus.Debug("addRoleConfig rendering arguments and mounts for kube-controller-manager")
		certDirArg, certDirMount := renderArgAndMount(config[KubeControllerManagerArg], config[KubeControllerManagerExtraMount], runtime, DefaultKubeControllerManagerDefaultSecurePort, DefaultKubeControllerManagerCertDir)
		config[KubeControllerManagerArg] = certDirArg
		if runtime == rancherruntime.RuntimeRKE2 {
			config[KubeControllerManagerExtraMount] = certDirMount
		}

		logrus.Debug("addRoleConfig rendering arguments and mounts for kube-scheduler")
		certDirArg, certDirMount = renderArgAndMount(config[KubeSchedulerArg], config[KubeSchedulerExtraMount], runtime, DefaultKubeSchedulerDefaultSecurePort, DefaultKubeSchedulerCertDir)
		config[KubeSchedulerArg] = certDirArg
		if runtime == rancherruntime.RuntimeRKE2 {
			config[KubeSchedulerExtraMount] = certDirMount
		}
	}

	if nodeName := machine.Labels[NodeNameLabel]; nodeName != "" {
		config["node-name"] = nodeName
	}
}

func addLocalClusterAuthenticationEndpointConfig(config map[string]interface{}, controlPlane *rkev1.RKEControlPlane,
	machine *capi.Machine) {
	if isOnlyWorker(machine) || !controlPlane.Spec.LocalClusterAuthEndpoint.Enabled {
		return
	}

	authFile := fmt.Sprintf(authnWebhookFileName, rancherruntime.GetRuntime(controlPlane.Spec.KubernetesVersion))
	config["kube-apiserver-arg"] = append(convert.ToStringSlice(config["kube-apiserver-arg"]),
		fmt.Sprintf("authentication-token-webhook-config-file=%s", authFile))
}

func addLocalClusterAuthenticationEndpointFile(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane,
	machine *capi.Machine) plan.NodePlan {
	if isOnlyWorker(machine) || !controlPlane.Spec.LocalClusterAuthEndpoint.Enabled {
		return nodePlan
	}

	authFile := fmt.Sprintf(authnWebhookFileName, rancherruntime.GetRuntime(controlPlane.Spec.KubernetesVersion))
	nodePlan.Files = append(nodePlan.Files, plan.File{
		Content: base64.StdEncoding.EncodeToString(AuthnWebhook),
		Path:    authFile,
	})

	return nodePlan
}

func (p *Planner) addManifests(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane, machine *capi.Machine) (plan.NodePlan, error) {
	files, err := p.getControlPlaneManifests(controlPlane, machine)
	if err != nil {
		return nodePlan, err
	}
	nodePlan.Files = append(nodePlan.Files, files...)

	return nodePlan, nil
}

func isVSphereProvider(controlPlane *rkev1.RKEControlPlane, machine *capi.Machine) (bool, error) {
	data := map[string]interface{}{}
	if err := addUserConfig(data, controlPlane, machine); err != nil {
		return false, err
	}
	return data["cloud-provider-name"] == "rancher-vsphere", nil
}

func addVSphereCharts(controlPlane *rkev1.RKEControlPlane, machine *capi.Machine) (map[string]interface{}, error) {
	if isVSphere, err := isVSphereProvider(controlPlane, machine); err != nil {
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

func (p *Planner) addChartConfigs(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane,
	machine *capi.Machine) (plan.NodePlan, error) {
	if isOnlyWorker(machine) {
		return nodePlan, nil
	}

	chartValues, err := addVSphereCharts(controlPlane, machine)
	if err != nil {
		return nodePlan, err
	}

	var chartConfigs []runtime.Object
	for chart, values := range chartValues {
		valuesMap := convert.ToMapInterface(values)
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
		Path:    fmt.Sprintf("/var/lib/rancher/%s/server/manifests/rancher/managed-chart-config.yaml", rancherruntime.GetRuntime(controlPlane.Spec.KubernetesVersion)),
		Dynamic: true,
	})

	return nodePlan, nil
}

func addOtherFiles(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane,
	machine *capi.Machine) (plan.NodePlan, error) {
	nodePlan = addLocalClusterAuthenticationEndpointFile(nodePlan, controlPlane, machine)
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

func addToken(config map[string]interface{}, machine *capi.Machine, secret plan.Secret) {
	if secret.ServerToken == "" {
		return
	}
	if isOnlyWorker(machine) {
		config["token"] = secret.AgentToken
	} else {
		config["token"] = secret.ServerToken
		config["agent-token"] = secret.AgentToken
	}
}

func addAddresses(secrets corecontrollers.SecretCache, config map[string]interface{}, machine *capi.Machine) {
	internalIPAddress := machine.Annotations[InternalAddressAnnotation]
	ipAddress := machine.Annotations[AddressAnnotation]
	internalAddressProvided, addressProvided := internalIPAddress != "", ipAddress != ""

	secret, err := secrets.Get(machine.Spec.InfrastructureRef.Namespace, name.SafeConcatName(machine.Spec.InfrastructureRef.Name, "machine", "state"))
	if err == nil && len(secret.Data["extractedConfig"]) != 0 {
		driverConfig, err := nodeconfig.ExtractConfigJSON(base64.StdEncoding.EncodeToString(secret.Data["extractedConfig"]))
		if err == nil && len(driverConfig) != 0 {
			if !addressProvided {
				ipAddress = convert.ToString(values.GetValueN(driverConfig, "Driver", "IPAddress"))
			}
			if !internalAddressProvided {
				internalIPAddress = convert.ToString(values.GetValueN(driverConfig, "Driver", "PrivateIPAddress"))
			}
		}
	}

	setNodeExternalIP := ipAddress != "" && internalIPAddress != "" && ipAddress != internalIPAddress

	if setNodeExternalIP && !isOnlyWorker(machine) {
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
}

func addLabels(config map[string]interface{}, machine *capi.Machine) error {
	var labels []string
	if data := machine.Annotations[LabelsAnnotation]; data != "" {
		labelMap := map[string]string{}
		if err := json.Unmarshal([]byte(data), &labelMap); err != nil {
			return err
		}
		for k, v := range labelMap {
			labels = append(labels, fmt.Sprintf("%s=%s", k, v))
		}
	}

	labels = append(labels, MachineUIDLabel+"="+string(machine.UID))
	sort.Strings(labels)
	if len(labels) > 0 {
		config["node-label"] = labels
	}
	return nil
}

func addTaints(config map[string]interface{}, machine *capi.Machine, runtime string) error {
	var (
		taintString []string
	)

	taints, err := getTaints(machine, runtime)
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

// configFile renders the full path to a config file based on the passed in filename and controlPlane
// If the desired filename does not have a defined path template in the `filePaths` map, the function will fall back
// to rendering a filepath based on `/var/lib/rancher/%s/etc/config-files/%s` where the first %s is the runtime and
// second %s is the filename.
func configFile(controlPlane *rkev1.RKEControlPlane, filename string) string {
	if path := filePaths[filename]; path != "" {
		if strings.Contains(path, "%s") {
			return fmt.Sprintf(path, rancherruntime.GetRuntime(controlPlane.Spec.KubernetesVersion))
		}
		return path
	}
	return fmt.Sprintf("/var/lib/rancher/%s/etc/config-files/%s",
		rancherruntime.GetRuntime(controlPlane.Spec.KubernetesVersion), filename)
}

func (p *Planner) addConfigFile(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane, machine *capi.Machine, secret plan.Secret,
	initNode bool, joinServer string) (plan.NodePlan, map[string]interface{}, error) {
	config := map[string]interface{}{}

	addDefaults(config, controlPlane, machine)

	// Must call addUserConfig first because it will filter out non-kdm data
	if err := addUserConfig(config, controlPlane, machine); err != nil {
		return nodePlan, config, err
	}

	files, err := p.addRegistryConfig(config, controlPlane)
	if err != nil {
		return nodePlan, config, err
	}
	nodePlan.Files = append(nodePlan.Files, files...)

	files, err = p.addETCD(config, controlPlane, machine)
	if err != nil {
		return nodePlan, config, err
	}
	nodePlan.Files = append(nodePlan.Files, files...)

	addRoleConfig(config, controlPlane, machine, initNode, joinServer)
	addLocalClusterAuthenticationEndpointConfig(config, controlPlane, machine)
	addToken(config, machine, secret)
	addAddresses(p.secretCache, config, machine)

	if err := addLabels(config, machine); err != nil {
		return nodePlan, config, err
	}

	runtime := rancherruntime.GetRuntime(controlPlane.Spec.KubernetesVersion)
	if err := addTaints(config, machine, runtime); err != nil {
		return nodePlan, config, err
	}

	for _, fileParam := range fileParams {
		content, ok := config[fileParam]
		if !ok {
			continue
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
