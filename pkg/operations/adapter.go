package operations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/rancher/channelserver/pkg/model"
	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/data/convert"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

// Adapter is an interface for different types of cluster objects.
// the Adapter interface exposes all methods required for constructing a node plan for the supported types.
// The adapter currently supports v2prov, CAPR and imported clusters.
type Adapter interface {
	// WaitForRegister waits for all machine-plan secrets to be created, ensuring the system-agent has checked in for
	// all expected nodes.
	WaitForRegister() (bool, error)

	PauseCluster(bool) error

	// RuntimeCommand returns the command used to interact with the distro CLI (RKe2/K3s).
	RuntimeCommand() string

	DataDirectory(*corev1.Secret) string

	// ProvisioningDataDirectory returns the absolute path to the provisioning data directory on the node.
	// This is where the idempotent action script and its tracking state live.
	ProvisioningDataDirectory() string

	// ServerUnit returns the systemd unit name for a distro server node.
	ServerUnit() string

	// RenderProbes renders the probes for a given machine-plan secret based on its role.
	RenderProbes(*corev1.Secret) (map[string]plan.Probe, error)

	KubectlPath(*corev1.Secret) string

	KubeconfigPath(*corev1.Secret) string

	// ElectLeader picks the most suitable machine-plan secret to lead operations for the given
	// role(s) within the given namespace. Returns nil if no eligible candidate holds all requested
	// roles.
	//
	// Adapters apply implementation-specific eligibility (CAPR: CAPI machine not deleting;
	// imported: v3.Node not deleting) and init-node detection (CAPR: capr.InitNodeLabel; imported:
	// parsed from v3.Node.Status.NodeAnnotations server args). The shared preference order is:
	//   1. Init candidate holding all requested roles
	//   2. Candidate whose role set EXACTLY matches the requested roles
	//   3. Candidate whose role set is a strict superset of the requested roles
	// Tied candidates within a tier are broken lexicographically by secret name, so re-elections
	// against the same inputs do not reshuffle leaders.
	ElectLeader(role LeaderRole, namespace string) (*corev1.Secret, error)
}

// NewAdapter returns an Adapter for the given cluster object.
// For Provisioning clusters the controlPlane object is extracted and then a CAPR Adapter is used to prevent duplication.
// The wrangler.CAPIContext is used in order to allow the adapter to access specific typed caches for ease of use.
func NewAdapter(clients *wrangler.CAPIContext, ustr *unstructured.Unstructured) (Adapter, error) {
	if ustr == nil {
		return nil, errors.New("nil unstructured")
	}
	// controlplane and provisioning cluster always have the same name
	if (ustr.GetAPIVersion() == "rke.cattle.io/v1" && ustr.GetKind() == "RKEControlPlane") ||
		(ustr.GetAPIVersion() == "provisioning.cattle.io/v1" && ustr.GetKind() == "Cluster") {
		controlPlane, err := clients.RKE.RKEControlPlane().Cache().Get(ustr.GetNamespace(), ustr.GetName())
		if err != nil {
			return nil, err
		}
		return &CAPRAdapter{
			controlPlane: controlPlane,
			clients:      clients,
		}, nil
	} else if ustr.GetAPIVersion() == "management.cattle.io/v3" && ustr.GetKind() == "Cluster" {
		var cluster *mgmtv3.Cluster
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(ustr.Object, &cluster)
		if err != nil {
			return nil, err
		}
		return &ImportedAdapter{
			cluster: cluster,
			clients: clients,
		}, nil
	}
	return nil, errors.New("unsupported object")
}

// CAPRAdapter is an implementation of the Adapter interface for CAPR clusters.
type CAPRAdapter struct {
	controlPlane *rkev1.RKEControlPlane
	clients      *wrangler.CAPIContext
}

const (
	SecurePortArgument  = "secure-port"
	CertDirArgument     = "cert-dir"
	TLSCertFileArgument = "tls-cert-file"

	KubeControllerManagerArg                      = "kube-controller-manager-arg"
	DefaultKubeControllerManagerCertDir           = "server/tls/kube-controller-manager"
	DefaultKubeControllerManagerDefaultSecurePort = "10257"
	DefaultKubeControllerManagerCert              = "kube-controller-manager.crt"

	KubeSchedulerArg                      = "kube-scheduler-arg"
	DefaultKubeSchedulerCertDir           = "server/tls/kube-scheduler"
	DefaultKubeSchedulerDefaultSecurePort = "10259"
	DefaultKubeSchedulerCert              = "kube-scheduler.crt"
)

var (
	allProbes = map[string]plan.Probe{
		"calico": {
			InitialDelaySeconds: 1,
			TimeoutSeconds:      5,
			SuccessThreshold:    1,
			FailureThreshold:    5,
			HTTPGetAction: plan.HTTPGetAction{
				URL: "http://%s:9099/liveness",
			},
		},
		"etcd": {
			InitialDelaySeconds: 1,
			TimeoutSeconds:      5,
			SuccessThreshold:    1,
			FailureThreshold:    5,
			HTTPGetAction: plan.HTTPGetAction{
				URL: "http://%s:2381/health",
			},
		},
		"kube-apiserver": {
			InitialDelaySeconds: 1,
			TimeoutSeconds:      5,
			SuccessThreshold:    1,
			FailureThreshold:    5,
			HTTPGetAction: plan.HTTPGetAction{
				URL:        "https://%s:6443/readyz",
				CACert:     "%s/server/tls/server-ca.crt",
				ClientCert: "%s/server/tls/client-kube-apiserver.crt",
				ClientKey:  "%s/server/tls/client-kube-apiserver.key",
			},
		},
		"kube-scheduler": {
			InitialDelaySeconds: 1,
			TimeoutSeconds:      5,
			SuccessThreshold:    1,
			FailureThreshold:    5,
			HTTPGetAction: plan.HTTPGetAction{
				URL: "https://%s:%s/healthz",
			},
		},
		"kube-controller-manager": {
			InitialDelaySeconds: 1,
			TimeoutSeconds:      5,
			SuccessThreshold:    1,
			FailureThreshold:    5,
			HTTPGetAction: plan.HTTPGetAction{
				URL: "https://%s:%s/healthz",
			},
		},
		"kubelet": {
			InitialDelaySeconds: 1,
			TimeoutSeconds:      5,
			SuccessThreshold:    1,
			FailureThreshold:    5,
			HTTPGetAction: plan.HTTPGetAction{
				URL: "http://%s:10248/healthz",
			},
		},
	}
	errEmptyCACert  = errors.New("cacert cannot be empty")
	errEmptyPort    = errors.New("port cannot be empty")
	errEmptyAddress = errors.New("address cannot be empty")
)

// WaitForRegister waits for all machine-plan secrets to be created, ensuring the system-agent has checked in for
// all expected nodes.
// All machine-plans secrets are listed and compared to the count of CAPI machines for the cluster.
func (a *CAPRAdapter) WaitForRegister() (bool, error) {
	secretList, err := a.clients.Core.Secret().List(a.controlPlane.Namespace, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", capr.ClusterNameLabel, a.controlPlane.Name),
		FieldSelector: fmt.Sprintf("type=%s", capr.SecretTypeMachinePlan),
	})
	if err != nil {
		return false, err
	}

	secrets := secretList.Items

	machines, err := a.clients.CAPI.Machine().Cache().List(a.controlPlane.Namespace, labels.SelectorFromSet(labels.Set{
		capr.ClusterNameLabel: a.controlPlane.Name,
	}))
	if err != nil {
		return false, err
	}

	// 1. If the counts don't match upfront, we already know it's not a 1:1 match
	if len(secrets) != len(machines) {
		return false, nil
	}

	// 2. Track the names of the machines we expect to see
	// Using a map[string]bool to easily check existence and prevent duplicates
	expectedMachines := make(map[string]bool, len(machines))
	for _, machine := range machines {
		expectedMachines[machine.Name] = true
	}

	// 3. Verify that every secret maps to a unique expected machine
	for _, secret := range secrets {
		if secret.Labels == nil {
			return false, nil
		}

		machineName, exists := secret.Labels[capr.MachineNameLabel]

		// If the label is missing, or it maps to a machine name we haven't seen/already matched
		if !exists || !expectedMachines[machineName] {
			return false, nil
		}

		// Mark this machine as "matched" by deleting it from the expected map.
		// This naturally catches duplicate secrets pointing to the same machine.
		delete(expectedMachines, machineName)
	}

	// 4. If the map is empty, we have a perfect, duplicate-free 1:1 match
	return len(expectedMachines) == 0, nil
}

// RuntimeCommand returns the command used to interact with the distro CLI (RKe2/K3s).
func (a *CAPRAdapter) RuntimeCommand() string {
	if strings.Contains(a.controlPlane.Spec.KubernetesVersion, "rke2") {
		return "rke2"
	}
	return "k3s"
}

// ServerUnit returns the systemd unit name for a distro server node.
func (a *CAPRAdapter) ServerUnit() string {
	if strings.Contains(a.controlPlane.Spec.KubernetesVersion, "rke2") {
		return "rke2-server"
	}
	return "k3s"
}

// RenderProbes renders the probes for a given machine-plan secret based on its role.
// If the cluster is using a custom data directory or secure probes, this information is extracted from the cluster object and rendered in.
func (a *CAPRAdapter) RenderProbes(secret *corev1.Secret) (map[string]plan.Probe, error) {
	var (
		runtime    = a.RuntimeCommand()
		probeNames []string
		probes     = map[string]plan.Probe{}
	)

	if runtime != capr.RuntimeK3S && IsEtcd(secret) {
		probeNames = append(probeNames, "etcd")
	}
	if IsControlPlane(secret) {
		probeNames = append(probeNames, "kube-apiserver")
		probeNames = append(probeNames, "kube-controller-manager")
		probeNames = append(probeNames, "kube-scheduler")
	}
	if !(And(IsEtcd, Not(IsControlPlane))(secret) && runtime == capr.RuntimeK3S) {
		// k3s doesn't run the kubelet on etcd only nodes
		probeNames = append(probeNames, "kubelet")
	}
	if !And(IsEtcd, Not(IsControlPlane))(secret) && isCalico(a.controlPlane, runtime) && Not(IsWindows)(secret) {
		probeNames = append(probeNames, "calico")
	}

	for _, probeName := range probeNames {
		probes[probeName] = allProbes[probeName]
	}

	dataDir := capr.GetDistroDataDir(a.controlPlane)

	probes = insertDataDirForProbes(dataDir, probes)

	loopbackAddress := capr.GetLoopbackAddress(a.controlPlane)

	config, err := a.renderConfig(secret)
	if err != nil {
		return nil, err
	}

	if IsControlPlane(secret) {
		kcmProbe, err := renderSecureProbe(config[KubeControllerManagerArg], probes["kube-controller-manager"], dataDir, loopbackAddress, DefaultKubeControllerManagerDefaultSecurePort, DefaultKubeControllerManagerCertDir, DefaultKubeControllerManagerCert)
		if err != nil {
			return probes, err
		}
		probes["kube-controller-manager"] = kcmProbe

		ksProbe, err := renderSecureProbe(config[KubeSchedulerArg], probes["kube-scheduler"], dataDir, loopbackAddress, DefaultKubeSchedulerDefaultSecurePort, DefaultKubeSchedulerCertDir, DefaultKubeSchedulerCert)
		if err != nil {
			return probes, err
		}
		probes["kube-scheduler"] = ksProbe
	}

	probes = replaceURLForProbes(probes, loopbackAddress)

	return probes, nil
}

// Note: most of this functionally has been copied from the planner 1:1.
// The intention is to split 100% of the planner code to both the planapi package and the operations package.
// This will ideally be performed before 2.15 is released; however, migrating piecemeal provides us with the flexibility
// to make changes as desired without putting the existing codebase at risk.

func (a *CAPRAdapter) renderConfig(secret *corev1.Secret) (map[string]any, error) {
	config := map[string]any{}
	if capr.GetRuntime(a.controlPlane.Spec.KubernetesVersion) == capr.RuntimeRKE2 {
		config["cni"] = "calico"
	}

	for k, v := range a.controlPlane.Spec.MachineGlobalConfig.Data {
		config[k] = v
	}

	for _, opts := range a.controlPlane.Spec.MachineSelectorConfig {
		sel, err := metav1.LabelSelectorAsSelector(opts.MachineLabelSelector)
		if err != nil {
			return nil, err
		}
		if opts.MachineLabelSelector == nil || sel.Matches(labels.Set(secret.Labels)) {
			for k, v := range opts.Config.Data {
				config[k] = v
			}
		}
	}

	if err := filterConfigData(config, a.controlPlane, secret); err != nil {
		return nil, err
	}

	// "data-dir" is explicitly not added to KDM for filtering because it is mapped to a field in the provisioning cluster
	// CRD. While technically possible to add feature gates and update KDM, there is nothing to be gained from such an
	// approach as the "data-dir" implementation will likely never change distro-side.
	if a.controlPlane.Spec.DataDirectories.K8sDistro != "" {
		config["data-dir"] = a.controlPlane.Spec.DataDirectories.K8sDistro
	}

	return config, nil
}

func (a *CAPRAdapter) DataDirectory(_ *corev1.Secret) string {
	return capr.GetDistroDataDir(a.controlPlane)
}

func (a *CAPRAdapter) ProvisioningDataDirectory() string {
	return capr.GetProvisioningDataDir(&a.controlPlane.Spec.ClusterConfiguration)
}

func (a *CAPRAdapter) KubectlPath(secret *corev1.Secret) string {
	if a.RuntimeCommand() == "k3s" {
		return "/usr/local/bin/kubectl"
	}
	return path.Join(a.DataDirectory(secret), "bin", "kubectl")
}

func (a *CAPRAdapter) KubeconfigPath(_ *corev1.Secret) string {
	if a.RuntimeCommand() == "k3s" {
		return "/etc/rancher/k3s/k3s.yaml"
	}
	return "/etc/rancher/rke2/rke2.yaml"
}

// ElectLeader picks a leader from the CAPR cluster's machine-plan secrets. Eligibility excludes
// secrets whose CAPI machine has a non-nil DeletionTimestamp (or whose machine is gone). Init
// detection uses capr.InitNodeLabel, which the planner sets during init-node election.
func (a *CAPRAdapter) ElectLeader(role LeaderRole, namespace string) (*corev1.Secret, error) {
	secrets, err := a.clients.Core.Secret().Cache().List(namespace, labels.SelectorFromSet(labels.Set{
		capr.ClusterNameLabel: a.controlPlane.Name,
	}))
	if err != nil {
		return nil, err
	}

	candidates := make([]LeaderCandidate, 0, len(secrets))
	for _, secret := range secrets {
		c := LeaderCandidate{Secret: secret, Eligible: true}
		c.Init = secret.Labels[capr.InitNodeLabel] == "true"

		machineName := secret.Labels[capr.MachineNameLabel]
		machineNamespace := secret.Labels[capr.MachineNamespaceLabel]
		if machineName != "" && machineNamespace != "" {
			machine, err := a.clients.CAPI.Machine().Cache().Get(machineNamespace, machineName)
			switch {
			case apierrors.IsNotFound(err):
				// The plan secret outlives its machine briefly during deletion; treat as ineligible.
				c.Eligible = false
			case err != nil:
				return nil, err
			case machine.DeletionTimestamp != nil:
				c.Eligible = false
			}
		}
		candidates = append(candidates, c)
	}
	return electLeader(role, candidates), nil
}

func (a *CAPRAdapter) PauseCluster(paused bool) error {
	cluster, err := a.clients.CAPI.Cluster().Cache().Get(a.controlPlane.Namespace, a.controlPlane.Name)
	if err != nil {
		return err
	}
	if ptr.Equal(cluster.Spec.Paused, &paused) {
		return nil
	}
	cluster = cluster.DeepCopy()
	cluster.Spec.Paused = &paused
	_, err = a.clients.CAPI.Cluster().Update(cluster)
	return err
}

func filterConfigData(config map[string]any, controlPlane *rkev1.RKEControlPlane, secret *corev1.Secret) error {
	var (
		isServer = IsControlPlane(secret) || IsEtcd(secret)
		release  = capr.GetKDMReleaseData(context.TODO(), controlPlane)
	)

	if release == nil {
		return fmt.Errorf("could not find release data")
	}

	for k, v := range config {
		if newV, ok := filterField(isServer, k, v, *release); ok {
			config[k] = newV
		} else {
			delete(config, k)
		}
	}
	return nil
}

func filterField(isServer bool, k string, v interface{}, release model.Release) (interface{}, bool) {
	if v == nil {
		return nil, false
	}

	field, fieldFound := release.AgentArgs[k]
	if !fieldFound && isServer {
		field, fieldFound = release.ServerArgs[k]
	}

	// can't find arg
	if !fieldFound {
		return nil, false
	}

	switch v.(type) {
	case string:
	case bool:
	case []interface{}:
	default:
		// unknown type
		return nil, false
	}

	if field.Type == "boolean" {
		return convert.ToBool(v), true
	}

	return v, true
}

// isCalico returns true if the cni is calico or calico+multus, and returns false otherwise.
func isCalico(controlPlane *rkev1.RKEControlPlane, runtime string) bool {
	// calico is only supported for rke2
	if runtime != capr.RuntimeRKE2 {
		return false
	}

	cni := convert.ToString(controlPlane.Spec.MachineGlobalConfig.Data["cni"])
	return cni == "" ||
		cni == "calico" ||
		cni == "calico+multus"
}

// getArgValue will search the passed in interface (arg) for a key that matches the searchArg. If a match is found, it
// returns the value of the argument, otherwise it returns an empty string.
func getArgValue(arg interface{}, searchArg string, delim string) string {
	logrus.Tracef("getArgValue (searchArg: %s, delim: %s) type of %v is %T", searchArg, delim, arg, arg)
	switch arg := arg.(type) {
	case []interface{}:
		logrus.Tracef("getArgValue (searchArg: %s, delim: %s) encountered interface slice %v", searchArg, delim, arg)
		return getArgValue(convertInterfaceSliceToStringSlice(arg), searchArg, delim)
	case []string:
		logrus.Tracef("getArgValue (searchArg: %s, delim: %s) found string array: %v", searchArg, delim, arg)
		for _, v := range arg {
			argKey, argVal := splitArgKeyVal(v, delim)
			if argKey == searchArg {
				return argVal
			}
		}
	case string:
		logrus.Tracef("getArgValue (searchArg: %s, delim: %s) found string: %v", searchArg, delim, arg)
		argKey, argVal := splitArgKeyVal(arg, delim)
		if argKey == searchArg {
			return argVal
		}
	}
	logrus.Tracef("getArgValue (searchArg: %s, delim: %s) did not find searchArg in: %v", searchArg, delim, arg)
	return ""
}

// splitArgKeyVal takes a value and returns a pair (key, value) of the argument, or two empty strings if there was not
// a parsed key/val.
func splitArgKeyVal(val string, delim string) (string, string) {
	if splitSubArg := strings.SplitN(val, delim, 2); len(splitSubArg) == 2 {
		return splitSubArg[0], splitSubArg[1]
	}
	return "", ""
}

// convertInterfaceSliceToStringSlice converts an input interface slice to a string slice by iterating through the
// interface slice and converting each entry to a string using Sprintf.
func convertInterfaceSliceToStringSlice(input []interface{}) []string {
	var stringArr []string
	for _, v := range input {
		stringArr = append(stringArr, fmt.Sprintf("%v", v))
	}
	return stringArr
}

// renderSecureProbe takes the existing argument value and renders a secure probe using the argument values and an error
// if one occurred.
func renderSecureProbe(arg any, rawProbe plan.Probe, dataDir string, loopbackAddress, defaultSecurePort string, defaultCertDir string, defaultCert string) (plan.Probe, error) {
	securePort := getArgValue(arg, SecurePortArgument, "=")
	if securePort == "" {
		// If the user set a custom --secure-port, set --secure-port to an empty string, so we don't override
		// their custom value
		securePort = defaultSecurePort
	}
	TLSCert := getArgValue(arg, TLSCertFileArgument, "=")
	if TLSCert == "" {
		// If the --tls-cert-file Argument was not set in the config for this component, we can look to see if
		// the --cert-dir was set. --tls-cert-file (if set) will take precedence over --cert-dir
		certDir := getArgValue(arg, CertDirArgument, "=")
		if certDir == "" {
			// If --cert-dir was not set, we use defaultCertDir value that was passed in, but must prefix the data-dir
			certDir = path.Join(dataDir, defaultCertDir)
		}
		// Our goal here is to generate the tlsCert. If we get to this point, we know we will be using the defaultCert
		TLSCert = certDir + "/" + defaultCert
	}
	return replaceCACertAndPortForProbes(rawProbe, TLSCert, loopbackAddress, securePort)
}

// replaceCACertAndPortForProbes adds/replaces the CACert and URL with rendered values based on the values provided.
func replaceCACertAndPortForProbes(probe plan.Probe, cacert, host, port string) (plan.Probe, error) {
	if cacert == "" {
		return plan.Probe{}, errEmptyCACert
	}
	if port == "" {
		return plan.Probe{}, errEmptyPort
	}
	if host == "" {
		return plan.Probe{}, errEmptyAddress
	}
	probe.HTTPGetAction.CACert = cacert
	probe.HTTPGetAction.URL = fmt.Sprintf(probe.HTTPGetAction.URL, host, port)
	return probe, nil
}

// replaceURLForProbes will insert the loopback host for all probes based on stack preference.
func replaceURLForProbes(probes map[string]plan.Probe, loopbackAddress string) map[string]plan.Probe {
	result := make(map[string]plan.Probe, len(probes))
	for k, v := range probes {
		v.HTTPGetAction.URL = replaceIfFormatSpecifier(v.HTTPGetAction.URL, loopbackAddress)
		result[k] = v
	}
	return result
}

// insertDataDirForProbes will insert the data-dir for all probes based on the controlplane object.
func insertDataDirForProbes(dataDir string, probes map[string]plan.Probe) map[string]plan.Probe {
	result := make(map[string]plan.Probe, len(probes))
	for k, v := range probes {
		v.HTTPGetAction.CACert = replaceIfFormatSpecifier(v.HTTPGetAction.CACert, dataDir)
		v.HTTPGetAction.ClientCert = replaceIfFormatSpecifier(v.HTTPGetAction.ClientCert, dataDir)
		v.HTTPGetAction.ClientKey = replaceIfFormatSpecifier(v.HTTPGetAction.ClientKey, dataDir)
		result[k] = v
	}
	return result
}

// replaceIfFormatSpecifier will insert the runtime of the k8s engine if the string str has a string format specifier.
func replaceIfFormatSpecifier(str string, runtime string) string {
	if !strings.Contains(str, "%s") {
		return str
	}
	return fmt.Sprintf(str, runtime)
}

type Filter func(secret *corev1.Secret) bool

func IsEtcd(secret *corev1.Secret) bool {
	return secret != nil && secret.Labels != nil && secret.Labels[capr.EtcdRoleLabel] == "true"
}

func IsControlPlane(secret *corev1.Secret) bool {
	return secret != nil && secret.Labels != nil && secret.Labels[capr.ControlPlaneRoleLabel] == "true"
}

func IsWindows(secret *corev1.Secret) bool {
	return secret != nil && secret.Labels != nil && secret.Labels[capr.CattleOSLabel] == "windows"
}

func And(l, r Filter) Filter {
	return func(secret *corev1.Secret) bool {
		return l(secret) && r(secret)
	}
}

func Or(l, r Filter) Filter {
	return func(secret *corev1.Secret) bool {
		return l(secret) || r(secret)
	}
}

func Not(filter Filter) Filter {
	return func(secret *corev1.Secret) bool {
		return !filter(secret)
	}
}

type ImportedAdapter struct {
	cluster *mgmtv3.Cluster
	clients *wrangler.CAPIContext
}

// WaitForRegister waits for all machine-plan secrets to be created, ensuring the system-agent has checked in for
// all expected nodes.
// All machine-plans secrets are listed and compared to the count of mgmtv3.Node objects for the cluster.
func (a *ImportedAdapter) WaitForRegister() (bool, error) {
	secretList, err := a.clients.Core.Secret().List(a.cluster.Name, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", capr.ClusterNameLabel, a.cluster.Name),
		FieldSelector: fmt.Sprintf("type=%s", capr.SecretTypeMachinePlan),
	})
	if err != nil {
		return false, err
	}

	secrets := secretList.Items

	machines, err := a.clients.Mgmt.Node().Cache().List(a.cluster.Name, labels.Everything())
	if err != nil {
		return false, err
	}

	// 1. If the counts don't match upfront, we already know it's not a 1:1 match
	if len(secrets) != len(machines) {
		return false, nil
	}

	// 2. Track the names of the machines we expect to see
	// Using a map[string]bool to easily check existence and prevent duplicates
	expectedMachines := make(map[string]bool, len(machines))
	for _, machine := range machines {
		expectedMachines[machine.Name] = true
	}

	// 3. Verify that every secret maps to a unique expected machine
	for _, secret := range secrets {
		if secret.Labels == nil {
			return false, nil
		}

		machineName, exists := secret.Labels[capr.MachineNameLabel]

		// If the label is missing, or it maps to a machine name we haven't seen/already matched
		if !exists || !expectedMachines[machineName] {
			return false, nil
		}

		// Mark this machine as "matched" by deleting it from the expected map.
		// This naturally catches duplicate secrets pointing to the same machine.
		delete(expectedMachines, machineName)
	}

	// 4. If the map is empty, we have a perfect, duplicate-free 1:1 match
	return len(expectedMachines) == 0, nil
}

// RuntimeCommand returns the command used to interact with the distro CLI (RKe2/K3s).
func (a *ImportedAdapter) RuntimeCommand() string {
	if a.cluster.Status.Provider == "rke2" {
		return "rke2"
	}
	return "k3s"
}

// ServerUnit returns the systemd unit name for a distro server node.
func (a *ImportedAdapter) ServerUnit() string {
	if a.cluster.Status.Provider == "rke2" {
		return "rke2-server"
	}
	return "k3s"
}

// RenderProbes renders the probes for a given machine-plan secret based on its role.
// Currently custom data directories, probes, and using ipv4 as the primary ip family are not supported.
func (a *ImportedAdapter) RenderProbes(secret *corev1.Secret) (map[string]plan.Probe, error) {
	var (
		runtime    = a.RuntimeCommand()
		probeNames []string
		probes     = map[string]plan.Probe{}
	)

	if runtime != capr.RuntimeK3S && IsEtcd(secret) {
		probeNames = append(probeNames, "etcd")
	}
	if IsControlPlane(secret) {
		probeNames = append(probeNames, "kube-apiserver")
		probeNames = append(probeNames, "kube-controller-manager")
		probeNames = append(probeNames, "kube-scheduler")
	}
	if !(And(IsEtcd, Not(IsControlPlane))(secret) && runtime == capr.RuntimeK3S) {
		// k3s doesn't run the kubelet on etcd only nodes
		probeNames = append(probeNames, "kubelet")
	}

	for _, probeName := range probeNames {
		probes[probeName] = allProbes[probeName]
	}

	dataDir := "/var/lib/rancher/rke2"
	if runtime == capr.RuntimeK3S {
		dataDir = "/var/lib/rancher/k3s"
	}

	probes = insertDataDirForProbes(dataDir, probes)

	// only support ipv4, need to implement per-node extraction mechanism
	loopbackAddress := "127.0.0.1"

	if IsControlPlane(secret) {
		kcmProbe, err := renderSecureProbe("", probes["kube-controller-manager"], dataDir, loopbackAddress, DefaultKubeControllerManagerDefaultSecurePort, DefaultKubeControllerManagerCertDir, DefaultKubeControllerManagerCert)
		if err != nil {
			return probes, err
		}
		probes["kube-controller-manager"] = kcmProbe

		ksProbe, err := renderSecureProbe("", probes["kube-scheduler"], dataDir, loopbackAddress, DefaultKubeSchedulerDefaultSecurePort, DefaultKubeSchedulerCertDir, DefaultKubeSchedulerCert)
		if err != nil {
			return probes, err
		}
		probes["kube-scheduler"] = ksProbe
	}

	probes = replaceURLForProbes(probes, loopbackAddress)

	return probes, nil
}

func (a *ImportedAdapter) PauseCluster(_ bool) error {
	return nil
}

func (a *ImportedAdapter) DataDirectory(_ *corev1.Secret) string {
	if a.cluster.Status.Provider == "rke2" {
		return "/var/lib/rancher/rke2/server"
	}
	return "/var/lib/rancher/k3s"
}

func (a *ImportedAdapter) ProvisioningDataDirectory() string {
	// Imported clusters do not expose the provisioning data directory; fall back to the default.
	return "/var/lib/rancher/capr"
}

func (a *ImportedAdapter) KubectlPath(secret *corev1.Secret) string {
	if a.cluster.Status.Provider == "k3s" {
		return "/usr/local/bin/kubectl"
	}
	return path.Join(a.DataDirectory(secret), "bin", "kubectl")
}

func (a *ImportedAdapter) KubeconfigPath(_ *corev1.Secret) string {
	if a.cluster.Status.Provider == "k3s" {
		return "/etc/rancher/k3s/k3s.yaml"
	}
	return "/etc/rancher/rke2/rke2.yaml"
}

// ElectLeader picks a leader from the imported cluster's machine-plan secrets. Eligibility excludes
// secrets whose backing v3.Node has a non-nil DeletionTimestamp (or whose Node is gone). Init
// detection parses the distro's node-args annotation off the v3.Node status — k3s init nodes pass
// --cluster-init; rke2 servers are init when they have no --server flag pointing elsewhere.
func (a *ImportedAdapter) ElectLeader(role LeaderRole, namespace string) (*corev1.Secret, error) {
	secrets, err := a.clients.Core.Secret().Cache().List(namespace, labels.SelectorFromSet(labels.Set{
		capr.ClusterNameLabel: a.cluster.Name,
	}))
	if err != nil {
		return nil, err
	}

	runtime := a.RuntimeCommand()

	candidates := make([]LeaderCandidate, 0, len(secrets))
	for _, secret := range secrets {
		c := LeaderCandidate{Secret: secret, Eligible: true}

		nodeName := secret.Labels[capr.MachineNameLabel]
		if nodeName != "" {
			node, err := a.clients.Mgmt.Node().Cache().Get(a.cluster.Name, nodeName)
			switch {
			case apierrors.IsNotFound(err):
				c.Eligible = false
			case err != nil:
				return nil, err
			case node.DeletionTimestamp != nil:
				c.Eligible = false
			default:
				c.Init = isImportedInitNode(node, runtime)
			}
		}
		candidates = append(candidates, c)
	}
	return electLeader(role, candidates), nil
}

// isImportedInitNode infers whether the downstream Node was started as the cluster's init server,
// based on the distro's node-args annotation kept on the v3.Node status by the nodesyncer.
//
//   - k3s: the init node passes --cluster-init.
//   - rke2: there is no explicit init flag; the first server has no --server <url>, so absence of
//     --server in a server-mode args list signals init.
//
// Returns false if the annotation is missing or unparseable; downstream tiers (etcd-only, etcd+cp)
// still apply.
func isImportedInitNode(node *mgmtv3.Node, runtime string) bool {
	if node == nil || node.Status.NodeAnnotations == nil {
		return false
	}
	raw := node.Status.NodeAnnotations[runtime+".io/node-args"]
	if raw == "" {
		return false
	}
	var args []string
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return false
	}

	isServer := false
	for _, arg := range args {
		if arg == "server" {
			isServer = true
			break
		}
	}
	if !isServer {
		return false
	}

	switch runtime {
	case capr.RuntimeK3S:
		for _, arg := range args {
			if arg == "--cluster-init" || strings.HasPrefix(arg, "--cluster-init=") {
				return true
			}
		}
		return false
	case capr.RuntimeRKE2:
		for _, arg := range args {
			if arg == "--server" || strings.HasPrefix(arg, "--server=") {
				return false
			}
		}
		return true
	}
	return false
}
