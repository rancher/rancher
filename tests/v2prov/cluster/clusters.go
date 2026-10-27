package cluster

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"maps"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provisioningv1api "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1/snapshotutil"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/capr/machineprovision"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/namespace"
	"github.com/rancher/rancher/tests/v2prov/nodeconfig"
	"github.com/rancher/rancher/tests/v2prov/registry"
	"github.com/rancher/rancher/tests/v2prov/systemdnode"
	"github.com/rancher/rancher/tests/v2prov/wait"
	"github.com/rancher/wrangler/v3/pkg/condition"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/controllers/external"
)

const ConflictMessageRegex = `\[K8s\] encountered an error while attempting to update the secret: Operation cannot be fulfilled on secrets.*: the object has been modified; please apply your changes to the latest version and try again`
const SaneConflictMessageThreshold = 10

type ImportedNodePool struct {
	ControlPlane bool
	ETCD         bool
	Worker       bool
	Quantity     int

	// DistroDataDir, when non-empty, sets the RKE2/K3s data-dir in the initial
	// generated config for pods created from this pool, before the service starts.
	DistroDataDir string
}

// NewImported creates a management cluster object for an imported cluster.
// This function is expected to be called in conjunction with NewImportedClusterPods.
func NewImported(clients *clients.Clients, c *v3.Cluster) (*v3.Cluster, error) {
	c = c.DeepCopy()
	if c.Name == "" && c.GenerateName == "" {
		c.GenerateName = "test-imported-"
	}

	created, err := clients.Mgmt.Cluster().Create(c)
	if err != nil {
		return nil, err
	}

	clients.OnClose(func() {
		clients.Mgmt.Cluster().Delete(created.Name, &metav1.DeleteOptions{})
	})

	return created, nil
}

// ImportedClusterOptions carries optional extras for the nodes NewImportedClusterPods creates.
type ImportedClusterOptions struct {
	// ServerConfig entries are added to the config.yaml drop-in of every server (etcd and/or
	// control-plane) node, emitted in sorted key order so the rendered user data is deterministic.
	// Agent nodes are deliberately left alone — server-only keys are not valid in an agent's config.
	ServerConfig map[string]string
	// ServerFiles are written verbatim on every server node before the distro is installed, keyed by
	// absolute path. For anything a config key has to point at, e.g. an S3 endpoint CA bundle.
	ServerFiles map[string]string
	// ExtractServerToken brings the init node up without a token, letting the distro generate its
	// own, then reads the resulting full server token (K10<ca-hash>::server:<password>) back off the
	// init node's disk and hands that to every joining node. This mirrors how a cluster is typically
	// stood up by hand, as opposed to pre-seeding every node with the same shared secret.
	ExtractServerToken bool
	// Network, when set, selects the addresses the nodes use to reach each other. Nil keeps the
	// distro defaults: a single-stack IPv4 cluster on the pod's primary address.
	Network *ImportedNetwork
	// ExternalCloudProvider runs the nodes the way they run under an external cloud controller: the
	// distro's own cloud controller is disabled and every kubelet runs with cloud-provider=external.
	// The kubelet then neither sets nor overwrites the node's addresses, and registers the node with
	// the node.cloudprovider.kubernetes.io/uninitialized taint, both of which are left to whatever
	// plays the cloud controller — in tests, the test itself. This is how a Node can report addresses
	// the distro knows nothing about, and so has not put in its serving certificate.
	ExternalCloudProvider bool
	// BeforeJoin, when set, is called with the running init node pod — after its server token has
	// been extracted, with ExtractServerToken — and before any joining node is created. Joining
	// nodes are only created once it returns nil. It lets a test observe the init node before anything
	// has connected to it.
	BeforeJoin func(initPod *corev1.Pod) error
}

// Labels NewImportedClusterPods puts on every node pod, so tests can map a pod to the downstream node
// it runs without reconstructing the pool ordering.
const (
	// ImportedNodeNameLabel is the downstream node name (the node-name in the node's config).
	ImportedNodeNameLabel = "v2prov.cattle.io/imported-node-name"
	// ImportedNodeETCDLabel is "true" on nodes that run etcd.
	ImportedNodeETCDLabel = "v2prov.cattle.io/imported-node-etcd"
)

// ImportedNetwork selects which of a node pod's subnets — and through which interface — the
// downstream cluster runs on. Every field is optional; zero values fall back to the defaults below.
//
// The node pods get their addresses from the cluster the test runs against, so a family can only be
// used if that cluster hands pods an address in it: IPv6 needs a dual-stack host cluster (see
// dev-scripts/k3d-dual-stack).
type ImportedNetwork struct {
	// IPFamilies are the address families the downstream cluster runs, primary first. The primary
	// family is what nodes join over and what etcd advertises its peer URLs on. Defaults to IPv4.
	IPFamilies []corev1.IPFamily
	// Interface is the in-pod interface each node takes its node-ip from, one address per family.
	// When set it is also passed to flannel as flannel-iface. Defaults to eth0, without flannel-iface.
	Interface string
	// ClusterCIDRs and ServiceCIDRs are the downstream pod and service CIDRs, one per entry of
	// IPFamilies and in the same order. They default per family to subnets that do not collide with
	// the host cluster's (see defaultImportedCIDRs).
	ClusterCIDRs []string
	ServiceCIDRs []string
	// JoinFamily is the family joining nodes use to reach the init node. Defaults to the primary
	// family. A server's certificate only covers its node-ip addresses, so setting this to a family
	// outside IPFamilies makes nodes join through an address the init node's certificate does not
	// list — which fails, and is only useful for exercising that failure.
	JoinFamily corev1.IPFamily
}

// defaultImportedCIDRs are the downstream cluster and service CIDRs per family. The IPv4 pair is what
// imported nodes have always used; the IPv6 pair sits next to the host cluster's fd00:42::/56 and
// fd00:43::/112 from dev-scripts/k3d-dual-stack without overlapping them.
var defaultImportedCIDRs = map[corev1.IPFamily][2]string{
	corev1.IPv4Protocol: {"10.44.0.0/16", "10.45.0.0/16"},
	corev1.IPv6Protocol: {"fd00:44::/56", "fd00:45::/112"},
}

// resolve returns a copy of n with every default filled in, or an error if n is inconsistent.
func (n ImportedNetwork) resolve() (ImportedNetwork, error) {
	if len(n.IPFamilies) == 0 {
		n.IPFamilies = []corev1.IPFamily{corev1.IPv4Protocol}
	}
	if len(n.IPFamilies) > 2 || (len(n.IPFamilies) == 2 && n.IPFamilies[0] == n.IPFamilies[1]) {
		return n, fmt.Errorf("IPFamilies must be one family, or one of each: got %v", n.IPFamilies)
	}
	for _, family := range n.IPFamilies {
		if _, ok := defaultImportedCIDRs[family]; !ok {
			return n, fmt.Errorf("unsupported IP family %q", family)
		}
	}

	if n.JoinFamily == "" {
		n.JoinFamily = n.IPFamilies[0]
	} else if _, ok := defaultImportedCIDRs[n.JoinFamily]; !ok {
		return n, fmt.Errorf("unsupported join IP family %q", n.JoinFamily)
	}

	if len(n.ClusterCIDRs) == 0 {
		for _, family := range n.IPFamilies {
			n.ClusterCIDRs = append(n.ClusterCIDRs, defaultImportedCIDRs[family][0])
		}
	}
	if len(n.ServiceCIDRs) == 0 {
		for _, family := range n.IPFamilies {
			n.ServiceCIDRs = append(n.ServiceCIDRs, defaultImportedCIDRs[family][1])
		}
	}
	if len(n.ClusterCIDRs) != len(n.IPFamilies) || len(n.ServiceCIDRs) != len(n.IPFamilies) {
		return n, fmt.Errorf("need one cluster and one service CIDR per IP family %v: got %v and %v",
			n.IPFamilies, n.ClusterCIDRs, n.ServiceCIDRs)
	}

	return n, nil
}

// interfaceName is the interface node IPs are taken from.
func (n ImportedNetwork) interfaceName() string {
	if n.Interface == "" {
		return "eth0"
	}
	return n.Interface
}

// PodIPForFamily returns pod's address in family, or "" if it has none.
func PodIPForFamily(pod *corev1.Pod, family corev1.IPFamily) string {
	for _, podIP := range pod.Status.PodIPs {
		ip := net.ParseIP(podIP.IP)
		if ip == nil {
			continue
		}
		if (ip.To4() != nil) == (family == corev1.IPv4Protocol) {
			return podIP.IP
		}
	}
	return ""
}

// NewImportedClusterPods creates systemd-node pods that independently bootstrap an RKE2/K3s cluster.
// The first ETCD node is brought up as the init node, and subsequent nodes join via the
// init node's IP at the supervisor port (9345 for RKE2, 6443 for K3s).
func NewImportedClusterPods(clients *clients.Clients, namespace, k8sVersion string, pools []ImportedNodePool, labels map[string]string, registryCACert []byte, opts ...ImportedClusterOptions) ([]*corev1.Pod, error) {
	var options ImportedClusterOptions
	if len(opts) > 1 {
		return nil, fmt.Errorf("at most one ImportedClusterOptions may be supplied, got %d", len(opts))
	} else if len(opts) == 1 {
		options = opts[0]
	}

	distro := capr.GetRuntime(k8sVersion)
	supervisorPort := capr.GetRuntimeSupervisorPort(k8sVersion)

	var network *ImportedNetwork
	if options.Network != nil {
		resolved, err := options.Network.resolve()
		if err != nil {
			return nil, fmt.Errorf("invalid imported network: %w", err)
		}
		network = &resolved
	}

	var token string
	if !options.ExtractServerToken {
		var err error
		token, err = generateImportedToken()
		if err != nil {
			return nil, err
		}
	}

	type nodeSpec struct {
		cp, etcd, worker bool
		dataDir          string
	}

	var initSpec *nodeSpec
	var joinSpecs []nodeSpec

	for _, pool := range pools {
		for i := 0; i < pool.Quantity; i++ {
			spec := nodeSpec{
				cp:      pool.ControlPlane,
				etcd:    pool.ETCD,
				worker:  pool.Worker,
				dataDir: pool.DistroDataDir,
			}
			if initSpec == nil && pool.ETCD {
				initSpec = &spec
			} else {
				joinSpecs = append(joinSpecs, spec)
			}
		}
	}

	if initSpec == nil {
		return nil, fmt.Errorf("at least one pool must have ETCD: true for the init node")
	}

	var pods []*corev1.Pod

	initScript := importedNodeUserData(distro, k8sVersion, token, "imported-init-0", "", true, initSpec.cp, initSpec.etcd, initSpec.worker, initSpec.dataDir, registryCACert, options, network)
	initPod, err := systemdnode.New(clients, namespace, initScript, importedNodeLabels(labels, "imported-init-0", initSpec.etcd), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create init node: %w", err)
	}
	pods = append(pods, initPod)

	err = wait.Object(clients.Ctx, clients.Core.Pod().Watch, initPod, func(obj runtime.Object) (bool, error) {
		initPod = obj.(*corev1.Pod)
		return initPod.Status.PodIP != "" && initPod.Status.Phase == corev1.PodRunning, nil
	})
	if err != nil {
		return nil, fmt.Errorf("init node pod did not become ready: %w", err)
	}

	// Joining nodes reach the init node over the network's join family, by default its primary one;
	// without a network that is simply the pod's primary address.
	joinAddress := initPod.Status.PodIP
	if network != nil {
		for _, family := range append(slices.Clone(network.IPFamilies), network.JoinFamily) {
			if PodIPForFamily(initPod, family) == "" {
				return nil, fmt.Errorf("init node pod %s/%s has no %s address (pod IPs %v); the cluster under test must give pods an address in every family the imported network uses",
					initPod.Namespace, initPod.Name, family, initPod.Status.PodIPs)
			}
		}
		joinAddress = PodIPForFamily(initPod, network.JoinFamily)
	}

	if options.ExtractServerToken {
		dataDir := initSpec.dataDir
		if dataDir == "" {
			dataDir = fmt.Sprintf("/var/lib/rancher/%s", distro)
		}
		token, err = waitForServerToken(clients, namespace, initPod.Name, dataDir)
		if err != nil {
			return nil, err
		}
	}

	if options.BeforeJoin != nil && len(joinSpecs) > 0 {
		if err := options.BeforeJoin(initPod); err != nil {
			return pods, fmt.Errorf("before joining nodes to %s/%s: %w", initPod.Namespace, initPod.Name, err)
		}
	}

	serverURL := "https://" + net.JoinHostPort(joinAddress, strconv.Itoa(supervisorPort))

	for i, spec := range joinSpecs {
		nodeName := fmt.Sprintf("imported-node-%d", i+1)
		script := importedNodeUserData(distro, k8sVersion, token, nodeName, serverURL, false, spec.cp, spec.etcd, spec.worker, spec.dataDir, registryCACert, options, network)
		pod, err := systemdnode.New(clients, namespace, script, importedNodeLabels(labels, nodeName, spec.etcd), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create node %s: %w", nodeName, err)
		}
		pods = append(pods, pod)
	}

	return pods, nil
}

func New(clients *clients.Clients, cluster *provisioningv1api.Cluster) (*provisioningv1api.Cluster, error) {
	cluster = cluster.DeepCopy()
	if cluster.Namespace == "" {
		newNs, err := namespace.Random(clients)
		if err != nil {
			return nil, err
		}
		cluster.Namespace = newNs.Name
	}

	if cluster.Name == "" && cluster.GenerateName == "" {
		cluster.GenerateName = "test-cluster-"
	}

	if cluster.Spec.KubernetesVersion == "" {
		cluster.Spec.KubernetesVersion = defaults.SomeK8sVersion
	}

	if defaults.PrimeMode == "1" && defaults.PrimeRegistryHost != "" {
		appendGlobalRegistrySelector(cluster, defaults.PrimeRegistryHost)
		logrus.Infof("Prime registry active")
	}

	if cluster.Spec.RKEConfig != nil {
		if cluster.Spec.RKEConfig.MachineGlobalConfig.Data == nil {
			cluster.Spec.RKEConfig.MachineGlobalConfig.Data = map[string]interface{}{}
		}
		for k, v := range defaults.CommonClusterConfig {
			cluster.Spec.RKEConfig.MachineGlobalConfig.Data[k] = v
		}

		for i, mp := range cluster.Spec.RKEConfig.MachinePools {
			if mp.NodeConfig == nil {
				podConfig, err := nodeconfig.NewPodConfig(clients, cluster.Namespace)
				if err != nil {
					return nil, err
				}
				cluster.Spec.RKEConfig.MachinePools[i].NodeConfig = podConfig
			}
			if mp.Name == "" {
				cluster.Spec.RKEConfig.MachinePools[i].Name = fmt.Sprintf("pool-%d", i)
			}
		}

		if cluster.Spec.RKEConfig.Registries == nil {
			registryConfig, err := registry.CreateOrGetRegistry(clients, cluster.Namespace, "registry-cache", false)
			if err != nil {
				return nil, err
			}
			cluster.Spec.RKEConfig.Registries = &registryConfig
		}

		if cluster.Spec.RKEConfig.ETCD == nil {
			cluster.Spec.RKEConfig.ETCD = &rkev1.ETCD{
				DisableSnapshots: true,
			}
		}

		// If an override is specified for a data directory, and one is not explicitly set for the cluster, use it
		if cluster.Spec.RKEConfig.DataDirectories.K8sDistro == "" {
			cluster.Spec.RKEConfig.DataDirectories.K8sDistro = defaults.DistroDataDir
		}

		if cluster.Spec.RKEConfig.DataDirectories.Provisioning == "" {
			cluster.Spec.RKEConfig.DataDirectories.Provisioning = defaults.ProvisioningDataDir
		}

		if cluster.Spec.RKEConfig.DataDirectories.SystemAgent == "" {
			cluster.Spec.RKEConfig.DataDirectories.SystemAgent = defaults.SystemAgentDataDir
		}
	}

	c, err := clients.Provisioning.Cluster().Create(cluster)
	if err != nil {
		return nil, err
	}

	clients.OnClose(func() {
		clients.Provisioning.Cluster().Delete(c.Namespace, c.Name, &metav1.DeleteOptions{})
	})

	return c, nil
}

func Machines(clients *clients.Clients, cluster *provisioningv1api.Cluster) (*capi.MachineList, error) {
	return clients.CAPI.Machine().List(cluster.Namespace, metav1.ListOptions{
		LabelSelector: "cluster.x-k8s.io/cluster-name=" + cluster.Name,
	})
}

func MachineSets(clients *clients.Clients, cluster *provisioningv1api.Cluster) (*capi.MachineSetList, error) {
	return clients.CAPI.MachineSet().List(cluster.Namespace, metav1.ListOptions{
		LabelSelector: "cluster.x-k8s.io/cluster-name=" + cluster.Name,
	})
}

func PodInfraMachines(clients *clients.Clients, cluster *provisioningv1api.Cluster) (*unstructured.UnstructuredList, error) {
	return clients.Dynamic.Resource(schema.GroupVersionResource{
		Group:    capr.RKEMachineAPIGroup,
		Version:  "v1",
		Resource: "podmachines",
	}).Namespace(cluster.Namespace).List(clients.Ctx, metav1.ListOptions{
		LabelSelector: "cluster.x-k8s.io/cluster-name=" + cluster.Name,
	})
}

func WaitForCreate(clients *clients.Clients, c *provisioningv1api.Cluster) (_ *provisioningv1api.Cluster, err error) {
	defer func() {
		if err != nil {
			data, newErr := GatherDebugData(clients, c)
			if newErr != nil {
				logrus.Error(newErr)
			}
			//nolint:revive
			err = fmt.Errorf("cluster %s creation wait failed on: %w\ncluster %s test data bundle: \n%s\n", c.Name, err, c.Name, data)
		}
	}()

	err = wait.Object(clients.Ctx, clients.Provisioning.Cluster().Watch, c, func(obj runtime.Object) (bool, error) {
		c = obj.(*provisioningv1api.Cluster)
		return c.Status.ClusterName != "" && c.Status.Ready && c.Status.ObservedGeneration == c.Generation && capr.Ready.IsTrue(c) && capr.Provisioned.IsTrue(c), nil
	})
	if err != nil {
		return nil, fmt.Errorf("prov cluster is not ready: %w", err)
	}

	if len(c.Spec.RKEConfig.MachinePools) > 0 {
		machineSets, err := MachineSets(clients, c)
		if err != nil {
			return nil, err
		}

		for _, machineSet := range machineSets.Items {
			// retrieve the corresponding machinedeployment and verify that it is "sane"
			mpName, ok := machineSet.Labels[capr.RKEMachinePoolNameLabel]
			if !ok {
				return nil, fmt.Errorf("machineset %s/%s did not have a corresponding machine pool name label", machineSet.Namespace, machineSet.Name)
			}
			md, err := clients.CAPI.MachineDeployment().Get(c.Namespace, name.SafeConcatName(c.Name, mpName), metav1.GetOptions{})
			if err != nil {
				return nil, err
			}
			err = wait.Object(clients.Ctx, clients.CAPI.MachineDeployment().Watch, md, func(obj runtime.Object) (bool, error) {
				md = obj.(*capi.MachineDeployment)
				for _, mp := range c.Spec.RKEConfig.MachinePools {
					if mpName == mp.Name {
						mpQuantityMatches := true
						if mp.Quantity != nil {
							mpQuantityMatches = *mp.Quantity == *md.Spec.Replicas
						}
						return mpQuantityMatches && md.Status.Phase == "Running" && capr.MachineDeploymentMachinesReadyCondition.IsTrue(md), nil
					}
				}
				return false, nil
			})
			if err != nil {
				return nil, fmt.Errorf("MachineDeployment %s/%s was not ready: %w", md.Namespace, md.Name, err)
			}
		}
	}

	machines, err := Machines(clients, c)
	if err != nil {
		return nil, err
	}

	for _, machine := range machines.Items {
		if !machine.DeletionTimestamp.IsZero() {
			continue
		}
		err = wait.Object(clients.Ctx, clients.CAPI.Machine().Watch, &machine, func(obj runtime.Object) (bool, error) {
			machine = *obj.(*capi.Machine)
			return machine.Status.NodeRef.IsDefined(), nil
		})
		if err != nil {
			return nil, fmt.Errorf("noderef not assigned to %s/%s: %w", machine.Namespace, machine.Name, err)
		}
	}

	mgmtCluster, err := clients.Mgmt.Cluster().Get(c.Status.ClusterName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	err = wait.Object(clients.Ctx, func(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
		return clients.Mgmt.Cluster().Watch(opts)
	}, mgmtCluster, func(obj runtime.Object) (bool, error) {
		mgmtCluster = obj.(*v3.Cluster)
		return condition.Cond("Ready").IsTrue(mgmtCluster), nil
	})
	if err != nil {
		return nil, fmt.Errorf("mgmt cluster is not ready: %w", err)
	}

	return c, nil
}

func WaitForControlPlane(clients *clients.Clients, c *provisioningv1api.Cluster, errorPrefix string, rkeControlPlaneCheckFunc func(rkeControlPlane *rkev1.RKEControlPlane) (bool, error)) (_ *rkev1.RKEControlPlane, err error) {
	defer func() {
		if err != nil {
			data, newErr := GatherDebugData(clients, c)
			if newErr != nil {
				logrus.Error(newErr)
			}
			//nolint:revive
			err = fmt.Errorf("cluster %s %s wait failed on: %w\ncluster %s test data bundle: \n%s\n", c.Name, errorPrefix, err, c.Name, data)
		}
	}()

	controlPlane, err := clients.RKE.RKEControlPlane().Get(c.Namespace, c.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("%s wait did not succeed : %w", errorPrefix, err)
	}

	err = wait.Object(clients.Ctx, clients.RKE.RKEControlPlane().Watch, controlPlane, func(obj runtime.Object) (bool, error) {
		controlPlane = obj.(*rkev1.RKEControlPlane)
		return rkeControlPlaneCheckFunc(controlPlane)
	})
	if err != nil {
		return nil, fmt.Errorf("%s wait did not succeed : %w", errorPrefix, err)
	}

	return controlPlane, nil
}

func WaitForDelete(clients *clients.Clients, c *provisioningv1api.Cluster) (_ *provisioningv1api.Cluster, err error) {
	defer func() {
		if err != nil {
			data, newErr := GatherDebugData(clients, c)
			if newErr != nil {
				logrus.Error(newErr)
			}
			//nolint:revive
			err = fmt.Errorf("cluster %s delete wait failed on: %w\ncluster %s test data bundle: \n%s\n", c.Name, err, c.Name, data)
		}
	}()

	err = wait.Object(clients.Ctx, clients.Provisioning.Cluster().Watch, c, func(obj runtime.Object) (bool, error) {
		c = obj.(*provisioningv1api.Cluster)
		return !c.DeletionTimestamp.IsZero(), nil
	})
	if err != nil {
		return nil, fmt.Errorf("cluster not deleted: %w", err)
	}

	machines, err := Machines(clients, c)
	if err != nil {
		return nil, err
	}

	for _, machine := range machines.Items {
		if err := wait.EnsureDoesNotExist(clients.Ctx, func() (runtime.Object, error) {
			return external.GetObjectFromContractVersionedRef(clients.Ctx, clients.Client, machine.Spec.InfrastructureRef, machine.Namespace)
		}); err != nil {
			return nil, fmt.Errorf("infra machine %s/%s not deleted: %w", machine.Namespace, machine.Spec.InfrastructureRef.Name, err)
		}

		if machine.Spec.Bootstrap.ConfigRef.IsDefined() {
			if err := wait.EnsureDoesNotExist(clients.Ctx, func() (runtime.Object, error) {
				return clients.RKE.RKEBootstrap().Get(machine.Namespace, machine.Spec.Bootstrap.ConfigRef.Name, metav1.GetOptions{})
			}); err != nil {
				return nil, fmt.Errorf("bootstrap config %s/%s not deleted: %w", machine.Namespace, machine.Spec.Bootstrap.ConfigRef.Name, err)
			}
		}

		if err := wait.EnsureDoesNotExist(clients.Ctx, func() (runtime.Object, error) {
			return clients.Batch.Job().Get(machine.Namespace, machineprovision.GetJobName(machine.Name), metav1.GetOptions{})
		}); err != nil {
			return nil, fmt.Errorf("machine provision job %s/%s not deleted: %w", machine.Namespace, machineprovision.GetJobName(machine.Name), err)
		}

		if err := wait.EnsureDoesNotExist(clients.Ctx, func() (runtime.Object, error) {
			return clients.CAPI.Machine().Get(machine.Namespace, machine.Name, metav1.GetOptions{})
		}); err != nil {
			return nil, fmt.Errorf("machine not deleted: %w", err)
		}
	}

	if err := wait.EnsureDoesNotExist(clients.Ctx, func() (runtime.Object, error) {
		return clients.Mgmt.Cluster().Get(c.Status.ClusterName, metav1.GetOptions{})
	}); err != nil {
		return nil, fmt.Errorf("mgmt cluster not cleaned up: %w", err)
	}

	if err := wait.EnsureDoesNotExist(clients.Ctx, func() (runtime.Object, error) {
		return clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
	}); err != nil {
		return nil, fmt.Errorf("cluster not cleaned up: %w", err)
	}

	return c, nil
}

func CustomCommand(clients *clients.Clients, c *provisioningv1api.Cluster) (string, error) {
	err := wait.Object(clients.Ctx, clients.Provisioning.Cluster().Watch, c, func(obj runtime.Object) (bool, error) {
		c = obj.(*provisioningv1api.Cluster)
		return c.Status.ClusterName != "", nil
	})
	if err != nil {
		return "", err
	}

	// Wait for at least one CRT to exist
	var crt *v3.ClusterRegistrationToken
	for i := 0; i < 15; i++ {
		tokens, err := clients.Mgmt.ClusterRegistrationToken().List(c.Status.ClusterName, metav1.ListOptions{})
		if err != nil || len(tokens.Items) == 0 {
			logrus.Infof("[CustomCommand] Attempt %d: No CRTs found yet (err=%v)", i+1, err)
			time.Sleep(time.Second)
			continue
		}
		crt = &tokens.Items[0]
		break
	}
	if crt == nil {
		return "", fmt.Errorf("timeout waiting for CRT to be created")
	}
	err = wait.Object(clients.Ctx, func(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
		return clients.Mgmt.ClusterRegistrationToken().Watch(namespace, opts)
	}, crt, func(obj runtime.Object) (bool, error) {
		crt = obj.(*v3.ClusterRegistrationToken)
		return crt.Status.TokenSecretName != "" && crt.Status.NodeCommand != "", nil
	})
	if err != nil {
		return "", fmt.Errorf("timeout waiting for CRT TokenSecretName and NodeCommand: %w", err)
	}
	secret, err := clients.Core.Secret().Get(crt.Namespace, crt.Status.TokenSecretName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get token secret %s/%s: %w", crt.Namespace, crt.Status.TokenSecretName, err)
	}
	token := string(secret.Data["token"])
	if token == "" {
		return "", fmt.Errorf("token not found in secret %s/%s", crt.Namespace, crt.Status.TokenSecretName)
	}

	command := strings.ReplaceAll(crt.Status.InsecureNodeCommand, "{token}", token)
	return strings.Replace(command, "curl", "curl --retry-connrefused --retry-delay 5 --retry 30", -1), nil
}

func ImportCommand(clients *clients.Clients, mgmtCluster *v3.Cluster) (string, error) {
	// Wait for at least one CRT to exist.
	var crt *v3.ClusterRegistrationToken
	for i := 0; i < 15; i++ {
		tokens, err := clients.Mgmt.ClusterRegistrationToken().List(mgmtCluster.Name, metav1.ListOptions{})
		if err != nil || len(tokens.Items) == 0 {
			logrus.Infof("[ImportCommand] Attempt %d: No CRTs found yet (err=%v)", i+1, err)
			time.Sleep(time.Second)
			continue
		}
		crt = &tokens.Items[0]
		break
	}
	if crt == nil {
		return "", fmt.Errorf("timeout waiting for CRT to be created")
	}

	// Wait for the CRT to have a TokenSecretName and a populated InsecureCommand.
	// InsecureCommand contains a "{token}" placeholder that must be resolved from the secret.
	err := wait.Object(clients.Ctx, func(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
		return clients.Mgmt.ClusterRegistrationToken().Watch(namespace, opts)
	}, crt, func(obj runtime.Object) (bool, error) {
		crt = obj.(*v3.ClusterRegistrationToken)
		return crt.Status.TokenSecretName != "" && crt.Status.InsecureCommand != "", nil
	})
	if err != nil {
		return "", fmt.Errorf("timeout waiting for CRT TokenSecretName and InsecureCommand: %w", err)
	}

	secret, err := clients.Core.Secret().Get(crt.Namespace, crt.Status.TokenSecretName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get token secret %s/%s: %w", crt.Namespace, crt.Status.TokenSecretName, err)
	}
	token := string(secret.Data["token"])
	if token == "" {
		return "", fmt.Errorf("token not found in secret %s/%s", crt.Namespace, crt.Status.TokenSecretName)
	}

	command := strings.ReplaceAll(crt.Status.InsecureCommand, "{token}", token)
	return strings.Replace(command, "curl", "curl --retry-connrefused --retry-delay 5 --retry 30", -1), nil
}

func ExecOnPod(clients *clients.Clients, namespace, podName string, cmd ...string) (string, error) {
	req := clients.K8s.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(podName).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: cmd,
			Stdout:  true,
			Stderr:  true,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(clients.RESTConfig, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("failed to create executor: %w", err)
	}

	var stdout, stderr strings.Builder
	err = executor.StreamWithContext(clients.Ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return stdout.String(), fmt.Errorf("exec failed: %w: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

// getPodLogs gathers the logs from the specified pod in a manner similar to `kubectl logs`
func getPodLogs(clients *clients.Clients, podNamespace, podName string) (string, error) {
	plr := clients.K8s.CoreV1().Pods(podNamespace).GetLogs(podName, &corev1.PodLogOptions{})
	stream, err := plr.Stream(context.TODO())
	if err != nil {
		return "", fmt.Errorf("error streaming pod logs for pod %s/%s: %v", podNamespace, podName, err)
	}
	defer stream.Close()

	reader := bufio.NewScanner(stream)
	var logs string
	for reader.Scan() {
		logs = logs + fmt.Sprintf("%sSnewlineG", reader.Text())
	}
	return snapshotutil.CompressInterface(logs)
}

// countPodLogRegexOccurances gathers the logs from the specified pod in a manner similar to `kubectl logs` and counts the number of times the log matches the given regex.
func countPodLogRegexOccurances(clients *clients.Clients, podNamespace, podName, regex string) (int, error) {
	count := 0
	plr := clients.K8s.CoreV1().Pods(podNamespace).GetLogs(podName, &corev1.PodLogOptions{})
	stream, err := plr.Stream(context.TODO())
	if err != nil {
		return count, fmt.Errorf("error streaming pod logs for pod %s/%s: %v", podNamespace, podName, err)
	}
	defer stream.Close()

	re := regexp.MustCompile(regex)

	reader := bufio.NewScanner(stream)
	for reader.Scan() {
		if re.Match(reader.Bytes()) {
			count++
		}
	}
	return count, nil
}

// getPodFileContents executes a corresponding `kubectl cp` and gathers data for the purposes of helping increasing debug data.
func getPodFileContents(podNamespace, podName, podPath string) (string, error) {
	destFile := fmt.Sprintf("/tmp/%s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s/%s/%s", podNamespace, podName, podPath))))
	kcp := []string{
		"-n",
		podNamespace,
		"cp",
		fmt.Sprintf("%s:%s", podName, podPath),
		destFile,
	}

	cmd := exec.Command("kubectl", kcp...)
	if err := cmd.Run(); err != nil {
		logrus.Errorf("error running kubectl -n %s cp %s:%s %s", podNamespace, podName, podPath, destFile)
		return "", nil
	}

	file, err := os.Open(destFile)
	if err != nil {
		logrus.Errorf("error retrieving destination file %s: %v", destFile, err)
		return "", nil
	}
	defer file.Close()
	reader := bufio.NewScanner(file)
	var logs string
	for reader.Scan() {
		logs = logs + fmt.Sprintf("%sSnewlineG", reader.Text())
	}
	return snapshotutil.CompressInterface(logs)
}

// GatherDebugData gathers debug data that is relevant to the current cluster and returns a gzip compressed + base64 encoded string of the json.
func GatherDebugData(clients *clients.Clients, c *provisioningv1api.Cluster) (string, error) {
	newC, newErr := clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
	if newErr != nil {
		logrus.Errorf("failed to get cluster %s/%s to print error: %v", c.Namespace, c.Name, newErr)
		newC = nil
	}

	newControlPlane, newErr := clients.RKE.RKEControlPlane().Get(c.Namespace, c.Name, metav1.GetOptions{})
	if newErr != nil {
		logrus.Errorf("failed to get controlplane %s/%s to print error: %v", c.Namespace, c.Name, newErr)
		newControlPlane = nil
	}

	var rkeBootstraps []*rkev1.RKEBootstrap
	var infraMachines []*unstructured.Unstructured
	var podLogs = make(map[string]map[string]string)

	machines, newErr := Machines(clients, c)
	if newErr != nil {
		logrus.Errorf("failed to get machines for %s/%s to print error: %v", c.Namespace, c.Name, newErr)
	} else {
		for _, machine := range machines.Items {
			rb, newErr := clients.RKE.RKEBootstrap().Get(machine.Namespace, machine.Spec.Bootstrap.ConfigRef.Name, metav1.GetOptions{})
			if newErr != nil {
				logrus.Errorf("failed to get RKEBootstrap %s/%s to print error: %v", c.Namespace, c.Name, newErr)
			} else {
				rkeBootstraps = append(rkeBootstraps, rb)
			}

			im, newErr := external.GetObjectFromContractVersionedRef(clients.Ctx, clients.Client, machine.Spec.InfrastructureRef, machine.Namespace)
			if newErr != nil {
				logrus.Errorf("failed to get infrastructure machine %s/%s: %v", machine.Namespace, machine.Spec.InfrastructureRef.Name, newErr)
			} else {
				infraMachines = append(infraMachines, im)
				if machine.Spec.InfrastructureRef.Kind == "PodMachine" {
					// In the case of a podMachine, the pod name will be strings.ReplaceAll(infra.meta.GetName(), ".", "-")
					podName := strings.ReplaceAll(im.GetName(), ".", "-")
					podLogs[podName] = populatePodLogs(clients, newControlPlane, im.GetNamespace(), podName)
				}
			}
		}
	}

	customPods, newErr := clients.Core.Pod().List(c.Namespace, metav1.ListOptions{
		LabelSelector: "custom-cluster-name=" + c.Name,
	})
	if newErr != nil {
		logrus.Errorf("failed to list custommachine pods: %v", newErr)
	} else {
		for _, pod := range customPods.Items {
			podLogs[pod.Name] = populatePodLogs(clients, newControlPlane, pod.Namespace, pod.Name)
		}
	}

	rkeBootstrapTemplates, newErr := clients.RKE.RKEBootstrapTemplate().List(c.Namespace, metav1.ListOptions{
		LabelSelector: "cluster.x-k8s.io/cluster-name=" + c.Name,
	})
	if newErr != nil {
		logrus.Errorf("failed to get rkebootstrap templates for %s/%s to print error: %v", c.Namespace, c.Name, newErr)
	}

	mgmtCluster, newErr := clients.Mgmt.Cluster().Get(c.Status.ClusterName, metav1.GetOptions{})
	if apierrors.IsNotFound(newErr) {
		mgmtCluster = nil
	} else if newErr != nil {
		logrus.Errorf("failed to get mgmt cluster %s/%s to print error: %v", c.Namespace, c.Name, newErr)
	}

	var infraCluster *unstructured.Unstructured
	var machineDeployments *capi.MachineDeploymentList
	var machineSets *capi.MachineSetList

	capiCluster, newErr := clients.CAPI.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(newErr) {
		capiCluster = nil
	} else if newErr != nil {
		logrus.Errorf("failed to get capi cluster %s/%s to print error: %v", c.Namespace, c.Name, newErr)
	} else {
		infraCluster, newErr = external.GetObjectFromContractVersionedRef(clients.Ctx, clients.Client, capiCluster.Spec.InfrastructureRef, capiCluster.Namespace)
		if newErr != nil {
			logrus.Errorf("failed to get %s %s/%s to print error: %v",
				capiCluster.Spec.InfrastructureRef.GroupKind(), capiCluster.Namespace, capiCluster.Spec.InfrastructureRef.Name, newErr)
			infraCluster = nil
		}
		machineDeployments, newErr = clients.CAPI.MachineDeployment().List(c.Namespace, metav1.ListOptions{
			LabelSelector: "cluster.x-k8s.io/cluster-name=" + c.Name,
		})
		if newErr != nil {
			logrus.Error(newErr)
			machineDeployments = nil
		}
		machineSets, newErr = clients.CAPI.MachineSet().List(c.Namespace, metav1.ListOptions{
			LabelSelector: "cluster.x-k8s.io/cluster-name=" + c.Name,
		})
		if newErr != nil {
			logrus.Error(newErr)
			machineSets = nil
		}
	}

	snapshots, newErr := clients.RKE.ETCDSnapshot().List(c.Namespace, metav1.ListOptions{})
	if newErr != nil {
		logrus.Error(newErr)
		snapshots = nil
	}

	return snapshotutil.CompressInterface(map[string]interface{}{
		"cluster":               newC,
		"rkecontrolplane":       newControlPlane,
		"mgmtCluster":           mgmtCluster,
		"capiCluster":           capiCluster,
		"machineDeployments":    machineDeployments,
		"machineSets":           machineSets,
		"machines":              machines,
		"rkeBootstraps":         rkeBootstraps,
		"rkeBootstrapTemplates": rkeBootstrapTemplates,
		"infraCluster":          infraCluster,
		"infraMachines":         infraMachines,
		"podLogs":               podLogs,
		"snapshots":             snapshots,
	})
}

// populatePodLogs creates a map[string]string of logs that correspond to the pod in question. If the pod is an RKE2 pod, it also collects the kubelet logs from the pod filesystem.
func populatePodLogs(clients *clients.Clients, controlPlane *rkev1.RKEControlPlane, podNamespace, podName string) map[string]string {
	var logMap = make(map[string]string)

	logs, err := getPodLogs(clients, podNamespace, podName)
	if err != nil {
		logrus.Errorf("error while retrieving pod logs: %v", err)
	} else {
		logMap["logs"] = logs
	}

	if controlPlane != nil && capr.GetRuntime(controlPlane.Spec.KubernetesVersion) == capr.RuntimeRKE2 {
		kubeletLogs, err := getPodFileContents(podNamespace, podName, path.Join(capr.GetDistroDataDir(controlPlane), "agent/logs/kubelet.log"))
		if err != nil {
			logrus.Errorf("error while retrieving pod kubelet logs: %v", err)
		} else {
			logMap["kubeletLogs"] = kubeletLogs
		}
	}

	return logMap
}

// EnsureMinimalConflictsWithThreshold walks through pod logs and ensures a minimal number of conflict messages have occurred.
func EnsureMinimalConflictsWithThreshold(clients *clients.Clients, c *provisioningv1api.Cluster, threshold int) error {
	customPods, newErr := clients.Core.Pod().List(c.Namespace, metav1.ListOptions{
		LabelSelector: "custom-cluster-name=" + c.Name,
	})
	if newErr != nil {
		logrus.Errorf("failed to list custommachine pods: %v", newErr)
	} else {
		for _, pod := range customPods.Items {
			count, err := countPodLogRegexOccurances(clients, pod.Namespace, pod.Name, ConflictMessageRegex)
			if err != nil {
				return err
			}
			if count > threshold {
				return fmt.Errorf("pod %s/%s had %d occurrences of conflicts which was greater than the threshold of %d", pod.Namespace, pod.Name, count, threshold)
			}
		}
	}

	machines, newErr := Machines(clients, c)
	if newErr != nil {
		logrus.Errorf("failed to get machines for %s/%s to count conflicts: %v", c.Namespace, c.Name, newErr)
	}
	for _, machine := range machines.Items {
		im, newErr := external.GetObjectFromContractVersionedRef(clients.Ctx, clients.Client, machine.Spec.InfrastructureRef, machine.Namespace)
		if newErr != nil {
			logrus.Errorf("failed to get infrastructure machine %s/%s: %v", machine.Namespace, machine.Spec.InfrastructureRef.Name, newErr)
		} else {
			if machine.Spec.InfrastructureRef.Kind == "PodMachine" {
				// In the case of a podMachine, the pod name will be strings.ReplaceAll(infra.meta.GetName(), ".", "-")
				podName := strings.ReplaceAll(im.GetName(), ".", "-")
				count, err := countPodLogRegexOccurances(clients, im.GetNamespace(), podName, ConflictMessageRegex)
				if err != nil {
					return err
				}
				if count > threshold {
					return fmt.Errorf("pod %s/%s had %d occurrences of conflicts which was greater than the threshold of %d", im.GetNamespace(), podName, count, threshold)
				}
			}
		}
	}
	return nil
}

func generateImportedToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate cluster token: %w", err)
	}
	return fmt.Sprintf("%x", b), nil
}

// waitForServerToken polls the init node until the distro has written its server token, and returns
// it. The file only appears once the server has generated its CA and bootstrap data, which is after
// the install script has run and the service has started, so it can take several minutes.
func waitForServerToken(clients *clients.Clients, namespace, podName, dataDir string) (string, error) {
	tokenPath := path.Join(dataDir, "server", "token")

	var lastErr error
	deadline := time.Now().Add(15 * time.Minute)
	for time.Now().Before(deadline) {
		out, err := ExecOnPod(clients, namespace, podName, "cat", tokenPath)
		if token := strings.TrimSpace(out); err == nil && token != "" {
			return token, nil
		}
		lastErr = err
		select {
		case <-clients.Ctx.Done():
			return "", clients.Ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	return "", fmt.Errorf("timed out waiting for server token at %s on %s/%s: %v", tokenPath, namespace, podName, lastErr)
}

// importedNodeLabels returns labels plus the node identity labels, without modifying labels.
func importedNodeLabels(labels map[string]string, nodeName string, etcd bool) map[string]string {
	merged := maps.Clone(labels)
	if merged == nil {
		merged = map[string]string{}
	}
	merged[ImportedNodeNameLabel] = nodeName
	merged[ImportedNodeETCDLabel] = strconv.FormatBool(etcd)
	return merged
}

// importedNodeIPScript renders the shell that appends node-ip to configFile: one address per family
// in network.IPFamilies order, read off the network's interface when the node boots. The address is
// only known once the pod is running, so it cannot be baked into the config ahead of time. Tentative
// IPv6 addresses are skipped, since a service binding one fails until duplicate address detection
// completes.
func importedNodeIPScript(network *ImportedNetwork, configFile string) string {
	var families []string
	for _, family := range network.IPFamilies {
		if family == corev1.IPv6Protocol {
			families = append(families, "6")
		} else {
			families = append(families, "4")
		}
	}

	return fmt.Sprintf(`iface=%[1]s
node_ips=""
for fam in %[2]s; do
  addr=""
  for i in $(seq 1 60); do
    addr=$(ip -o -$fam addr show dev "$iface" scope global -tentative | awk '{print $4}' | cut -d/ -f1 | head -n1)
    [ -n "$addr" ] && break
    sleep 1
  done
  if [ -z "$addr" ]; then
    echo "no IPv$fam address on $iface" >&2
    exit 1
  fi
  node_ips="${node_ips:+$node_ips,}$addr"
done
echo "node-ip: $node_ips" >> %[3]s
`, network.interfaceName(), strings.Join(families, " "), configFile)
}

func importedNodeUserData(distro, k8sVersion, token, nodeName, serverURL string, isInit, cp, etcd, worker bool, dataDir string, registryCACert []byte, options ImportedClusterOptions, network *ImportedNetwork) string {
	isServer := etcd || cp
	rancherDir := fmt.Sprintf("/etc/rancher/%s", distro)
	configDir := rancherDir + "/config.yaml.d"

	var configLines []string
	configLines = append(configLines, "node-name: "+nodeName)
	if token != "" {
		configLines = append(configLines, "token: "+token)
	}

	if dataDir != "" {
		configLines = append(configLines, "data-dir: "+dataDir)
	}

	if isInit && distro == capr.RuntimeK3S {
		configLines = append(configLines, "cluster-init: true")
	}

	if !isInit && serverURL != "" {
		configLines = append(configLines, "server: "+serverURL)
	}

	if network != nil && network.Interface != "" {
		configLines = append(configLines, "flannel-iface: "+network.Interface)
	}

	if options.ExternalCloudProvider {
		// kubelet-arg+ appends to, rather than replaces, the kubelet args other drop-ins set.
		configLines = append(configLines, "kubelet-arg+:\n  - cloud-provider=external")
		if isServer {
			configLines = append(configLines, "disable-cloud-controller: true")
		}
	}

	if isServer {
		serviceCIDR, clusterCIDR := "10.45.0.0/16", "10.44.0.0/16"
		if network != nil {
			serviceCIDR, clusterCIDR = strings.Join(network.ServiceCIDRs, ","), strings.Join(network.ClusterCIDRs, ",")
		}
		configLines = append(configLines, "service-cidr: "+serviceCIDR)
		configLines = append(configLines, "cluster-cidr: "+clusterCIDR)

		// flannel masquerades IPv4 pod traffic leaving the cluster CIDR, but IPv6 only when asked.
		// Without it, a pod reaching another node's address — such as the cluster agent reaching an
		// API server on another node through the kubernetes service — goes out unencapsulated with
		// its pod address as source while the reply returns through the VXLAN tunnel, and conntrack
		// marks the reply INVALID, which the network policy controller drops: TCP connects, then
		// the TLS handshake hangs. The setting is server-side and applies to agents too.
		if network != nil && distro == capr.RuntimeK3S && slices.Contains(network.IPFamilies, corev1.IPv6Protocol) {
			if _, set := options.ServerConfig["flannel-ipv6-masq"]; !set {
				configLines = append(configLines, "flannel-ipv6-masq: true")
			}
		}

		if !etcd {
			configLines = append(configLines, "disable-etcd: true")
		}

		if !cp {
			configLines = append(configLines, "disable-apiserver: true")
			configLines = append(configLines, "disable-controller-manager: true")
			configLines = append(configLines, "disable-scheduler: true")
		}
	}

	var extraFiles strings.Builder
	if isServer {
		for _, key := range slices.Sorted(maps.Keys(options.ServerConfig)) {
			configLines = append(configLines, fmt.Sprintf("%s: %s", key, options.ServerConfig[key]))
		}
		for _, path := range slices.Sorted(maps.Keys(options.ServerFiles)) {
			extraFiles.WriteString(fmt.Sprintf("mkdir -p %s\ncat > %s <<'EXTRAFILE'\n%s\nEXTRAFILE\n",
				filepath.Dir(path), path, options.ServerFiles[path]))
		}
	}

	configContent := strings.Join(configLines, "\n")

	var nodeIPBlock string
	if network != nil {
		nodeIPBlock = importedNodeIPScript(network, configDir+"/50-test.yaml")
	}

	var installCmd, serviceName string
	if isServer {
		if distro == capr.RuntimeRKE2 {
			installCmd = fmt.Sprintf("curl -sfL https://get.rke2.io | INSTALL_RKE2_VERSION=%s sh -", k8sVersion)
			serviceName = "rke2-server"
		} else {
			installCmd = fmt.Sprintf("curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=%s INSTALL_K3S_SKIP_START=true sh -", k8sVersion)
			serviceName = "k3s"
		}
	} else {
		if distro == capr.RuntimeRKE2 {
			installCmd = fmt.Sprintf("curl -sfL https://get.rke2.io | INSTALL_RKE2_TYPE=agent INSTALL_RKE2_VERSION=%s sh -", k8sVersion)
			serviceName = "rke2-agent"
		} else {
			installCmd = fmt.Sprintf("curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=%s INSTALL_K3S_EXEC=agent INSTALL_K3S_SKIP_START=true sh -", k8sVersion)
			serviceName = "k3s-agent"
		}
	}

	var registryBlock string
	if len(registryCACert) > 0 {
		registryHost := "registry-cache.default.svc.cluster.local:5000"
		caPath := rancherDir + "/registry-ca.crt"
		registryBlock = fmt.Sprintf(
			"cat > %s/registries.yaml <<'REGISTRIES'\nmirrors:\n  docker.io:\n    endpoint:\n      - \"https://%s\"\n      - \"https://registry-1.docker.io\"\nconfigs:\n  \"%s\":\n    auth:\n      username: admin\n      password: admin\n    tls:\n      ca_file: %s\nREGISTRIES\ncat > %s <<'CACERT'\n%s\nCACERT\n",
			rancherDir, registryHost, registryHost, caPath, caPath, string(registryCACert))
	}

	return fmt.Sprintf("#!/usr/bin/env sh\nset -e\nmkdir -p %s\ncat > %s/50-test.yaml <<'EOF'\n%s\nEOF\n%s%s%s%s\nsystemctl enable %s\nsystemctl start %s\n",
		configDir, configDir, configContent, nodeIPBlock, extraFiles.String(), registryBlock, installCmd, serviceName, serviceName)
}

func appendGlobalRegistrySelector(c *provisioningv1api.Cluster, host string) {
	if c.Spec.RKEConfig == nil {
		c.Spec.RKEConfig = &provisioningv1api.RKEConfig{}
	}
	if c.Spec.RKEConfig.MachineSelectorConfig == nil {
		c.Spec.RKEConfig.MachineSelectorConfig = []rkev1.RKESystemConfig{}
	}

	sel := rkev1.RKESystemConfig{
		Config: rkev1.GenericMap{
			Data: map[string]any{
				"system-default-registry": host,
			},
		},
	}
	c.Spec.RKEConfig.MachineSelectorConfig = append(c.Spec.RKEConfig.MachineSelectorConfig, sel)
}
