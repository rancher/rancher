package operations

import (
	"fmt"
	"path"

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
	return capiClusterAdapter(clients, capiCluster)
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
	return capiClusterAdapter(clients, capiCluster)
}

// BeaconRef returns the mgmt v3 Cluster's name-as-namespace + name convention. The mgmt v3
// Cluster is cluster-scoped, but its per-cluster namespace on the local cluster is named after
// the cluster itself and hosts every downstream artifact (mgmt v3 Nodes, beacons, machine-plan
// secrets stamped by systemagent, etc.).
func (a *ImportedAdapter) BeaconRef() (string, string) {
	return a.cluster.Name, a.cluster.Name
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
	if a.RuntimeCommand() == "rke2" {
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

func (a *ImportedAdapter) DistroDataDirectory(_ *corev1.Secret) string {
	if a.cluster.Status.Provider == "rke2" {
		return "/var/lib/rancher/rke2"
	}
	return "/var/lib/rancher/k3s"
}

func (a *ImportedAdapter) ProvisioningDataDirectory(_ *corev1.Secret) string {
	// Imported clusters do not expose the provisioning data directory; fall back to the default.
	return "/var/lib/rancher/capr"
}

// RenderProbes renders the probes for a given machine-plan secret based on its role.
// Currently custom data directories, probes, and using ipv4 as the primary ip family are not supported.
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

	for _, probeName := range probeNames {
		probes[probeName] = AllProbes[probeName]
	}

	dataDir := "/var/lib/rancher/rke2"
	if runtime == capr.RuntimeK3S {
		dataDir = "/var/lib/rancher/k3s"
	}

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
	}
	if err != nil {
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
	if a.cluster.Status.Provider == "k3s" {
		return "/usr/local/bin/kubectl"
	}
	return path.Join(a.DistroDataDirectory(secret), "bin", "kubectl")
}

func (a *ImportedAdapter) KubeconfigPath(_ *corev1.Secret) string {
	if a.cluster.Status.Provider == "k3s" {
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
