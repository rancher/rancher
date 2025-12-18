package usercontrollers

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/features"
	controllers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apimachinery/pkg/util/wait"
)

// currentClusterControllersVersion is the version of the controllers that are run in the local cluster for a particular downstream cluster.
const currentClusterControllersVersion = "management.cattle.io/current-cluster-controllers-version"

// Register adds the user-controllers-controller handler and starts the peer manager.
func Register(ctx context.Context, scaledContext *config.ScaledContext, clusterManager *clustermanager.Manager) {
	// Configure a handler for features that will cancel the owner strategy if the value is flipped.
	// This eliminates the need for using "Recreate" rollout strategy to avoid a split-brain situation
	// At the cost of temporarily stopping usercontrollers while waiting for new pods to be up and running
	clusterOwnershipFeature := features.ClusterOwnershipStrategy
	initialValue := clusterOwnershipFeature.Enabled()
	featureName := clusterOwnershipFeature.Name()
	ctx, cancel := context.WithCancel(ctx)
	scaledContext.Wrangler.Mgmt.Feature().OnChange(ctx, "cluster-owner-strategy-change", func(s string, obj *v3.Feature) (*v3.Feature, error) {
		if obj == nil || s != featureName {
			return nil, nil
		}
		if value := features.IsEnabled(obj); initialValue != value {
			logrus.Warnf("Feature flag %q was flipped, new value is %v, disabling userscontrollers in preparation for restart", featureName, value)
			cancel()
		}
		return nil, nil
	})

	u := &userControllersController{
		ctx:           ctx,
		starter:       clusterManager,
		clusterLister: scaledContext.Wrangler.Mgmt.Cluster().Cache(),
		clusters:      scaledContext.Wrangler.Mgmt.Cluster(),
		ownerStrategy: getOwnerStrategy(ctx, scaledContext.PeerManager, initialValue),
	}

	scaledContext.Wrangler.Mgmt.Cluster().OnChange(ctx, "user-controllers-controller", u.sync)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-u.forcedResync():
			}

			backoff := wait.Backoff{
				Steps:    3,
				Duration: 5 * time.Second,
				Factor:   1,
			}
			if err := wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
				if err := u.reconcileClusterOwnership(); err != nil {
					logrus.Warnf("Failed to reconcile cluster ownership: %v, retrying...", err)
					return false, nil
				}
				return true, nil
			}); err != nil {
				logrus.Errorf("Giving up reconciling cluster ownership after %d attempts", backoff.Steps)
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
	clusterLister controllers.ClusterCache
	clusters      controllers.ClusterClient
}

func (u *userControllersController) sync(key string, cluster *v3.Cluster) (*v3.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return nil, nil
	}

	// Check if the cluster has been upgraded to a major or minor version and restart the controllers.
	newVersion, restartNeeded, err := u.checkClusterControllerVersion(cluster)
	if err != nil {
		return nil, err
	}

	// Skip usercontrollers if cluster is not yet provisioned
	if !v3.ClusterConditionProvisioned.IsTrue(cluster) {
		return cluster, nil
	}

	if restartNeeded {
		u.starter.Stop(cluster)
	}
	if err := u.starter.Start(u.ctx, cluster, u.isOwner(cluster)); err != nil {
		action := "start"
		if restartNeeded {
			action = "restart"
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
	clusterName := cluster.Name
	cluster = cluster.DeepCopy()
	cluster.Annotations[currentClusterControllersVersion] = value
	cluster, err := u.clusters.Update(cluster)
	if err != nil {
		return nil, fmt.Errorf("unable to update cluster %s with annotation indicating its K8s version: %w", clusterName, err)
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
	clusters, err := u.clusterLister.List(labels.Everything())
	if err != nil {
		return err
	}

	var (
		errs []error
	)

	for _, cluster := range clusters {
		if cluster.DeletionTimestamp != nil || !v3.ClusterConditionProvisioned.IsTrue(cluster) {
			continue
		}

		if err := u.starter.Start(u.ctx, cluster, u.isOwner(cluster)); err != nil {
			errs = append(errs, errors.Wrapf(err, "failed to start user controllers for cluster %s", cluster.Name))
		}
	}

	return types.NewErrors(errs...)
}
