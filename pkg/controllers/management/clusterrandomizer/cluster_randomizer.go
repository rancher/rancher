package clusterrandomizer

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	featureflags "github.com/rancher/rancher/pkg/features"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

type Controller struct {
	ctx           context.Context
	clusterClient v3.ClusterInterface
	clusterLister v3.ClusterLister
}

func Register(ctx context.Context, management *config.ManagementContext) {
	c := &Controller{
		ctx:           ctx,
		clusterClient: management.Management.Clusters(""),
		clusterLister: management.Management.Clusters("").Controller().Lister(),
	}
	m := management.Management.ClusterRandomizers("")
	m.AddFeatureHandler(featureflags.IsFeatEnabled, "cluster-randomizer", ctx, "cluster-randomizer-controller", c.sync)
}

func (c *Controller) sync(key string, exampleConfig *v3.ClusterRandomizer) (runtime.Object, error) {
	fmt.Println("TEST IN")
	rand.Seed(time.Now().UTC().UnixNano())
	clusters, _ := c.clusterLister.List("", labels.Everything())
	for _, cluster := range clusters {
		cluster.Spec.DisplayName = fmt.Sprintf("cluster%v", rand.Intn(1000))
		_, err := c.clusterClient.Update(cluster)
		if err == nil {
			fmt.Println("TEST UPDATED SUCCESS")
		}
	}

	return nil, nil
}