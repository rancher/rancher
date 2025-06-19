package node

import (
	"context"
	"os"

	"github.com/rancher/norman/objectclient"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/encryptedstore"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/nodeconfig"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/types/config/systemtokens"
	"github.com/rancher/rancher/pkg/user"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	amazonec2                       = "amazonec2"
	userNodeRemoveCleanupAnnotation = "cleanup.cattle.io/user-node-remove"
	userNodeRemoveFinalizerPrefix   = "clusterscoped.controller.cattle.io/user-node-remove_"
)

func Register(ctx context.Context, management *config.ManagementContext, clusterManager *clustermanager.Manager) {
	secretStore, err := nodeconfig.NewStore(management.Core.Namespaces(""), management.Core)
	if err != nil {
		logrus.Fatal(err)
	}

	nodeClient := management.Management.Nodes("")

	nodeLifecycle := &Lifecycle{
		ctx:                       ctx,
		systemAccountManager:      systemaccount.NewManager(management),
		secretStore:               secretStore,
		nodeClient:                nodeClient,
		nodeTemplateClient:        management.Management.NodeTemplates(""),
		nodePoolLister:            management.Management.NodePools("").Controller().Lister(),
		nodePoolController:        management.Management.NodePools("").Controller(),
		nodeTemplateGenericClient: management.Management.NodeTemplates("").ObjectClient().UnstructuredClient(),
		configMapGetter:           management.K8sClient.CoreV1(),
		clusterLister:             management.Management.Clusters("").Controller().Lister(),
		schemaLister:              management.Management.DynamicSchemas("").Controller().Lister(),
		secretLister:              management.Core.Secrets("").Controller().Lister(),
		userManager:               management.UserManager,
		systemTokens:              management.SystemTokens,
		clusterManager:            clusterManager,
		devMode:                   os.Getenv("CATTLE_DEV_MODE") != "",
	}

	nodeClient.AddHandler(ctx, "node-controller-sync", nodeLifecycle.sync)
}

type Lifecycle struct {
	ctx                       context.Context
	systemAccountManager      *systemaccount.Manager
	secretStore               *encryptedstore.GenericEncryptedStore
	nodeTemplateGenericClient objectclient.GenericClient
	nodeClient                v3.NodeInterface
	nodeTemplateClient        v3.NodeTemplateInterface
	nodePoolLister            v3.NodePoolLister
	nodePoolController        v3.NodePoolController
	configMapGetter           typedv1.ConfigMapsGetter
	clusterLister             v3.ClusterLister
	schemaLister              v3.DynamicSchemaLister
	secretLister              corev1.SecretLister
	userManager               user.Manager
	systemTokens              systemtokens.Interface
	clusterManager            *clustermanager.Manager
	devMode                   bool
}

func (m *Lifecycle) sync(_ string, machine *apimgmtv3.Node) (runtime.Object, error) {
	if machine == nil {
		return nil, nil
	}

	if machine.Annotations[userNodeRemoveCleanupAnnotation] != "true" {
		machine = m.userNodeRemoveCleanup(machine)
	}

	return m.nodeClient.Update(machine)
}
