package clusterlocal

import (
	"fmt"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/event"
	"github.com/rancher/rancher/pkg/clusterprovisioninglogger"
	"github.com/rancher/rancher/pkg/clusteryaml"
	"github.com/rancher/rancher/pkg/rkecerts"
	"github.com/rancher/rancher/pkg/rkedialerfactory"
	"github.com/rancher/rke/cluster"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

func Register(management *config.ManagementContext) {
	cl := &clusterLocal{
		clusterController: management.Management.Clusters("").Controller(),
		clusters:          management.Management.Clusters(""),
		eventLogger:       management.EventLogger,
		nodeLister:        management.Management.Nodes("").Controller().Lister(),
		rkeDialer: rkedialerfactory.RKEDialerFactory{
			Factory: management.Dialer,
		},
	}

	management.Management.Clusters("").AddHandler("cluster-local-provisioner", cl.sync)
	management.Management.Nodes("").AddHandler("cluster-local-provisioner", cl.nodeChanged)
}

type clusterLocal struct {
	clusterController v3.ClusterController
	clusters          v3.ClusterInterface
	eventLogger       event.Logger
	nodeLister        v3.NodeLister
	rkeDialer         rkedialerfactory.RKEDialerFactory
}

func (cl *clusterLocal) nodeChanged(key string, node *v3.Node) error {
	if node == nil {
		return nil
	}

	cl.clusterController.Enqueue("", node.Namespace)
	return nil
}

func (cl *clusterLocal) sync(key string, cluster *v3.Cluster) error {
	if cluster == nil {
		return nil
	}

	if cluster.Status.Driver != v3.ClusterDriverLocal {
		return nil
	}

	changed := false
	newObj, err := v3.ClusterConditionAddonDeploy.Once(cluster, func() (runtime.Object, error) {
		changed = true
		if err := cl.deploy(cluster); err != nil {
			return nil, err
		}

		// Need to reload because logger will have changed the cluster object
		return cl.clusters.Get(cluster.Name, v1.GetOptions{})
	})

	if changed && err == nil {
		_, err = cl.clusters.Update(newObj.(*v3.Cluster))
	}

	return err
}

func (cl *clusterLocal) checkNodes(c *v3.Cluster) error {
	nodes, err := cl.nodeLister.List(c.Name, labels.Everything())
	if err != nil {
		return err
	}

	for _, node := range nodes {
		if v3.NodeConditionRegistered.IsTrue(node) {
			return nil
		}
	}

	return &controller.ForgetError{
		Err: fmt.Errorf("waiting for nodes to join cluster: %s", c.Name),
	}
}

func (cl *clusterLocal) deploy(c *v3.Cluster) error {
	ctx, logger := clusterprovisioninglogger.NewNonRPCLogger(cl.clusters, cl.eventLogger, c, v3.ClusterConditionAddonDeploy)
	defer logger.Close()

	if err := cl.checkNodes(c); err != nil {
		return err
	}

	rkeConfig, err := clusteryaml.LocalConfig()
	if err != nil {
		return err
	}

	wt := cl.rkeDialer.WrapTransport(rkeConfig)

	rkeCluster, err := cluster.ParseCluster(ctx,
		rkeConfig,
		"",
		"",
		nil,
		nil,
		wt)
	if err != nil {
		return err
	}

	rkeCluster.UseKubectlDeploy = true

	if err := cluster.ApplyAuthzResources(ctx, *rkeConfig, "", "", wt); err != nil {
		return err
	}

	bundle, err := rkecerts.Load()
	if err != nil {
		return err
	}

	return cluster.ConfigureCluster(ctx,
		*rkeConfig,
		bundle.Certs(),
		"",
		"",
		cl.rkeDialer.WrapTransport(rkeConfig),
		true)
}
