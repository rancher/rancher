package etcdbackup

import (
	"context"
	"fmt"

	"github.com/rancher/kontainer-engine/drivers/rke"
	"github.com/rancher/kontainer-engine/types"
	"github.com/rancher/rancher/pkg/rkedialerfactory"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type Controller struct {
	ctx                context.Context
	clusterClient      v3.ClusterInterface
	backupClient       v3.EtcdBackupInterface
	backupConfigClient v3.EtcdBackupConfigInterface
	RKEDriver          types.Driver
}

func Register(ctx context.Context, management *config.ManagementContext) {

	c := &Controller{
		ctx:                ctx,
		clusterClient:      management.Management.Clusters(""),
		backupClient:       management.Management.EtcdBackups(""),
		backupConfigClient: management.Management.EtcdBackupConfigs(""),
		RKEDriver:          rke.NewDriver(),
	}
	local := &rkedialerfactory.RKEDialerFactory{
		Factory: management.Dialer,
	}
	docker := &rkedialerfactory.RKEDialerFactory{
		Factory: management.Dialer,
		Docker:  true,
	}

	driver := c.RKEDriver
	rkeDriver := driver.(*rke.Driver)
	rkeDriver.DockerDialer = docker.Build
	rkeDriver.LocalDialer = local.Build
	rkeDriver.WrapTransportFactory = docker.WrapTransport

	c.backupClient.AddHandler(ctx, "backup-handler", c.sync)
}

func (c *Controller) sync(key string, b *v3.EtcdBackup) (runtime.Object, error) {
	clusterName, _ := c.getOwnerClusterName(b)
	cluster, _ := c.clusterClient.Get(clusterName, metav1.GetOptions{})
	logrus.Infof("%v", cluster.Spec.RancherKubernetesEngineConfig)

	return nil, nil
}

func (c *Controller) getOwnerClusterName(b *v3.EtcdBackup) (string, error) {
	for _, ref := range b.GetOwnerReferences() {
		if ref.Kind == "Cluster" {
			return ref.Name, nil
		}
	}
	return "", fmt.Errorf("Can't find Cluster OwnerRerference for backup: %s", b.Name)
}
