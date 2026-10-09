package operations

import (
	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta2"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	provcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// stubCoreInterface minimal implementation to satisfy wrangler.Context.Core.
type stubCoreInterface struct {
	wcorev1.Interface
	secretCache generic.CacheInterface[*corev1.Secret]
}

func (s *stubCoreInterface) Secret() wcorev1.SecretController {
	return &stubSecretController{cache: s.secretCache}
}

type stubSecretController struct {
	wcorev1.SecretController
	cache generic.CacheInterface[*corev1.Secret]
}

func (s *stubSecretController) List(namespace string, opts metav1.ListOptions) (*corev1.SecretList, error) {
	secrets, err := s.cache.List(namespace, nil)
	if err != nil {
		return nil, err
	}
	items := make([]corev1.Secret, len(secrets))
	for i, sec := range secrets {
		items[i] = *sec
	}
	return &corev1.SecretList{Items: items}, nil
}

// stubCAPIInterface minimal implementation to satisfy CAPIContext.CAPI.
type stubCAPIInterface struct {
	capicontrollers.Interface
	machineCache generic.CacheInterface[*capi.Machine]
	clusters     *stubCAPIClusterController
}

func (s *stubCAPIInterface) Cluster() capicontrollers.ClusterController {
	return s.clusters
}

// stubCAPIClusterController serves one CAPI Cluster through its cache, and records every Update,
// which it also makes the cluster the cache serves from then on.
type stubCAPIClusterController struct {
	capicontrollers.ClusterController
	cluster *capi.Cluster
	updates []*capi.Cluster
}

func (s *stubCAPIClusterController) Cache() generic.CacheInterface[*capi.Cluster] {
	return &stubCAPIClusterCache{controller: s}
}

func (s *stubCAPIClusterController) Update(cluster *capi.Cluster) (*capi.Cluster, error) {
	s.updates = append(s.updates, cluster.DeepCopy())
	s.cluster = cluster.DeepCopy()
	return cluster, nil
}

type stubCAPIClusterCache struct {
	generic.CacheInterface[*capi.Cluster]
	controller *stubCAPIClusterController
}

func (c *stubCAPIClusterCache) Get(_, _ string) (*capi.Cluster, error) {
	return c.controller.cluster.DeepCopy(), nil
}

func (s *stubCAPIInterface) Machine() capicontrollers.MachineController {
	return &stubMachineController{cache: s.machineCache}
}

type stubMachineController struct {
	capicontrollers.MachineController
	cache generic.CacheInterface[*capi.Machine]
}

func (s *stubMachineController) Cache() generic.CacheInterface[*capi.Machine] {
	return s.cache
}

// stubMgmtInterface minimal implementation to satisfy wrangler.Context.Mgmt.
type stubMgmtInterface struct {
	mgmtcontrollers.Interface
	nodeCache generic.CacheInterface[*mgmtv3.Node]
	clusters  *stubClusterController
}

func (s *stubMgmtInterface) Node() mgmtcontrollers.NodeController {
	return &stubNodeController{cache: s.nodeCache}
}

func (s *stubMgmtInterface) Cluster() mgmtcontrollers.ClusterController {
	return s.clusters
}

// stubClusterController serves mgmt v3 Clusters from a map and records every Update, so a test can
// assert both what an adapter wrote and that it did not write at all.
type stubClusterController struct {
	mgmtcontrollers.ClusterController
	clusters  map[string]*mgmtv3.Cluster
	updates   []*mgmtv3.Cluster
	getErr    error
	updateErr error
}

func (s *stubClusterController) Get(name string, _ metav1.GetOptions) (*mgmtv3.Cluster, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	cluster, ok := s.clusters[name]
	if !ok {
		return nil, apierrors.NewNotFound(schema.GroupResource{Group: "management.cattle.io", Resource: "clusters"}, name)
	}
	return cluster.DeepCopy(), nil
}

func (s *stubClusterController) Update(cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
	s.updates = append(s.updates, cluster.DeepCopy())
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	if s.clusters == nil {
		s.clusters = map[string]*mgmtv3.Cluster{}
	}
	s.clusters[cluster.Name] = cluster.DeepCopy()
	return cluster, nil
}

type stubNodeController struct {
	mgmtcontrollers.NodeController
	cache generic.CacheInterface[*mgmtv3.Node]
}

func (s *stubNodeController) Cache() generic.CacheInterface[*mgmtv3.Node] {
	return s.cache
}

// stubProvisioningClusterController serves a provisioning Cluster and records every Update. Serve it
// through stubProvisioningInterface.
type stubProvisioningClusterController struct {
	provcontrollers.ClusterController
	cluster *provv1.Cluster
	updates []*provv1.Cluster
}

func (s *stubProvisioningClusterController) Get(namespace, name string, _ metav1.GetOptions) (*provv1.Cluster, error) {
	if s.cluster == nil || s.cluster.Namespace != namespace || s.cluster.Name != name {
		return nil, apierrors.NewNotFound(schema.GroupResource{Group: "provisioning.cattle.io", Resource: "clusters"}, name)
	}
	return s.cluster.DeepCopy(), nil
}

func (s *stubProvisioningClusterController) Update(cluster *provv1.Cluster) (*provv1.Cluster, error) {
	s.updates = append(s.updates, cluster.DeepCopy())
	s.cluster = cluster.DeepCopy()
	return cluster, nil
}

// stubRKEInterface serves one RKEControlPlane through stubRKEControlPlaneController.
type stubRKEInterface struct {
	rkecontrollers.Interface
	controlPlanes *stubRKEControlPlaneController
}

func (s *stubRKEInterface) RKEControlPlane() rkecontrollers.RKEControlPlaneController {
	return s.controlPlanes
}

// stubRKEControlPlaneController serves an RKEControlPlane and records every Update.
type stubRKEControlPlaneController struct {
	rkecontrollers.RKEControlPlaneController
	controlPlane *rkev1.RKEControlPlane
	updates      []*rkev1.RKEControlPlane
}

func (s *stubRKEControlPlaneController) Get(namespace, name string, _ metav1.GetOptions) (*rkev1.RKEControlPlane, error) {
	if s.controlPlane == nil || s.controlPlane.Namespace != namespace || s.controlPlane.Name != name {
		return nil, apierrors.NewNotFound(schema.GroupResource{Group: "rke.cattle.io", Resource: "rkecontrolplanes"}, name)
	}
	return s.controlPlane.DeepCopy(), nil
}

func (s *stubRKEControlPlaneController) Update(controlPlane *rkev1.RKEControlPlane) (*rkev1.RKEControlPlane, error) {
	s.updates = append(s.updates, controlPlane.DeepCopy())
	s.controlPlane = controlPlane.DeepCopy()
	return controlPlane, nil
}
