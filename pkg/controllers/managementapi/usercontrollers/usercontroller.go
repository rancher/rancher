package usercontrollers

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	v33 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/version"
)

// currentClusterControllersVersion is the version of the controllers that are run in the local cluster for a particular downstream cluster.
const currentClusterControllersVersion = "management.cattle.io/current-cluster-controllers-version"

// Register adds the user-controllers-controller handler and starts the peer manager.
func Register(ctx context.Context, scaledContext *config.ScaledContext, clusterManager *clustermanager.Manager) {
	u := &userControllersController{
		ctx:           ctx,
		starter:       clusterManager,
		clusterLister: scaledContext.Management.Clusters("").Controller().Lister(),
		clusters:      scaledContext.Management.Clusters(""),
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
	ownerStrategy
	ctx           context.Context
	starter       controllerStarter
	clusterLister v3.ClusterLister
	clusters      v3.ClusterInterface
}

func (u *userControllersController) sync(key string, cluster *v3.Cluster) (runtime.Object, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return nil, nil
	}

	// Check if the cluster has been upgraded to a major or minor version and restart the controllers.
	newVersion, restartNeeded, err := u.checkClusterControllerVersion(cluster)
	if err != nil {
		return nil, err
	}

	// Skip usercontrollers if cluster is not yet provisioned
	if !v33.ClusterConditionProvisioned.IsTrue(cluster) {
		return cluster, nil
	}

	if restartNeeded {
		u.starter.Stop(cluster)
	}
	if err := u.starter.Start(u.ctx, cluster, u.isOwner(cluster)); err != nil {
		action := "starting"
		if restartNeeded {
			action = "restarting"
		}
		return nil, fmt.Errorf("userControllersController: unable to %s controllers for cluster %s: %w", action, key, err)
	}

	// Only update the version annotation after a successful start
	if newVersion != "" {
		if cluster, err = u.saveClusterControllersVersionAnnotation(cluster, newVersion); err != nil {
			return nil, fmt.Errorf("userControllersController: failed to save new version annotation for cluster %s: %w", key, err)
		}
	}

	return cluster, nil
}

func (u *userControllersController) checkClusterControllerVersion(cluster *v3.Cluster) (newVersion string, needsRestart bool, err error) {
	if cluster.Status.Version == nil {
		return "", false, nil
	}

	clusterName := cluster.Name
	clusterVersion := cluster.Status.Version.String()
	clusterSemver, err := version.ParseSemantic(clusterVersion)
	if err != nil {
		logrus.Errorf("failed to parse the K8s version of the upgraded cluster %s, will not restart cluster controllers: %v", clusterName, err)
		return "", false, nil
	}

	currentVersion, err := getCurrentClusterControllersVersion(cluster)
	if err != nil {
		// If the version is not found on the annotation, just set it after the start
		return clusterSemver.String(), false, nil
	}

	if !clusterVersionChanged(currentVersion, clusterSemver) {
		return "", false, nil
	}
	return clusterSemver.String(), true, nil
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

	var (
		errs []error
	)

	for _, cluster := range clusters {
		if cluster.DeletionTimestamp != nil || !v33.ClusterConditionProvisioned.IsTrue(cluster) {
			continue
		}

		if err := u.starter.Start(u.ctx, cluster, u.isOwner(cluster)); err != nil {
			errs = append(errs, errors.Wrapf(err, "failed to start user controllers for cluster %s", cluster.Name))
		}
	}

	return types.NewErrors(errs...)
}
