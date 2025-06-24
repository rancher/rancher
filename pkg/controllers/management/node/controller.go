package node

import (
	"context"
	"os"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/types/config/systemtokens"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	amazonec2                       = "amazonec2"
	userNodeRemoveCleanupAnnotation = "cleanup.cattle.io/user-node-remove"
	userNodeRemoveFinalizerPrefix   = "clusterscoped.controller.cattle.io/user-node-remove_"
)

func Register(ctx context.Context, management *config.ManagementContext, clusterManager *clustermanager.Manager) {
	nodeClient := management.Management.Nodes("")

	nodeLifecycle := &Lifecycle{
		ctx:                  ctx,
		systemAccountManager: systemaccount.NewManager(management),
		nodeClient:           nodeClient,
		systemTokens:         management.SystemTokens,
		clusterManager:       clusterManager,
		devMode:              os.Getenv("CATTLE_DEV_MODE") != "",
	}

	nodeClient.AddHandler(ctx, "node-controller-sync", nodeLifecycle.sync)
}

type Lifecycle struct {
	ctx                  context.Context
	systemAccountManager *systemaccount.Manager
	nodeClient           v3.NodeInterface
	systemTokens         systemtokens.Interface
	clusterManager       *clustermanager.Manager
	devMode              bool
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
