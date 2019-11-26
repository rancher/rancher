package usercontrollers

import (
	"context"
	"hash/crc32"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/metrics"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	tpeermanager "github.com/rancher/types/peermanager"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	all = "_all_"
)

func Register(ctx context.Context, scaledContext *config.ScaledContext, clusterManager *clustermanager.Manager) {
	u := &userControllersController{
		manager:       clusterManager,
		clusterLister: scaledContext.Management.Clusters("").Controller().Lister(),
		clusters:      scaledContext.Management.Clusters(""),
		clustered:     scaledContext.PeerManager != nil,
		ctx:           ctx,
		start:         time.Now(),
	}

	scaledContext.Management.Clusters("").AddHandler(ctx, "user-controllers-controller", u.sync)

	if scaledContext.PeerManager != nil {
		c := make(chan tpeermanager.Peers, 100)
		scaledContext.PeerManager.AddListener(c)

		go func() {
			for peer := range c {
				if err := u.setPeers(&peer); err != nil {
					logrus.Errorf("Failed syncing peers [%v]: %v", peer, err)
				}
			}
		}()

		go func() {
			<-ctx.Done()
			scaledContext.PeerManager.RemoveListener(c)
			close(c)
		}()
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				if err := u.setPeers(nil); err == nil {
					time.Sleep(2 * time.Minute)
				}
			}
		}
	}()
}

type userControllersController struct {
	sync.Mutex
	clustered     bool
	manager       *clustermanager.Manager
	clusterLister v3.ClusterLister
	clusters      v3.ClusterInterface
	ctx           context.Context
	peers         tpeermanager.Peers
	start         time.Time
}

func (u *userControllersController) sync(key string, cluster *v3.Cluster) (runtime.Object, error) {
	if cluster != nil && cluster.DeletionTimestamp != nil {
		err := u.cleanFinalizers(key, cluster)
		if err != nil {
			return nil, err
		}
	}
	if key == all {
		return nil, u.setPeers(nil)
	}
	u.clusters.Controller().Enqueue("", all)
	return cluster, nil
}

func (u *userControllersController) setPeers(peers *tpeermanager.Peers) error {
	u.Lock()
	defer u.Unlock()

	if peers != nil {
		u.peers = *peers
		u.peers.IDs = append(u.peers.IDs, u.peers.SelfID)
		sort.Strings(u.peers.IDs)
	}

	if err := u.peersSync(); err != nil {
		return err
	}

	return nil
}

func (u *userControllersController) peersSync() error {
	clusters, err := u.clusterLister.List("", labels.Everything())
	if err != nil {
		return err
	}

	var (
		errs []error
	)

	for _, cluster := range clusters {
		if cluster.DeletionTimestamp != nil || !v3.ClusterConditionProvisioned.IsTrue(cluster) {
			u.manager.Stop(cluster)
		} else {
			amOwner := u.amOwner(u.peers, cluster)
			if amOwner {
				metrics.SetClusterOwner(u.peers.SelfID, cluster.Name)
			} else {
				metrics.UnsetClusterOwner(u.peers.SelfID, cluster.Name)
			}
			if err := u.manager.Start(u.ctx, cluster, amOwner); err != nil {
				errs = append(errs, errors.Wrapf(err, "failed to start user controllers for cluster %s", cluster.Name))
			}
		}
	}

	return types.NewErrors(errs...)
}

func (u *userControllersController) amOwner(peers tpeermanager.Peers, cluster *v3.Cluster) bool {
	if !u.clustered {
		return true
	}

	if !peers.Ready || len(peers.IDs) == 0 || (len(peers.IDs) == 1 && !peers.Leader) {
		return false
	}

	ck := crc32.ChecksumIEEE([]byte(cluster.UID))
	if ck == math.MaxUint32 {
		ck--
	}

	scaled := int(ck) * len(peers.IDs) / math.MaxUint32
	logrus.Debugf("%s(%v): (%v * %v) / %v = %v[%v] = %v, self = %v\n", cluster.Name, cluster.UID, ck,
		uint32(len(peers.IDs)), math.MaxUint32, peers.IDs, scaled, peers.IDs[scaled], peers.SelfID)
	return peers.IDs[scaled] == peers.SelfID
}

func (u *userControllersController) cleanFinalizers(key string, cluster *v3.Cluster) error {
	c, err := u.clusters.Get(key, metav1.GetOptions{})
	if err != nil {
		return err
	}

	var updated bool
	var newFinalizers []string

	for _, finalizer := range c.ObjectMeta.Finalizers {
		if finalizer == "controller.cattle.io/cluster-agent-controller" {
			updated = true
			continue
		}
		newFinalizers = append(newFinalizers, finalizer)
	}

	if updated {
		c.ObjectMeta.Finalizers = newFinalizers
		_, err = u.clusters.Update(c)
		return err
	}

	return nil
}
