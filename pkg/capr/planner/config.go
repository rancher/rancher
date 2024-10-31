package planner

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/rancher/norman/types/values"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/management/secretmigrator"
	"github.com/rancher/rancher/pkg/nodeconfig"
	"github.com/rancher/rancher/pkg/provisioningv2/image"
	"github.com/rancher/wrangler/v3/pkg/data"
	"github.com/rancher/wrangler/v3/pkg/data/convert"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/kv"
	"github.com/rancher/wrangler/v3/pkg/yaml"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

// addEtcd mutates the given config map with etcd-specific configuration elements, and adds S3-related arguments and files if renderS3 is true.
func (p *Planner) addETCD(config map[string]interface{}, controlPlane *rkev1.RKEControlPlane, entry *planEntry, renderS3 bool) (result []plan.File, _ error) {
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

	if renderS3 {
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
	}

	return
}

func addDefaults(config map[string]interface{}, controlPlane *rkev1.RKEControlPlane) {
	if capr.GetRuntime(controlPlane.Spec.KubernetesVersion) == capr.RuntimeRKE2 {
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

	if err := filterConfigData(config, controlPlane, entry); err != nil {
		return err
	}

	// "data-dir" is explicitly not added to KDM for filtering because it is mapped to a field in the provisioning cluster
	// CRD. While technically possible to add feature gates and update KDM, there is nothing to be gained from such an
	// approach as the "data-dir" implementation will likely never change distro-side.
	if controlPlane.Spec.DataDirectories.K8sDistro != "" {
		config["data-dir"] = controlPlane.Spec.DataDirectories.K8sDistro
	}

	return nil
}

// addRoleConfig adds the role config to the passed in map, and returns the join server that the config was rendered for.
// It will return "-" as the join server if the entry is an init node (the init node should not join a server)
func addRoleConfig(config map[string]interface{}, controlPlane *rkev1.RKEControlPlane, entry *planEntry, joinServer string) string {
	runtime := capr.GetRuntime(controlPlane.Spec.KubernetesVersion)
	if isInitNode(entry) {
		// If this node is the init node, it should not be joined to anything. Clear the joinServer URL.
		if runtime == capr.RuntimeK3S {
			config["cluster-init"] = true
		}
		joinServer = "-"
	} else if joinServer == "" {
		// If no join server was specified, use the join server annotation for the node.
		var ok bool
		joinServer, ok = entry.Metadata.Annotations[capr.JoinedToAnnotation]
		if !ok {
			return capr.JoinServerImplausible
		}
	}

	if joinServer != "" && joinServer != "-" {
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

	if pr := image.GetPrivateRepoURLFromControlPlane(controlPlane); pr != "" && !isOnlyWorker(entry) {
		config["system-default-registry"] = pr
	}

	// If this is a control-plane node, then we need to set arguments/(and for RKE2, volume mounts) to allow probes
	// to run.
	if isControlPlane(entry) {
		logrus.Debug("addRoleConfig rendering arguments and mounts for kube-controller-manager")
		certDirArg, certDirMount := renderArgAndMount(config[KubeControllerManagerArg], config[KubeControllerManagerExtraMount], controlPlane, DefaultKubeControllerManagerDefaultSecurePort, DefaultKubeControllerManagerCertDir)
		config[KubeControllerManagerArg] = certDirArg
		if runtime == capr.RuntimeRKE2 {
			config[KubeControllerManagerExtraMount] = certDirMount
		}

		logrus.Debug("addRoleConfig rendering arguments and mounts for kube-scheduler")
		certDirArg, certDirMount = renderArgAndMount(config[KubeSchedulerArg], config[KubeSchedulerExtraMount], controlPlane, DefaultKubeSchedulerDefaultSecurePort, DefaultKubeSchedulerCertDir)
		config[KubeSchedulerArg] = certDirArg
		if runtime == capr.RuntimeRKE2 {
			config[KubeSchedulerExtraMount] = certDirMount
		}
	}

	if nodeName := entry.Metadata.Labels[capr.NodeNameLabel]; nodeName != "" {
		config["node-name"] = nodeName
	}
	return joinServer
}

func addLocalClusterAuthenticationEndpointConfig(config map[string]interface{}, controlPlane *rkev1.RKEControlPlane, entry *planEntry) {
	if isOnlyWorker(entry) || !controlPlane.Spec.LocalClusterAuthEndpoint.Enabled {
		return
	}

	authFile := path.Join(capr.GetDistroDataDir(controlPlane), authnWebhookFileName)
	config["kube-apiserver-arg"] = append(convert.ToStringSlice(config["kube-apiserver-arg"]),
		fmt.Sprintf("authentication-token-webhook-config-file=%s", authFile))
}

func addLocalClusterAuthenticationEndpointFile(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane, entry *planEntry) plan.NodePlan {
	if isOnlyWorker(entry) || !controlPlane.Spec.LocalClusterAuthEndpoint.Enabled {
		return nodePlan
	}

	loopbackAddress := capr.GetLoopbackAddress(controlPlane)
	authFile := path.Join(capr.GetDistroDataDir(controlPlane), authnWebhookFileName)
	nodePlan.Files = append(nodePlan.Files, plan.File{
		Content: base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(AuthnWebhook, loopbackAddress))),
		Path:    authFile,
	})

	return nodePlan
}

func (p *Planner) addManifests(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane, entry *planEntry) (plan.NodePlan, error) {
	bootstrapManifests, err := p.retrievalFunctions.GetBootstrapManifests(controlPlane)
	if err != nil {
		return nodePlan, err
	}

	if len(bootstrapManifests) > 0 {
		logrus.Debugf("[planner] adding pre-bootstrap manifests")
		nodePlan.Files = append(nodePlan.Files, bootstrapManifests...)
		return nodePlan, err
	}

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

type helmChartConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec helmChartConfigSpec `json:"spec,omitempty"`
}

type helmChartConfigSpec struct {
	ValuesContent string `json:"valuesContent,omitempty"`
}

func (h *helmChartConfig) DeepCopyObject() runtime.Object {
	panic("unsupported")
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
	for _, chart := range capr.SortedKeys(chartValues) {
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
		Path:    path.Join(capr.GetDistroDataDir(controlPlane), "server/manifests/rancher/managed-chart-config.yaml"),
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
	internalIPAddress := entry.Metadata.Annotations[capr.InternalAddressAnnotation]
	ipAddress := entry.Metadata.Annotations[capr.AddressAnnotation]
	internalAddressProvided, addressProvided := internalIPAddress != "", ipAddress != ""

	// If this is a provisioned node (not a custom node), then get the IP addresses from the machine driver config.
	if entry.Machine.Spec.InfrastructureRef.APIVersion == capr.RKEMachineAPIVersion && (!internalAddressProvided || !addressProvided) {
		secret, err := secrets.Get(entry.Machine.Spec.InfrastructureRef.Namespace, capr.MachineStateSecretName(entry.Machine.Spec.InfrastructureRef.Name))
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
	if data := entry.Metadata.Annotations[capr.LabelsAnnotation]; data != "" {
		labelMap := map[string]string{}
		if err := json.Unmarshal([]byte(data), &labelMap); err != nil {
			return err
		}
		for k, v := range labelMap {
			labels = append(labels, fmt.Sprintf("%s=%s", k, v))
		}
	}

	labels = append(labels, capr.MachineUIDLabel+"="+string(entry.Machine.UID))
	sort.Strings(labels)
	if len(labels) > 0 {
		config["node-label"] = labels
	}
	return nil
}

func addTaints(config map[string]interface{}, entry *planEntry, cp *rkev1.RKEControlPlane) error {
	var (
		taintString []string
	)

	taints, err := getTaints(entry, cp)
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
	authorized, ownerFound := clusterObjectAuthorized(secret, secretmigrator.AuthorizedSecretAnnotation, clusterName)
	if !ownerFound || !authorized {
		return nil, fmt.Errorf("the secret 'secret://%s:%s' provided within the cloud-provider-config does not belong to cluster '%s'", secret.Namespace, secret.Name, clusterName)
	}

	secretContent, configFound := secret.Data["credential"]
	if !configFound {
		return nil, fmt.Errorf("the cloud-provider-config specified a secret, but no config could be found within the secret 'secret://%s:%s'", secret.Namespace, secret.Name)
	}
	return secretContent, nil
}

// clusterObjectAuthorized accepts any object, and inspects the metadata.Annotations of the object for the specified annotation
// and determines if the object has authorized the cluster to access it. It returns two booleans, the first being whether the
// cluster is authorized to access the object and the second being whether the annotation and a corresponding value were found
// on the object
func clusterObjectAuthorized(obj runtime.Object, annotation, clusterName string) (bool, bool) {
	annotationValueFound := false
	if obj == nil || annotation == "" || clusterName == "" {
		return false, annotationValueFound
	}
	copiedObj := obj.DeepCopyObject()
	if objMeta, err := meta.Accessor(copiedObj); err == nil && objMeta != nil {
		authorizedClusters := strings.Split(objMeta.GetAnnotations()[annotation], ",")
		if len(authorizedClusters) > 0 {
			annotationValueFound = true
		}
		for _, authorizedCluster := range authorizedClusters {
			if clusterName == authorizedCluster {
				return true, annotationValueFound
			}
		}
	}
	return false, annotationValueFound
}

func checkForSecretFormat(secretFieldName, configValue string) (bool, string, string, error) {
	if strings.HasPrefix(configValue, "secret://") {
		configValue = strings.ReplaceAll(configValue, "secret://", "")
		namespaceAndName := strings.Split(configValue, ":")
		if len(namespaceAndName) != 2 || namespaceAndName[0] == "" || namespaceAndName[1] == "" {
			return true, "", "", fmt.Errorf("provided value for %s secret is malformed, must be of the format secret://namespace:name", secretFieldName)
		}
		return true, namespaceAndName[0], namespaceAndName[1], nil
	}
	return false, "", "", nil
}

// configFile renders the full path to a config file based on the passed in filename and controlPlane
// If the desired filename does not have a defined path template in the `filePaths` map, the function will fall back
// to rendering a filepath based on `%s/etc/config-files/%s` where the first %s is the data-dir and
// second %s is the filename.
func configFile(controlPlane *rkev1.RKEControlPlane, filename string) string {
	if path := filePaths[filename]; path != "" {
		if strings.Contains(path, "%s") {
			return fmt.Sprintf(path, capr.GetRuntime(controlPlane.Spec.KubernetesVersion))
		}
		return path
	}
	return path.Join(capr.GetDistroDataDir(controlPlane), "etc/config-files", filename)
}

func (p *Planner) renderFiles(controlPlane *rkev1.RKEControlPlane, entry *planEntry) ([]plan.File, error) {
	var files []plan.File
	for _, msf := range controlPlane.Spec.MachineSelectorFiles {
		sel, err := metav1.LabelSelectorAsSelector(msf.MachineLabelSelector)
		if err != nil {
			return files, err
		}
		if msf.MachineLabelSelector != nil && !sel.Matches(labels.Set(entry.Machine.Labels)) {
			continue
		}
		for _, fs := range msf.FileSources {
			if fs.Secret.Name != "" && fs.ConfigMap.Name != "" {
				return files, fmt.Errorf("secret %s/%s and configmap %s/%s cannot both be defined at the same time for files, use separate entries", controlPlane.Namespace, fs.Secret.Name, controlPlane.Namespace, fs.ConfigMap.Name)
			}
			if fs.Secret.Name != "" {
				// retrieve secret and auth then use contents
				secret, err := p.secretCache.Get(controlPlane.Namespace, fs.Secret.Name)
				if err != nil {
					return files, fmt.Errorf("error retrieving secret %s/%s while rendering files: %v", controlPlane.Namespace, fs.Secret.Name, err)
				}
				if authorized, found := clusterObjectAuthorized(secret, capr.AuthorizedObjectAnnotation, controlPlane.Name); authorized && found {
					for _, v := range fs.Secret.Items {
						file := plan.File{
							Path:    v.Path,
							Content: base64.StdEncoding.EncodeToString(secret.Data[v.Key]),
							Dynamic: v.Dynamic,
						}
						hash := sha256.Sum256(secret.Data[v.Key])
						if v.Hash != "" && v.Hash != base64.StdEncoding.EncodeToString(hash[:]) {
							return files, fmt.Errorf("secret %s does not cotain the expected content", secret.Name)
						}
						if v.Permissions != "" {
							file.Permissions = v.Permissions
						} else if fs.Secret.DefaultPermissions != "" {
							file.Permissions = fs.Secret.DefaultPermissions
						}
						files = append(files, file)
					}
				} else {
					return files, fmt.Errorf("error rendering files: cluster %s/%s was not authorized to access secret %s/%s", controlPlane.Namespace, controlPlane.Name, controlPlane.Namespace, fs.Secret.Name)
				}
			}
			if fs.ConfigMap.Name != "" {
				configmap, err := p.configMapCache.Get(controlPlane.Namespace, fs.ConfigMap.Name)
				if err != nil {
					return files, fmt.Errorf("error retrieving configmap %s/%s while rendering files: %v", controlPlane.Namespace, fs.ConfigMap.Name, err)
				}
				// retrieve configmap and use contents
				if authorized, found := clusterObjectAuthorized(configmap, capr.AuthorizedObjectAnnotation, controlPlane.Name); authorized && found {
					for _, v := range fs.ConfigMap.Items {
						file := plan.File{
							Path:    v.Path,
							Content: base64.StdEncoding.EncodeToString([]byte(configmap.Data[v.Key])),
							Dynamic: v.Dynamic,
						}
						hash := sha256.Sum256([]byte(configmap.Data[v.Key]))
						if v.Hash != "" && v.Hash != base64.StdEncoding.EncodeToString(hash[:]) {
							return files, fmt.Errorf("configmap %s does not cotain the expected content", configmap.Name)
						}
						if v.Permissions != "" {
							file.Permissions = v.Permissions
						} else if fs.ConfigMap.DefaultPermissions != "" {
							file.Permissions = fs.ConfigMap.DefaultPermissions
						}
						files = append(files, file)
					}
				} else {
					return files, fmt.Errorf("error rendering files: cluster %s/%s was not authorized to access configmap %s/%s", controlPlane.Namespace, controlPlane.Name, controlPlane.Namespace, fs.ConfigMap.Name)
				}
			}
		}
	}
	return files, nil
}

// addConfigFile will render the distribution configuration file and add it to the nodePlan. It also renders files that
// are referenced in the distribution configuration file (for example, ACE and the cloud-provider). It returns the updated
// NodePlan, the config that was rendered in a map, the joined server, and an error if one exists.
// NOTE: the joined server can be "-" if the config file is being added for the init node.
func (p *Planner) addConfigFile(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane, entry *planEntry, tokensSecret plan.Secret,
	joinServer string, reg registries, renderS3 bool) (plan.NodePlan, map[string]interface{}, string, error) {
	config := map[string]interface{}{}

	addDefaults(config, controlPlane)

	// Must call addUserConfig first because it will filter out non-kdm data
	if err := addUserConfig(config, controlPlane, entry); err != nil {
		return nodePlan, config, "", err
	}

	files, err := p.addETCD(config, controlPlane, entry, renderS3)
	if err != nil {
		return nodePlan, config, "", err
	}
	nodePlan.Files = append(nodePlan.Files, files...)

	joinedServer := addRoleConfig(config, controlPlane, entry, joinServer)
	if joinedServer == capr.JoinServerImplausible {
		return nodePlan, config, "", fmt.Errorf("implausible joined server for entry")
	}

	addLocalClusterAuthenticationEndpointConfig(config, controlPlane, entry)
	addToken(config, entry, tokensSecret)

	if err := addAddresses(p.secretCache, config, entry); err != nil {
		return nodePlan, config, joinedServer, err
	}
	if err := addLabels(config, entry); err != nil {
		return nodePlan, config, joinedServer, err
	}
	if err := addTaints(config, entry, controlPlane); err != nil {
		return nodePlan, config, joinedServer, err
	}

	for _, fileParam := range fileParams {
		var content interface{}
		if fileParam == privateRegistryArg {
			content = string(reg.registriesFileRaw)
		} else {
			var ok bool
			content, ok = config[fileParam]
			if !ok {
				continue
			}
		}

		if fileParam == cloudProviderConfigArg {
			isSecretFormat, namespace, name, err := checkForSecretFormat(cloudProviderConfigArg, convert.ToString(content))
			if err != nil {
				// provided secret for cloud-provider-config does not follow the format of
				// secret://namespace:name
				return nodePlan, config, joinedServer, err
			}
			if isSecretFormat {
				secret, err := p.secretCache.Get(namespace, name)
				if err != nil {
					return nodePlan, config, joinedServer, err
				}

				secretContent, err := retrieveClusterAuthorizedSecret(secret, controlPlane.Name)
				if err != nil {
					return nodePlan, config, joinedServer, err
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

	files, err = p.renderFiles(controlPlane, entry)
	if err != nil {
		return nodePlan, config, joinedServer, err
	}

	nodePlan.Files = append(nodePlan.Files, files...)

	PruneEmpty(config)

	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nodePlan, config, joinedServer, err
	}

	nodePlan.Files = append(nodePlan.Files, plan.File{
		Content: base64.StdEncoding.EncodeToString(configData),
		Path:    fmt.Sprintf(ConfigYamlFileName, capr.GetRuntime(controlPlane.Spec.KubernetesVersion)),
	})

	return nodePlan, config, joinedServer, nil
}
