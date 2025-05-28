package managedcluster

import (
	"context"
	"fmt"
	"sync"
	"time"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	controllerv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	initMdGathererOnce sync.Once
	mdGatherer         *mgmtClusterMdGatherer
)

const (
	CallbackMetricID = "prom-metrics-managed-cluster"
	CallbackSccID    = "scc-managed-cluster"
)

func NewMetadataGatherer(
	clusterCache controllerv3.ClusterCache,
	opts GatherOpts,
) {
	logrus.Debug("Attempting to initialize metatadata gatherer for managed clusters...")
	count := 0
	initMdGathererOnce.Do(func() {
		mdGatherer = newMetadataGatherer(clusterCache, GatherOpts{
			CollectInterval: opts.CollectInterval,
			Ctx:             opts.Ctx,
		})
		logrus.Info("Metadata gatherer for managed clusters initialized")
		count += 1
		go mdGatherer.Run()
	})

	if count == 0 {
		logrus.Warn("Metadata gatherer for managed clusters already initialized")
	}

}

func RegisterCallback(
	name string,
	cb func([]*apiv3.Cluster),
) error {
	if mdGatherer == nil {
		logrus.Errorf("Metadata gatherer is not initialized, cannot register callback %s", name)
		return fmt.Errorf("metadata gatherer not initialized")
	}
	mdGatherer.registerCallback(name, cb)
	return nil
}

type mgmtClusterMdGatherer struct {
	clusterCache controllerv3.ClusterCache

	cbMu *sync.Mutex
	cbs  map[string]func([]*apiv3.Cluster)

	opts GatherOpts
}

type GatherOpts struct {
	CollectInterval time.Duration
	Ctx             context.Context
}

func newMetadataGatherer(clusterCache controllerv3.ClusterCache, opts GatherOpts) *mgmtClusterMdGatherer {
	mg := &mgmtClusterMdGatherer{
		clusterCache: clusterCache,
		cbs:          make(map[string]func([]*apiv3.Cluster)),
		cbMu:         &sync.Mutex{},
		opts:         opts,
	}
	return mg
}

func (mg *mgmtClusterMdGatherer) registerCallback(name string, cb func([]*apiv3.Cluster)) {
	logrus.Debugf("Registering callback %s for managed cluster metadata gatherer", name)
	mg.cbMu.Lock()
	defer mg.cbMu.Unlock()
	mg.cbs[name] = cb
}

func (mg *mgmtClusterMdGatherer) Run() {
	logrus.Info("Starting managed cluster metadata gatherer...")
	tDur := mg.opts.CollectInterval
	if mg.opts.CollectInterval <= 0 {
		tDur = 60 * time.Second
	}
	t := time.NewTicker(tDur)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			clusters, err := mg.clusterCache.List(labels.Everything())
			if err != nil {
				continue
			}
			mg.cbMu.Lock()
			for name, cb := range mg.cbs {
				logrus.Debugf("Calling callback %s for managed cluster metadata gatherer", name)
				cb(clusters)
			}
			mg.cbMu.Unlock()
		case <-mg.opts.Ctx.Done():
			logrus.Infof("Stopping metadata gatherer for managed clusters...")
			return
		}
	}
}
