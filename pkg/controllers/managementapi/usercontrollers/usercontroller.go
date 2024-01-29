package usercontrollers

import (
	"context"
	"fmt"
	"hash/crc32"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	v33 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/metrics"
	tpeermanager "github.com/rancher/rancher/pkg/peermanager"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v2/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/version"
)

// currentClusterControllersVersion is the version of the controllers that are run in the local cluster for a particular downstream cluster.
const currentClusterControllersVersion = "management.cattle.io/current-cluster-controllers-version"

// Register adds the user-controllers-controller handler and starts the peer manager.
func Register(ctx context.Context, scaledContext *config.ScaledContext, clusterManager *clustermanager.Manager) {
	u := &userControllersController{
		starter:       clusterManager,
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

// controllerStarter starts and stops controllers for a given cluster.
type controllerStarter interface {
	Start(ctx context.Context, cluster *v3.Cluster, clusterOwner bool) error
	Stop(cluster *v3.Cluster)
}

type userControllersController struct {
	sync.Mutex
	clustered     bool
	starter       controllerStarter
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
			return nil, fmt.Errorf("userControllersController: failed to clean finalizers for cluster %s: %w", cluster.Name, err)
		}
	}
	if key == relatedresource.AllKey {
		err := u.setPeers(nil)
		if err != nil {
			return nil, fmt.Errorf("userControllersController: failed to set peers for key %s: %w", key, err)
		}
		return nil, nil
	}

	// Check if the cluster has been upgraded to a major or minor version and restart the controllers.
	if cluster != nil && cluster.Status.Version != nil {
		var err error
		currentVersion, err := getCurrentClusterControllersVersion(cluster)
		// If the version is not found on the annotation, set it and update the cluster object.
		var clusterCopy *v3.Cluster
		if err != nil {
			if clusterCopy, err = u.saveClusterControllersVersionAnnotation(cluster, cluster.Status.Version.String()); err != nil {
				return nil, fmt.Errorf("userControllersController: failed to save version annotation for cluster %s: %w", cluster.Name, err)
			}
			cluster = clusterCopy
			u.clusters.Controller().Enqueue("", relatedresource.AllKey)
			return cluster, nil
		}

		newVersion, err := version.ParseSemantic(cluster.Status.Version.String())
		if err != nil {
			logrus.Errorf("failed to parse the K8s version of the upgraded cluster %s, will not restart cluster controllers: %v", cluster.Name, err)
			u.clusters.Controller().Enqueue("", relatedresource.AllKey)
			return cluster, nil
		}

		if !clusterVersionChanged(currentVersion, newVersion) {
			u.clusters.Controller().Enqueue("", relatedresource.AllKey)
			return cluster, nil
		}

		u.starter.Stop(cluster)
		err = u.starter.Start(u.ctx, cluster, u.amOwner(u.peers, cluster))
		if err != nil {
			return nil, fmt.Errorf("userControllersController: unable to restart controllers for cluster %s: %w", cluster.Name, err)
		}
		if clusterCopy, err = u.saveClusterControllersVersionAnnotation(cluster, newVersion.String()); err != nil {
			return nil, fmt.Errorf("userControllersController: failed to save new version annotation for cluster %s: %w", cluster.Name, err)
		}
		cluster = clusterCopy
	}

	u.clusters.Controller().Enqueue("", relatedresource.AllKey)
	return cluster, nil
}

// saveClusterControllersVersionAnnotation updates and persists a cluster object with a given value for the annotation
// that indicates the cluster controllers' version. It does not matter if the provided version starts with a "v" or doesn't,
// as it's parsed the same by other code.
func (u *userControllersController) saveClusterControllersVersionAnnotation(cluster *v3.Cluster, value string) (*v3.Cluster, error) {
	cluster = cluster.DeepCopy()
	cluster.Annotations[currentClusterControllersVersion] = value
	cluster, err := u.clusters.Update(cluster)
	if err != nil {
		return nil, fmt.Errorf("unable to update cluster %s with annotation indicating its K8s version: %w", cluster.Name, err)
	}
	return cluster, nil
}

func getCurrentClusterControllersVersion(cluster *v3.Cluster) (*version.Version, error) {
	v := cluster.Annotations[currentClusterControllersVersion]
	return version.ParseSemantic(v)
}

// clusterVersionChanged checks if the two given cluster versions differ in the major or minor levels.
func clusterVersionChanged(current, new *version.Version) bool {
	return current.Major() != new.Major() || current.Minor() != new.Minor()
}

func (u *userControllersController) setPeers(peers *tpeermanager.Peers) error {
	u.Lock()
	defer u.Unlock()

	if peers != nil {
		u.peers = *peers
		u.peers.IDs = append(u.peers.IDs, u.peers.SelfID)
		sort.Strings(u.peers.IDs)
	}

	return u.peersSync()
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
		if cluster.DeletionTimestamp != nil || !v33.ClusterConditionProvisioned.IsTrue(cluster) {
			u.starter.Stop(cluster)
		} else {
			amOwner := u.amOwner(u.peers, cluster)
			if amOwner {
				metrics.SetClusterOwner(u.peers.SelfID, cluster.Name)
			} else {
				metrics.UnsetClusterOwner(u.peers.SelfID, cluster.Name)
			}
			if err := u.starter.Start(u.ctx, cluster, amOwner); err != nil {
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
