package cluster

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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
	"sync"
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
	"k8s.io/client-go/kubernetes"
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

// NewImportedJoinNodePod creates one more node for a running imported cluster, set up the way
// NewImportedClusterPods sets up its joining nodes: it joins through serverURL with token, and runs
// the roles of pool, whose Quantity is ignored. opts should be the options the cluster was created
// with, so the new node is configured like the rest — a server that does not get the cluster's
// etcd-s3 settings, for instance, would not see its S3 snapshots. The pod is returned as soon as it
// is created; it is up to the caller to wait for the node to join.
//
// token is the full server token (K10<ca-hash>::server:<password>), as read from a running server
// with ImportedServerToken.
func NewImportedJoinNodePod(clients *clients.Clients, namespace, k8sVersion, nodeName, serverURL, token string, pool ImportedNodePool, labels map[string]string, registryCACert []byte, opts ...ImportedClusterOptions) (*corev1.Pod, error) {
	var options ImportedClusterOptions
	if len(opts) > 1 {
		return nil, fmt.Errorf("at most one ImportedClusterOptions may be supplied, got %d", len(opts))
	} else if len(opts) == 1 {
		options = opts[0]
	}

	var network *ImportedNetwork
	if options.Network != nil {
		resolved, err := options.Network.resolve()
		if err != nil {
			return nil, fmt.Errorf("invalid imported network: %w", err)
		}
		network = &resolved
	}

	script := importedNodeUserData(capr.GetRuntime(k8sVersion), k8sVersion, token, nodeName, serverURL, false, pool.ControlPlane, pool.ETCD, pool.Worker, pool.DistroDataDir, registryCACert, options, network)
	pod, err := systemdnode.New(clients, namespace, script, importedNodeLabels(labels, nodeName, pool.ETCD), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create node %s: %w", nodeName, err)
	}
	return pod, nil
}

// ImportedServerToken reads the full server token off a running server node of an imported cluster,
// waiting for the distro to have written it. dataDir is the node's distro data directory.
func ImportedServerToken(clients *clients.Clients, namespace, podName, dataDir string) (string, error) {
	return waitForServerToken(clients, namespace, podName, dataDir)
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
			registryConfig, err := registry.CreateOrGetRegistry(clients, cluster.Namespace, "registry-cache")
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
	return ExecOnPodContext(clients.Ctx, clients, namespace, podName, cmd...)
}

// ExecOnPodContext is ExecOnPod, stopping the exec when ctx is done.
func ExecOnPodContext(ctx context.Context, clients *clients.Clients, namespace, podName string, cmd ...string) (string, error) {
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

	// When ctx is done, StreamWithContext returns while its stream goroutine may still be writing, so the buffers
	// have to be safe to read concurrently.
	var stdout, stderr lockedBuilder
	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return stdout.String(), fmt.Errorf("exec failed: %w: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

// lockedBuilder is a strings.Builder that is safe for one writer and concurrent readers.
type lockedBuilder struct {
	lock    sync.Mutex
	builder strings.Builder
}

func (b *lockedBuilder) Write(p []byte) (int, error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.builder.Write(p)
}

func (b *lockedBuilder) String() string {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.builder.String()
}

// podLogsTimeout bounds a single pod log fetch on the failure path, so a wedged stream can't stall cleanup.
const podLogsTimeout = time.Minute

// getPodLogs gathers the logs from the specified pod in a manner similar to `kubectl logs`
func getPodLogs(clients *clients.Clients, podNamespace, podName string) (string, error) {
	ctx, cancel := context.WithTimeout(clients.Ctx, podLogsTimeout)
	defer cancel()
	return PodLogs(ctx, clients.K8s, podNamespace, podName, &corev1.PodLogOptions{})
}

// PodLogs gathers the logs of a pod in a manner similar to `kubectl logs`, compressed the same way as GatherDebugData's pod
// logs. If the stream breaks partway, the logs read so far are kept and the read error is appended to them.
func PodLogs(ctx context.Context, kube kubernetes.Interface, podNamespace, podName string, opts *corev1.PodLogOptions) (string, error) {
	stream, err := kube.CoreV1().Pods(podNamespace).GetLogs(podName, opts).Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("error streaming pod logs for pod %s/%s: %w", podNamespace, podName, err)
	}
	defer stream.Close()

	logs, err := io.ReadAll(stream)
	if err != nil {
		logrus.Errorf("error reading pod logs for pod %s/%s, keeping the %d bytes read: %v", podNamespace, podName, len(logs), err)
		logs = fmt.Appendf(logs, "\nerror reading logs: %v\n", err)
	}
	return snapshotutil.CompressInterface(strings.ReplaceAll(string(logs), "\n", "SnewlineG"))
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
	// Allow long lines, which the default 64KiB limit would stop the scan on and undercount.
	reader.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for reader.Scan() {
		if re.Match(reader.Bytes()) {
			count++
		}
	}
	if err := reader.Err(); err != nil {
		return count, fmt.Errorf("error reading pod logs for pod %s/%s: %w", podNamespace, podName, err)
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

	ctx, cancel := context.WithTimeout(context.Background(), podLogsTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kubectl", kcp...)
	if err := cmd.Run(); err != nil {
		logrus.Errorf("error running kubectl -n %s cp %s:%s %s", podNamespace, podName, podPath, destFile)
		return "", nil
	}

	logs, err := os.ReadFile(destFile)
	if err != nil {
		logrus.Errorf("error retrieving destination file %s: %v", destFile, err)
		return "", nil
	}
	return snapshotutil.CompressInterface(strings.ReplaceAll(string(logs), "\n", "SnewlineG"))
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

// importedNodeEnvScript detects the distro inside an imported node pod and defines kc (kubectl against the downstream
// cluster) and cri (crictl against the node's containerd). A node's data-dir may be overridden in its config, so it is
// resolved on the node rather than assumed.
const importedNodeEnvScript = `if [ -d /etc/rancher/rke2 ]; then
  dd=$(cat /etc/rancher/rke2/config.yaml /etc/rancher/rke2/config.yaml.d/*.yaml 2>/dev/null | sed -n 's/^data-dir:[[:space:]]*//p' | tail -n1 | tr -d "\"'")
  dd=${dd:-/var/lib/rancher/rke2}
  kubeconfig=/etc/rancher/rke2/rke2.yaml
  kc() { KUBECONFIG=$kubeconfig "$dd/bin/kubectl" --request-timeout=20s "$@"; }
  cri() { "$dd/bin/crictl" --runtime-endpoint unix:///run/k3s/containerd/containerd.sock "$@"; }
else
  kubeconfig=/etc/rancher/k3s/k3s.yaml
  kc() { KUBECONFIG=$kubeconfig k3s kubectl --request-timeout=20s "$@"; }
  cri() { k3s crictl "$@"; }
fi
`

// importedKubectlScript runs kubectl against the downstream cluster from inside an imported node pod, passing its arguments
// through. It fails on nodes without a kubeconfig, i.e. agent-only nodes.
const importedKubectlScript = importedNodeEnvScript + `if [ ! -f "$kubeconfig" ]; then echo "` + notServerNodeMessage + `" >&2; exit 3; fi
kc "$@"`

// importedNodeDiskCommands read the same information as the kubectl queries, but from the node itself, so they still work when
// the downstream apiserver is down.
// notServerNodeMessage is what importedKubectlScript prints on an agent-only node.
const notServerNodeMessage = "not a server node"

// importedNodeDiskCommand is one command run by gatherImportedNodeDiskData, stored under key.
type importedNodeDiskCommand struct {
	key, script string
}

// importedNodeDiskCommands are run in order, most useful first, so a node that runs out of time loses the least.
var importedNodeDiskCommands = []importedNodeDiskCommand{
	// Includes rotated, and possibly compressed, files, which hold the previous container instances' logs.
	{key: "agentLogs", script: `for f in $(find /var/log/pods -path '/var/log/pods/cattle-system_cattle-cluster-agent-*' -name '*.log*' -type f 2>/dev/null | sort); do
  echo "===== $f"
  case "$f" in *.gz) zcat "$f" ;; *) cat "$f" ;; esac
done`},
	{key: "runtimePods", script: `echo "===== crictl pods"; cri pods 2>&1
echo "===== crictl ps -a"; cri ps -a 2>&1`},
}

const (
	// importedKubectlPhaseTimeout caps the kubectl phase of GatherImportedDownstreamData. It runs in the clients' OnClose
	// hooks, so without a cap a cluster whose nodes don't answer could hold up cleanup for each node in turn. The disk
	// phase has its own budget (see gatherImportedDiskData) so a wedged node in one can't starve the other.
	importedKubectlPhaseTimeout = 3 * time.Minute

	// importedDiskCommandTimeout and importedKubectlTimeout are passed to timeout(1) inside the node pod. The exec itself
	// gets a little longer, so the in-pod timeout fires first and its partial output is kept.
	importedDiskCommandTimeout = 120 * time.Second
	importedKubectlTimeout     = 60 * time.Second
	importedExecGrace          = 15 * time.Second
)

// execOnPodWithTimeout runs cmd on the pod under timeout(1), and bounds the exec stream as well in case it wedges.
func execOnPodWithTimeout(ctx context.Context, clients *clients.Clients, pod corev1.Pod, timeout time.Duration, cmd ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout+importedExecGrace)
	defer cancel()

	seconds := strconv.Itoa(int(timeout.Seconds()))
	return ExecOnPodContext(ctx, clients, pod.Namespace, pod.Name, append([]string{"timeout", seconds}, cmd...)...)
}

// GatherImportedDownstreamData collects the downstream cluster's view of an imported cluster whose nodes run as pods in
// namespace, focused on which pods are running and what the cattle-cluster-agent logged. It is gathered two ways so that as
// much as possible survives a broken cluster:
//
//   - from every node's disk (nodes): the containerd pods and containers, and the cattle-cluster-agent pod logs under
//     /var/log/pods. This doesn't need the apiserver.
//   - through kubectl on the first server node that answers (pods, podLogs): every downstream pod and the cattle-cluster-agent
//     logs. This goes around Rancher, since the agent tunnel it would use is often what's broken.
//
// Logs are compressed the same way as GatherDebugData's pod logs, so DEBUG.md applies.
func GatherImportedDownstreamData(clients *clients.Clients, namespace string) map[string]any {
	data := map[string]any{}

	nodePods, err := clients.Core.Pod().List(namespace, metav1.ListOptions{LabelSelector: ImportedNodeNameLabel})
	if err != nil {
		data["errors"] = []string{fmt.Sprintf("failed to list imported node pods in %s: %v", namespace, err)}
		return data
	}

	// The kubectl view goes first: it holds the cattle-cluster-agent logs, which are usually what explains a failure.
	errs := gatherImportedKubectlData(clients, nodePods.Items, data)
	data["nodes"] = gatherImportedDiskData(clients, nodePods.Items)
	if len(errs) > 0 {
		data["errors"] = errs
	}

	return data
}

// gatherImportedKubectlData fills data with the downstream pods, cattle-cluster-agent logs, ClusterRepos and cattle-system
// ReplicaSets, queried through kubectl on the server nodes. Each query moves on to the next server node if one fails, so
// an apiserver that flaps partway through still yields as much as possible. It returns the errors it hit along the way.
func gatherImportedKubectlData(clients *clients.Clients, nodePods []corev1.Pod, data map[string]any) []string {
	ctx, cancel := context.WithTimeout(clients.Ctx, importedKubectlPhaseTimeout)
	defer cancel()

	var errs []string
	agentOnly := map[string]bool{}
	queriedFrom := map[string]string{}
	data["queriedFrom"] = queriedFrom

	// onAnyServer runs query with kubectl on each server node in turn, until one succeeds.
	onAnyServer := func(what string, query func(kubectl func(args ...string) (string, error)) error) {
		for _, nodePod := range nodePods {
			if agentOnly[nodePod.Name] {
				continue
			}
			kubectl := func(args ...string) (string, error) {
				cmd := append([]string{"sh", "-c", importedKubectlScript, "kubectl"}, args...)
				return execOnPodWithTimeout(ctx, clients, nodePod, importedKubectlTimeout, cmd...)
			}
			err := query(kubectl)
			if err == nil {
				queriedFrom[what] = nodePod.Name
				return
			}
			if strings.Contains(err.Error(), notServerNodeMessage) {
				agentOnly[nodePod.Name] = true
				continue
			}
			errs = append(errs, fmt.Sprintf("%s: failed to get downstream %s: %v", nodePod.Name, what, err))
		}
	}

	onAnyServer("pods", func(kubectl func(args ...string) (string, error)) error {
		out, err := kubectl("get", "pods", "-A", "-o", "json")
		if err != nil {
			return err
		}
		pods := &corev1.PodList{}
		if err := json.Unmarshal([]byte(out), pods); err != nil {
			return fmt.Errorf("decoding pods: %w", err)
		}

		podLogs := map[string]map[string]string{}
		for _, pod := range pods.Items {
			if pod.Namespace == "cattle-system" && pod.Labels["app"] == "cattle-cluster-agent" {
				podLogs[pod.Name] = importedDownstreamPodLogs(kubectl, pod)
			}
		}
		data["pods"] = pods
		data["podLogs"] = podLogs
		return nil
	})

	// ClusterRepo status shows whether rancher-charts finished downloading, and the cattle-system ReplicaSets show whether
	// the cluster agent was redeployed while it was. Raw JSON keeps the objects as they were served.
	rawQueries := []struct {
		key  string
		args []string
	}{
		{key: "clusterRepos", args: []string{"get", "clusterrepos.catalog.cattle.io", "-o", "json"}},
		{key: "replicaSets", args: []string{"-n", "cattle-system", "get", "replicasets", "-o", "json"}},
	}
	for _, q := range rawQueries {
		onAnyServer(q.key, func(kubectl func(args ...string) (string, error)) error {
			out, err := kubectl(q.args...)
			if err != nil {
				return err
			}
			if !json.Valid([]byte(out)) {
				return fmt.Errorf("output is not JSON: %s", out)
			}
			data[q.key] = json.RawMessage(out)
			return nil
		})
	}
	return errs
}

// gatherImportedDiskData runs gatherImportedNodeDiskData on every node at once, so a wedged node only costs its own data.
func gatherImportedDiskData(clients *clients.Clients, nodePods []corev1.Pod) map[string]map[string]string {
	// Nodes are gathered in parallel, so the budget only has to fit one node's worst case.
	budget := podLogsTimeout + time.Duration(len(importedNodeDiskCommands))*(importedDiskCommandTimeout+importedExecGrace) + 30*time.Second
	ctx, cancel := context.WithTimeout(clients.Ctx, budget)
	defer cancel()

	var lock sync.Mutex
	var wg sync.WaitGroup
	nodes := map[string]map[string]string{}
	for _, nodePod := range nodePods {
		wg.Add(1)
		go func() {
			defer wg.Done()
			nodeData := gatherImportedNodeDiskData(ctx, clients, nodePod)
			lock.Lock()
			defer lock.Unlock()
			nodes[nodePod.Name] = nodeData
		}()
	}
	wg.Wait()
	return nodes
}

// gatherImportedNodeDiskData runs importedNodeDiskCommands on a node pod. Failures are recorded in place of the output so one
// broken command doesn't hide the rest.
func gatherImportedNodeDiskData(ctx context.Context, clients *clients.Clients, nodePod corev1.Pod) map[string]string {
	logs := map[string]string{
		"nodeName": nodePod.Labels[ImportedNodeNameLabel],
	}
	// The node pod's stdout carries rancher-system-agent's output.
	logsCtx, cancel := context.WithTimeout(ctx, podLogsTimeout)
	podLogs, err := PodLogs(logsCtx, clients.K8s, nodePod.Namespace, nodePod.Name, &corev1.PodLogOptions{})
	cancel()
	if err != nil {
		logrus.Errorf("failed to collect logs for pod %s/%s: %v", nodePod.Namespace, nodePod.Name, err)
	} else {
		logs["logs"] = podLogs
	}
	for _, cmd := range importedNodeDiskCommands {
		key := cmd.key
		out, err := execOnPodWithTimeout(ctx, clients, nodePod, importedDiskCommandTimeout, "sh", "-c", importedNodeEnvScript+cmd.script)
		if err != nil {
			out = fmt.Sprintf("%s\nerror collecting %s: %v", out, key, err)
		}
		compressed, err := snapshotutil.CompressInterface(strings.ReplaceAll(out, "\n", "SnewlineG"))
		if err != nil {
			logrus.Errorf("failed to compress %s for pod %s/%s: %v", key, nodePod.Namespace, nodePod.Name, err)
			continue
		}
		logs[key] = compressed
	}
	return logs
}

// importedDownstreamPodLogs collects the logs of a downstream pod, and those of the previous container instance if the pod has
// restarted, since an agent that crashloops after a restore usually logs the cause before exiting.
func importedDownstreamPodLogs(kubectl func(args ...string) (string, error), pod corev1.Pod) map[string]string {
	logMap := map[string]string{}

	collect := func(key string, extraArgs ...string) {
		out, err := kubectl(append([]string{"-n", pod.Namespace, "logs", pod.Name, "--all-containers"}, extraArgs...)...)
		if err != nil {
			out = fmt.Sprintf("%s\nerror collecting %s: %v", out, key, err)
		}
		compressed, err := snapshotutil.CompressInterface(strings.ReplaceAll(out, "\n", "SnewlineG"))
		if err != nil {
			logrus.Errorf("failed to compress %s for downstream pod %s/%s: %v", key, pod.Namespace, pod.Name, err)
			return
		}
		logMap[key] = compressed
	}

	collect("logs")
	for _, status := range pod.Status.ContainerStatuses {
		if status.RestartCount > 0 {
			collect("previousLogs", "--previous")
			break
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

	var containerdBlock string
	if dataDir != "" {
		configLines = append(configLines, "data-dir: "+dataDir)
		// systemdnode.New only mounts the containerd config drop-in under the default data-dir.
		containerdBlock = fmt.Sprintf("mkdir -p %[1]s/agent/etc/containerd/config-v3.toml.d\ncp %[2]s %[1]s/agent/etc/containerd/config-v3.toml.d/\n",
			dataDir, systemdnode.ContainerdDropInPath(distro))
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

	return fmt.Sprintf("#!/usr/bin/env sh\nset -e\nmkdir -p %s\ncat > %s/50-test.yaml <<'EOF'\n%s\nEOF\n%s%s%s%s%s\nsystemctl enable %s\nsystemctl start %s\n",
		configDir, configDir, configContent, nodeIPBlock, extraFiles.String(), registryBlock, containerdBlock, installCmd, serviceName, serviceName)
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
