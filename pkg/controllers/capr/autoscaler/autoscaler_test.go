package autoscaler

import (
	"testing"

	fleet "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

type autoscalerSuite struct {
	suite.Suite

	mockCtrl                   *gomock.Controller
	h                          *autoscalerHandler
	capiClusterCache           *fake.MockCacheInterface[*capi.Cluster]
	capiMachineCache           *fake.MockCacheInterface[*capi.Machine]
	capiMachineDeploymentCache *fake.MockCacheInterface[*capi.MachineDeployment]
	clusterClient              *fake.MockClientInterface[*provv1.Cluster, *provv1.ClusterList]
	clusterCache               *fake.MockCacheInterface[*provv1.Cluster]
	globalRole                 *fake.MockNonNamespacedClientInterface[*v3.GlobalRole, *v3.GlobalRoleList]
	globalRoleCache            *fake.MockNonNamespacedCacheInterface[*v3.GlobalRole]
	globalRoleBinding          *fake.MockNonNamespacedClientInterface[*v3.GlobalRoleBinding, *v3.GlobalRoleBindingList]
	globalRoleBindingCache     *fake.MockNonNamespacedCacheInterface[*v3.GlobalRoleBinding]
	user                       *fake.MockNonNamespacedClientInterface[*v3.User, *v3.UserList]
	userCache                  *fake.MockNonNamespacedCacheInterface[*v3.User]
	token                      *fake.MockNonNamespacedClientInterface[*v3.Token, *v3.TokenList]
	tokenCache                 *fake.MockNonNamespacedCacheInterface[*v3.Token]
	secret                     *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]
	secretCache                *fake.MockCacheInterface[*corev1.Secret]
	helmOp                     *fake.MockControllerInterface[*fleet.HelmOp, *fleet.HelmOpList]
	helmOpCache                *fake.MockCacheInterface[*fleet.HelmOp]
}

func TestAutoscaler(t *testing.T) {
	suite.Run(t, &autoscalerSuite{})
}

func (s *autoscalerSuite) SetupTest() {
	// Create mock controller
	s.mockCtrl = gomock.NewController(s.T())

	// Create mock caches and clients using the correct types from the autoscaler.go file
	s.capiClusterCache = fake.NewMockCacheInterface[*capi.Cluster](s.mockCtrl)
	s.capiMachineCache = fake.NewMockCacheInterface[*capi.Machine](s.mockCtrl)
	s.capiMachineDeploymentCache = fake.NewMockCacheInterface[*capi.MachineDeployment](s.mockCtrl)
	s.clusterClient = fake.NewMockClientInterface[*provv1.Cluster, *provv1.ClusterList](s.mockCtrl)
	s.clusterCache = fake.NewMockCacheInterface[*provv1.Cluster](s.mockCtrl)
	s.globalRole = fake.NewMockNonNamespacedClientInterface[*v3.GlobalRole, *v3.GlobalRoleList](s.mockCtrl)
	s.globalRoleCache = fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](s.mockCtrl)
	s.globalRoleBinding = fake.NewMockNonNamespacedClientInterface[*v3.GlobalRoleBinding, *v3.GlobalRoleBindingList](s.mockCtrl)
	s.globalRoleBindingCache = fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRoleBinding](s.mockCtrl)
	s.user = fake.NewMockNonNamespacedClientInterface[*v3.User, *v3.UserList](s.mockCtrl)
	s.userCache = fake.NewMockNonNamespacedCacheInterface[*v3.User](s.mockCtrl)
	s.token = fake.NewMockNonNamespacedClientInterface[*v3.Token, *v3.TokenList](s.mockCtrl)
	s.tokenCache = fake.NewMockNonNamespacedCacheInterface[*v3.Token](s.mockCtrl)
	s.secret = fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](s.mockCtrl)
	s.secretCache = fake.NewMockCacheInterface[*corev1.Secret](s.mockCtrl)
	s.helmOp = fake.NewMockControllerInterface[*fleet.HelmOp, *fleet.HelmOpList](s.mockCtrl)
	s.helmOpCache = fake.NewMockCacheInterface[*fleet.HelmOp](s.mockCtrl)

	s.h = &autoscalerHandler{
		capiClusterCache:           s.capiClusterCache,
		capiMachineCache:           s.capiMachineCache,
		capiMachineDeploymentCache: s.capiMachineDeploymentCache,
		clusterClient:              s.clusterClient,
		clusterCache:               s.clusterCache,
		globalRole:                 s.globalRole,
		globalRoleCache:            s.globalRoleCache,
		globalRoleBinding:          s.globalRoleBinding,
		globalRoleBindingCache:     s.globalRoleBindingCache,
		user:                       s.user,
		userCache:                  s.userCache,
		token:                      s.token,
		tokenCache:                 s.tokenCache,
		secret:                     s.secret,
		secretCache:                s.secretCache,
		helmOp:                     s.helmOp,
		helmOpCache:                s.helmOpCache,
		dynamicClient:              nil,
	}
}

func (s *autoscalerSuite) TearDownTest() {
	if s.mockCtrl != nil {
		s.mockCtrl.Finish()
	}
}
