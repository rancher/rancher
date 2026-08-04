package operations

import (
	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta2"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
}

func (s *stubMgmtInterface) Node() mgmtcontrollers.NodeController {
	return &stubNodeController{cache: s.nodeCache}
}

type stubNodeController struct {
	mgmtcontrollers.NodeController
	cache generic.CacheInterface[*mgmtv3.Node]
}

func (s *stubNodeController) Cache() generic.CacheInterface[*mgmtv3.Node] {
	return s.cache
}
