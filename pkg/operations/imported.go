package operations

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	provcluster "github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	"github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
)

const (
	rke2NodeArgsAnnotation = "rke2.io/node-args"
	k3sNodeArgsAnnotation  = "k3s.io/node-args"
	rke2NodeEnvAnnotation  = "rke2.io/node-env"
	k3sNodeEnvAnnotation   = "k3s.io/node-env"

	defaultRKE2DataDirectory = "/var/lib/rancher/rke2"
	defaultK3sDataDirectory  = "/var/lib/rancher/k3s"
)

func init() {
	// The Rancher UI creates day-2 operations with ClusterRef pointing at the mgmtv3.Cluster
	// (cluster-scoped, unique per user-facing cluster) regardless of the underlying
	// provisioner. This factory is where that ClusterRef is resolved to the right adapter:
	//
	//   - Turtles-imported CAPI cluster: mgmt cluster carries
	//     cluster-api.cattle.io/capi-cluster-owner{,-ns} labels naming the real CAPI Cluster.
	//     Load it and dispatch via capiClusterAdapter → CAPRAdapter or CAPRKE2Adapter.
	//
	//   - v2prov-administrated cluster (custom + node-driver): mgmt cluster carries the
	//     provisioning.cattle.io/administrated=true annotation. Trace mgmt → provv1.Cluster
	//     (via the ByCluster index) → CAPI Cluster in the provv1 namespace, then dispatch via
	//     capiClusterAdapter. This is CAPR.
	//
	//   - Generic imported RKE2/K3s (no turtles labels, no administrated annotation): fall
	//     through to ImportedAdapter.
	//
	// The mgmtv3.Cluster GVK is registered here because it is the *user-facing* handle for
	// every cluster type. capr.go and capi.go still register their native GVKs (provv1.Cluster,
	// capi.Cluster, RKEControlPlane) for direct-ref callers.
	RegisterAdapter(mgmtv3.SchemeGroupVersion.WithKind("Cluster"), func(clients *wrangler.CAPIContext, ustr *unstructured.Unstructured) (Adapter, error) {
		var cluster *mgmtv3.Cluster
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(ustr.Object, &cluster); err != nil {
			return nil, err
		}

		if adapter, err := turtlesCAPIAdapter(clients, cluster); adapter != nil || err != nil {
			return adapter, err
		}
		if adapter, err := administratedCAPIAdapter(clients, cluster); adapter != nil || err != nil {
			return adapter, err
		}

		return &ImportedAdapter{
			cluster: cluster,
			clients: clients,
		}, nil
	})
}

// turtlesCAPIAdapter returns a CAPI-backed adapter when cluster is a turtles-imported CAPI
// cluster shell — identified by the presence of both the capi-cluster-owner and -owner-ns
// labels. Returns (nil, nil) when the labels are absent (caller should try the next dispatch).
// One label present without the other is a misconfiguration and returns an error rather than
// silently falling through — matches the identity-resolver behaviour in the config server.
func turtlesCAPIAdapter(clients *wrangler.CAPIContext, cluster *mgmtv3.Cluster) (Adapter, error) {
	ownerName := cluster.Labels[capr.CAPIClusterOwnerLabel]
	ownerNS := cluster.Labels[capr.CAPIClusterOwnerNSLabel]
	if (ownerName == "") != (ownerNS == "") {
		return nil, fmt.Errorf(
			"mgmt cluster %s carries only one of %s/%s; both must be set for a turtles-imported CAPI cluster",
			cluster.Name, capr.CAPIClusterOwnerLabel, capr.CAPIClusterOwnerNSLabel)
	}
	if ownerName == "" {
		return nil, nil
	}
	capiCluster, err := clients.CAPI.Cluster().Cache().Get(ownerNS, ownerName)
	if err != nil {
		return nil, fmt.Errorf("mgmt cluster %s references CAPI cluster %s/%s: %w",
			cluster.Name, ownerNS, ownerName, err)
	}
	return capiClusterAdapter(clients, capiCluster, cluster.Name)
}

// administratedCAPIAdapter returns a CAPR adapter when cluster is a v2prov-administrated shell —
// identified by the provisioning.cattle.io/administrated=true annotation. Traces the mgmt
// cluster → provv1.Cluster (via the ByCluster index) → CAPI Cluster and dispatches via
// capiClusterAdapter. Returns (nil, nil) when the annotation is not set.
func administratedCAPIAdapter(clients *wrangler.CAPIContext, cluster *mgmtv3.Cluster) (Adapter, error) {
	if cluster.Annotations["provisioning.cattle.io/administrated"] != "true" {
		return nil, nil
	}
	provClusters, err := clients.Provisioning.Cluster().Cache().GetByIndex(provcluster.ByCluster, cluster.Name)
	if err != nil {
		return nil, fmt.Errorf("finding provisioning cluster for mgmt cluster %s: %w", cluster.Name, err)
	}
	if len(provClusters) != 1 {
		return nil, fmt.Errorf("expected exactly 1 provisioning cluster for mgmt cluster %s, got %d",
			cluster.Name, len(provClusters))
	}
	prov := provClusters[0]
	capiCluster, err := clients.CAPI.Cluster().Cache().Get(prov.Namespace, prov.Name)
	if err != nil {
		return nil, fmt.Errorf("finding CAPI cluster %s/%s for administrated mgmt cluster %s: %w",
			prov.Namespace, prov.Name, cluster.Name, err)
	}
	// v2prov clusters never take the CAPRKE2 branch (they route to CAPRAdapter), so the
	// mgmt-cluster-name parameter is inert here — pass empty.
	return capiClusterAdapter(clients, capiCluster, "")
}

// BeaconRef returns the mgmt v3 Cluster's name-as-namespace + name convention. The mgmt v3
// Cluster is cluster-scoped, but its per-cluster namespace on the local cluster is named after
// the cluster itself and hosts every downstream artifact (mgmt v3 Nodes, beacons, machine-plan
// secrets stamped by systemagent, etc.).
func (a *ImportedAdapter) BeaconRef() (string, string) {
	return a.cluster.Name, a.cluster.Name
}

// EtcdSnapshotNamespace returns the mgmt v3 Cluster's namespace convention (name-as-namespace).
// Snapshots on generic imported RKE2/K3s clusters live alongside the mgmt v3 Nodes.
func (a *ImportedAdapter) EtcdSnapshotNamespace() string {
	return a.cluster.Name
}

func (a *ImportedAdapter) ClusterObject() (*unstructured.Unstructured, error) {
	ustr, err := runtime.DefaultUnstructuredConverter.ToUnstructured(a.cluster)
	if err != nil {
		return nil, err
	}

	return &unstructured.Unstructured{Object: ustr}, nil
}

func (a *ImportedAdapter) LoopbackAddress(_ *corev1.Secret) string {
	return "127.0.0.1"
}

func (a *ImportedAdapter) ConfigFile(_ *corev1.Secret) string {
	return fmt.Sprintf("/etc/rancher/%s/config.yaml", a.RuntimeCommand())
}

func (a *ImportedAdapter) ConfigDirectory(_ *corev1.Secret) string {
	return fmt.Sprintf("/etc/rancher/%s/config.yaml.d", a.RuntimeCommand())
}

func (a *ImportedAdapter) GetServerURL(secret *corev1.Secret) string {
	if secret == nil {
		return ""
	}

	if !planv1alpha1.HasMachineLifecycleLabels(secret) {
		return ""
	}

	ref, err := planv1alpha1.MachineLifecycleLabelsToObjectReference(secret, secret.Namespace, a.clients.RESTMapper)
	if err != nil {
		logrus.Errorf("error getting reference for machine lifecycle labels: %v", err)
		return ""
	}

	machine, err := a.clients.Mgmt.Node().Cache().Get(ref.Namespace, ref.Name)
	if err != nil {
		logrus.Errorf("error getting machine %s for machine lifecycle: %v", ref.Name, err)
		return ""
	}

	if len(machine.Status.InternalNodeStatus.Addresses) == 0 {
		return ""
	}

	var address string

	for _, addr := range machine.Status.InternalNodeStatus.Addresses {
		if addr.Type == corev1.NodeExternalIP && address == "" {
			address = addr.Address
		} else if addr.Type == corev1.NodeInternalIP {
			address = addr.Address
		}
	}

	return address
}

func (a *ImportedAdapter) GetSupervisorPort(_ *corev1.Secret) string {
	if a.RuntimeCommand() == capr.RuntimeRKE2 {
		return "9345"
	}
	return "6443"
}

type ImportedAdapter struct {
	cluster *mgmtv3.Cluster
	clients *wrangler.CAPIContext
}

func (a *ImportedAdapter) ToS3ArgsEnvAndFiles(_ *corev1.Secret) ([]string, []string, []plan.File) {
	return nil, nil, nil
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

	// If the counts don't match upfront, we already know it's not a 1:1 match
	if len(secrets) != len(machines) {
		return false, nil
	}

	// Track the names of the machines we expect to see
	// Using a map[string]bool to easily check existence and prevent duplicates
	expectedMachines := make(map[string]bool, len(machines))
	for _, machine := range machines {
		expectedMachines[machine.Name] = true
	}

	// Verify that every secret maps to a unique expected machine
	for _, secret := range secrets {
		if secret.Labels == nil {
			return false, nil
		}

		machineName, exists := secret.Labels[planv1alpha1.MachineLifecycleNameLabel]

		// If the label is missing, or it maps to a machine name we haven't seen/already matched
		if !exists || !expectedMachines[machineName] {
			return false, nil
		}

		// Mark this machine as "matched" by deleting it from the expected map.
		// This naturally catches duplicate secrets pointing to the same machine.
		delete(expectedMachines, machineName)
	}

	// If the map is empty, we have a perfect, duplicate-free 1:1 match
	return len(expectedMachines) == 0, nil
}

// RuntimeCommand returns the command used to interact with the distro CLI (RKE2/K3S).
func (a *ImportedAdapter) RuntimeCommand() string {
	if a.cluster.Status.Provider == capr.RuntimeRKE2 {
		return capr.RuntimeRKE2
	}
	return capr.RuntimeK3S
}

// ServerUnit returns the systemd unit name for a distro server node.
func (a *ImportedAdapter) ServerUnit() string {
	if a.cluster.Status.Provider == capr.RuntimeRKE2 {
		return capr.RuntimeRKE2 + "-server"
	}
	return capr.RuntimeK3S
}

// managementNodeForSecret resolves the machine-plan Secret -> lifecycle labels ->
// management Node cache lookup chain. Returns (nil, nil) when the secret carries no
// lifecycle labels.
func (a *ImportedAdapter) managementNodeForSecret(secret *corev1.Secret) (*mgmtv3.Node, error) {
	if !planv1alpha1.HasMachineLifecycleLabels(secret) {
		return nil, nil
	}

	ref, err := planv1alpha1.MachineLifecycleLabelsToObjectReference(secret, secret.Namespace, a.clients.RESTMapper)
	if err != nil {
		return nil, fmt.Errorf("unable to resolve lifecycle reference for machine-plan secret %s/%s: %w", secret.Namespace, secret.Name, err)
	}

	node, err := a.clients.Mgmt.Node().Cache().Get(ref.Namespace, ref.Name)
	if apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("unable to find management node %s/%s for machine-plan secret %s/%s", ref.Namespace, ref.Name, secret.Namespace, secret.Name)
	} else if err != nil {
		return nil, fmt.Errorf("unable to get management node %s/%s for machine-plan secret %s/%s: %w", ref.Namespace, ref.Name, secret.Namespace, secret.Name, err)
	}

	return node, nil
}

// nodeArgsForRuntime parses the runtime-specific node-args annotation off the management
// Node's status. Returns nil, nil when node is nil or the annotation is absent/empty.
func nodeArgsForRuntime(node *mgmtv3.Node, runtime string) ([]string, error) {
	if node == nil {
		return nil, nil
	}

	argsKey := rke2NodeArgsAnnotation
	if runtime == capr.RuntimeK3S {
		argsKey = k3sNodeArgsAnnotation
	}

	argsJSON, ok := node.Status.NodeAnnotations[argsKey]
	if !ok || argsJSON == "" {
		return nil, nil
	}

	var args []string
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nil, fmt.Errorf("unable to parse %s annotation on management node %s/%s: %w", argsKey, node.Namespace, node.Name, err)
	}
	return args, nil
}

// nodeEnvForRuntime parses the runtime-specific node-env annotation off the management
// Node's status. Returns nil, nil when node is nil or the annotation is absent/empty.
func nodeEnvForRuntime(node *mgmtv3.Node, runtime string) (map[string]string, error) {
	if node == nil {
		return nil, nil
	}

	envKey := rke2NodeEnvAnnotation
	if runtime == capr.RuntimeK3S {
		envKey = k3sNodeEnvAnnotation
	}

	envJSON, ok := node.Status.NodeAnnotations[envKey]
	if !ok || envJSON == "" {
		return nil, nil
	}

	var env map[string]string
	if err := json.Unmarshal([]byte(envJSON), &env); err != nil {
		return nil, fmt.Errorf("unable to parse %s annotation on management node %s/%s: %w", envKey, node.Namespace, node.Name, err)
	}
	return env, nil
}

// nodeArgs returns the selected runtime's node arguments for the machine-plan secret.
// It follows the machine-plan Secret -> lifecycle labels -> management Node -> status
// nodeAnnotations lookup chain and returns an error when the lookup or parsing fails.
func (a *ImportedAdapter) nodeArgs(secret *corev1.Secret) ([]string, error) {
	node, err := a.managementNodeForSecret(secret)
	if err != nil {
		return nil, err
	}
	return nodeArgsForRuntime(node, a.RuntimeCommand())
}

// arguments is an ordered command-line argument list. It is a lightweight view
// over the supplied slice; newArguments does not copy the values.
type arguments []string

// newArguments creates an arguments view for querying command-line options.
func newArguments(args []string) arguments {
	return arguments(args)
}

// Last returns the last non-empty value supplied for any exact option name, or
// an empty string when none has a value. It accepts both split (--option value)
// and combined (--option=value) forms. When multiple aliases are given, the last
// value wins across all aliases.
func (a arguments) Last(names ...string) string {
	var last string
	for i, arg := range a {
		for _, name := range names {
			if arg == name {
				if i+1 < len(a) && a[i+1] != "" {
					last = a[i+1]
				}
				continue
			}
			if strings.HasPrefix(arg, name+"=") {
				if v := strings.TrimPrefix(arg, name+"="); v != "" {
					last = v
				}
			}
		}
	}
	return last
}

// Values returns every value supplied for one exact option name, preserving
// command-line order. It accepts both split (--option value) and combined
// (--option=value) forms, which is useful for repeated wrapper options.
func (a arguments) Values(name string) []string {
	var values []string
	for i, arg := range a {
		if arg == name {
			if i+1 < len(a) {
				values = append(values, a[i+1])
			}
			continue
		}
		if strings.HasPrefix(arg, name+"=") {
			values = append(values, strings.TrimPrefix(arg, name+"="))
		}
	}
	return values
}

// importedDistroDataDirectory returns the data directory for an imported cluster
// using runtime-specific precedence rules.
func importedDistroDataDirectory(runtime string, args []string, env map[string]string) string {
	// Environment variables take precedence over command-line arguments.
	if runtime == capr.RuntimeRKE2 {
		if dir := env["RKE2_DATA_DIR"]; dir != "" {
			return dir
		}
	} else if runtime == capr.RuntimeK3S {
		if dir := env["K3S_DATA_DIR"]; dir != "" {
			return dir
		}
	}

	// Check for explicit data directory in command-line arguments.
	if dataDir := newArguments(args).Last("--data-dir", "-d"); dataDir != "" {
		return dataDir
	}

	// Fall back to runtime default.
	if runtime == capr.RuntimeRKE2 {
		return defaultRKE2DataDirectory
	}
	return defaultK3sDataDirectory
}

func (a *ImportedAdapter) DistroDataDirectory(secret *corev1.Secret) string {
	runtime := a.RuntimeCommand()
	defaultDir := defaultRKE2DataDirectory
	if runtime == capr.RuntimeK3S {
		defaultDir = defaultK3sDataDirectory
	}

	node, err := a.managementNodeForSecret(secret)
	if err != nil {
		logrus.Debugf("[imported adapter] unable to read node configuration for %s/%s, using default data directory: %v", secret.Namespace, secret.Name, err)
		return defaultDir
	}

	args, err := nodeArgsForRuntime(node, runtime)
	if err != nil {
		logrus.Debugf("[imported adapter] unable to parse node args for %s/%s, using default data directory: %v", secret.Namespace, secret.Name, err)
		return defaultDir
	}

	env, err := nodeEnvForRuntime(node, runtime)
	if err != nil {
		logrus.Debugf("[imported adapter] unable to parse node env for %s/%s, using default data directory: %v", secret.Namespace, secret.Name, err)
		return defaultDir
	}

	return importedDistroDataDirectory(runtime, args, env)
}

// componentTLSSettingsFromNodeArgs extracts scheduler/controller-manager
// TLS settings from imported node arguments.
func componentTLSSettingsFromNodeArgs(args []string, component string) ComponentTLSSettings {
	var outer string
	switch component {
	case KubeControllerManagerProbeName:
		outer = "--" + KubeControllerManagerArg
	case KubeSchedulerProbeName:
		outer = "--" + KubeSchedulerArg
	default:
		return ComponentTLSSettings{}
	}

	innerArgs := newArguments(args).Values(outer)

	var settings ComponentTLSSettings
	for _, arg := range innerArgs {
		key, value, ok := strings.Cut(arg, "=")
		if !ok {
			continue
		}
		switch key {
		case SecurePortArgument:
			settings.SecurePort = value
		case TLSCertFileArgument:
			settings.TLSCertFile = value
		case TLSPrivateKeyFile:
			settings.TLSPrivateKeyFile = value
		}
	}
	return settings
}

// CertificateRotationComponentTLSSettings returns scheduler/controller-manager
// TLS settings parsed from the imported node's effective runtime arguments.
func (a *ImportedAdapter) CertificateRotationComponentTLSSettings(secret *corev1.Secret, component string) (ComponentTLSSettings, error) {
	args, err := a.nodeArgs(secret)
	if err != nil {
		return ComponentTLSSettings{}, err
	}
	return componentTLSSettingsFromNodeArgs(args, component), nil
}

func (a *ImportedAdapter) ProvisioningDataDirectory(_ *corev1.Secret) string {
	// Imported clusters do not expose the provisioning data directory; fall back to the default.
	return "/var/lib/rancher/capr"
}

// RenderProbes renders the probes for a given machine-plan secret based on its role.
// Imported clusters currently support per-node custom data-directory paths via DistroDataDirectory.
func (a *ImportedAdapter) RenderProbes(secret *corev1.Secret, supervisor bool) (map[string]plan.Probe, error) {
	var (
		runtime    = a.RuntimeCommand()
		probeNames []string
		probes     = map[string]plan.Probe{}
	)

	if runtime != capr.RuntimeK3S && IsEtcd(secret) {
		probeNames = append(probeNames, ETCDProbeName)
	}
	if IsControlPlane(secret) {
		probeNames = append(probeNames, KubeAPIServerProbeName)
		probeNames = append(probeNames, KubeControllerManagerProbeName)
		probeNames = append(probeNames, KubeSchedulerProbeName)
	}
	if !(And(IsEtcd, Not(IsControlPlane))(secret) && runtime == capr.RuntimeK3S) {
		// k3s doesn't run the kubelet on etcd only nodes
		probeNames = append(probeNames, KubeletProbeName)
	}

	// Add Calico probe for imported RKE2 nodes that are not etcd-only and not Windows.
	if runtime == capr.RuntimeRKE2 && !And(IsEtcd, Not(IsControlPlane))(secret) && !IsWindows(secret) {
		probeNames = append(probeNames, CalicoProbeName)
	}

	for _, probeName := range probeNames {
		probes[probeName] = AllProbes[probeName]
	}

	dataDir := a.DistroDataDirectory(secret)

	// only support ipv4, need to implement per-node extraction mechanism
	loopbackAddress := "127.0.0.1"

	// render this probe separately because it has a specific format
	if supervisor && (IsEtcd(secret) || IsControlPlane(secret)) {
		supervisorProbe := AllProbes[SupervisorProbeName]
		port := 9345
		if runtime == capr.RuntimeK3S {
			port = 6443
		}
		supervisorProbe.HTTPGetAction.URL = fmt.Sprintf(supervisorProbe.HTTPGetAction.URL, loopbackAddress, port, runtime)
		probes[SupervisorProbeName] = supervisorProbe
	}

	probes = InsertDataDirForProbes(dataDir, probes)

	if IsControlPlane(secret) {
		kcmProbe, err := renderSecureProbe("", probes[KubeControllerManagerProbeName], dataDir, loopbackAddress, DefaultKubeControllerManagerPort, DefaultKubeControllerManagerCertDir, DefaultKubeControllerManagerCert)
		if err != nil {
			return probes, err
		}
		probes[KubeControllerManagerProbeName] = kcmProbe

		ksProbe, err := renderSecureProbe("", probes[KubeSchedulerProbeName], dataDir, loopbackAddress, DefaultKubeSchedulerPort, DefaultKubeSchedulerCertDir, DefaultKubeSchedulerCert)
		if err != nil {
			return probes, err
		}
		probes[KubeSchedulerProbeName] = ksProbe
	}

	probes = ReplaceURLForProbes(probes, loopbackAddress)

	return probes, nil
}

// isSuitableLeader returns true when the mgmtv3.Node backing the plan secret exists,
// is not deleting, and is Ready. Imported clusters have no CAPI Machine, readiness is
// verified via mgmtv3.Node.
func (a *ImportedAdapter) isSuitableLeader(s *corev1.Secret) (bool, error) {
	machineName := MachineName(s)
	node, err := a.clients.Mgmt.Node().Cache().Get(a.cluster.Name, machineName)
	if apierrors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if node.DeletionTimestamp != nil {
		return false, nil
	}
	return mgmtv3.NodeConditionReady.IsTrue(node), nil
}

// FindOrElectLeader finds or elects a machine-plan secret to lead the given operation.
// Candidates are collected from the cluster namespace, filtered by filter, and sorted
// deterministically. An existing leader annotation is reused if the leader is still suitable;
// otherwise a new leader is elected and the annotation written with retry-on-conflict.
// Returns nil, nil when no suitable candidate exists yet.
func (a *ImportedAdapter) FindOrElectLeader(operation string, filter Filter) (*corev1.Secret, error) {
	candidates, err := plan.NewCollector(a.clients.Core.Secret(), a.cluster, a.cluster.Name).
		WithFilter(plan.FilterFunc(filter)).
		WithSorter(plan.DefaultSorter()).
		Collect()
	if err != nil {
		return nil, err
	}

	var (
		marked        *corev1.Secret
		markedCount   int
		markedReady   bool
		initCandidate *corev1.Secret
		fallback      *corev1.Secret
	)
	for _, secret := range candidates {
		if secret.Annotations[OperationLeaderAnnotation] == operation {
			marked = secret
			markedCount++
			if markedCount > 1 {
				return nil, fmt.Errorf("multiple machine-plan secrets marked as operation leader for %s", operation)
			}
		}

		ok, err := a.isSuitableLeader(secret)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if marked != nil && secret.Namespace == marked.Namespace && secret.Name == marked.Name {
			markedReady = true
		}
		if initCandidate == nil && IsInitNode(secret) {
			initCandidate = secret
		}
		if fallback == nil {
			fallback = secret
		}
	}

	if marked != nil {
		if markedReady {
			return marked, nil
		}
		logrus.Warnf("[operations] %s: elected leader %s is no longer suitable, re-electing", a.cluster.Name, marked.Name)
		if err := a.clearLeaderAnnotation(marked, operation); err != nil {
			return nil, err
		}
	}
	if initCandidate != nil {
		return a.markLeader(initCandidate, operation)
	}
	if fallback != nil {
		return a.markLeader(fallback, operation)
	}
	return nil, nil
}

func (a *ImportedAdapter) KubectlPath(secret *corev1.Secret) string {
	if a.RuntimeCommand() == capr.RuntimeK3S {
		return "/usr/local/bin/kubectl"
	}
	return path.Join(a.DistroDataDirectory(secret), "bin", "kubectl")
}

func (a *ImportedAdapter) KubeconfigPath(_ *corev1.Secret) string {
	if a.RuntimeCommand() == capr.RuntimeK3S {
		return "/etc/rancher/k3s/k3s.yaml"
	}
	return "/etc/rancher/rke2/rke2.yaml"
}

func (a *ImportedAdapter) markLeader(secret *corev1.Secret, operation string) (*corev1.Secret, error) {
	var updated *corev1.Secret
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		s, err := a.clients.Core.Secret().Get(secret.Namespace, secret.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if s.Annotations[OperationLeaderAnnotation] == operation {
			updated = s
			return nil
		}
		if s.Annotations == nil {
			s.Annotations = make(map[string]string)
		}
		s.Annotations[OperationLeaderAnnotation] = operation
		updated, err = a.clients.Core.Secret().Update(s)
		return err
	})
	return updated, err
}

func (a *ImportedAdapter) clearLeaderAnnotation(secret *corev1.Secret, operation string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		s, err := a.clients.Core.Secret().Get(secret.Namespace, secret.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if s.Annotations == nil || s.Annotations[OperationLeaderAnnotation] != operation {
			return nil
		}
		delete(s.Annotations, OperationLeaderAnnotation)
		_, err = a.clients.Core.Secret().Update(s)
		return err
	})
}

// PauseCluster is a no-op for imported clusters since they have no CAPI cluster.
func (a *ImportedAdapter) PauseCluster(_ bool) error {
	return nil
}
