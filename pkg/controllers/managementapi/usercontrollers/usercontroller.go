package usercontrollers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	v33 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
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
		ctx:           ctx,
		ownerStrategy: getOwnerStrategy(ctx, scaledContext.PeerManager),
	}

	scaledContext.Management.Clusters("").AddHandler(ctx, "user-controllers-controller", u.sync)

	go func() {
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			case <-u.forcedResync():
				timer.Stop()
			}
			if err := u.reconcileClusterOwnership(); err != nil {
				// faster retry on error
				logrus.WithError(err).Errorf("Failed syncing peers")
				timer.Reset(5 * time.Second)
			} else {
				timer.Reset(2 * time.Minute)
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
	ownerStrategy
	starter       controllerStarter
	clusterLister v3.ClusterLister
	clusters      v3.ClusterInterface
	ctx           context.Context
}

func (u *userControllersController) sync(key string, cluster *v3.Cluster) (runtime.Object, error) {
	if cluster != nil && cluster.DeletionTimestamp != nil {
		err := u.cleanFinalizers(key, cluster)
		if err != nil {
			return nil, fmt.Errorf("userControllersController: failed to clean finalizers for cluster %s: %w", cluster.Name, err)
		}
	}
	if key == relatedresource.AllKey {
		if err := u.reconcileClusterOwnership(); err != nil {
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
		err = u.starter.Start(u.ctx, cluster, u.isOwner(cluster))
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

func (u *userControllersController) reconcileClusterOwnership() error {
	clusters, err := u.clusterLister.List("", labels.Everything())
	if err != nil {
		return err
	}

	u.Lock()
	defer u.Unlock()

	var (
		errs []error
	)

	for _, cluster := range clusters {
		if cluster.DeletionTimestamp != nil || !v33.ClusterConditionProvisioned.IsTrue(cluster) {
			u.starter.Stop(cluster)
		} else {
			amOwner := u.isOwner(cluster)
			if err := u.starter.Start(u.ctx, cluster, amOwner); err != nil {
				errs = append(errs, errors.Wrapf(err, "failed to start user controllers for cluster %s", cluster.Name))
			}
		}
	}

	return types.NewErrors(errs...)
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
