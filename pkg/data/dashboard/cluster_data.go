package dashboard

import (
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/features"
	fleetconst "github.com/rancher/rancher/pkg/fleet"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func addLocalCluster(embedded bool, clusters mgmtcontrollers.ClusterClient, namespaces corecontrollers.NamespaceClient) error {
	c := &v32.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name: "local",
		},
		Spec: v32.ClusterSpec{
			Internal:           true,
			DisplayName:        "local",
			FleetWorkspaceName: fleetconst.ClustersLocalNamespace,
			ClusterSpecBase: v32.ClusterSpecBase{
				DockerRootDir: settings.InitialDockerRootDir.Get(),
			},
		},
	}

	desiredDriver := v32.ClusterDriverImported
	if embedded {
		desiredDriver = v32.ClusterDriverLocal
	}

	var err error
	err = wait.PollImmediateInfinite(100*time.Millisecond, func() (bool, error) {
		temporaryCluster, err := clusters.Create(c)
		if err == nil {
			c = temporaryCluster
			return true, nil
		} else if apierrors.IsAlreadyExists(err) {
			temporaryCluster, err = clusters.Get("local", v1.GetOptions{})
			if err == nil {
				c = temporaryCluster
				return true, nil
			}
		}
		if apierrors.IsServiceUnavailable(err) || apierrors.IsInternalError(err) {
			return false, nil
		}
		return false, err
	})
	if err != nil {
		return err
	}

	if !hasReadyCondition(c.Status.Conditions) {
		c.Status.Driver = desiredDriver
		c.Status.Conditions = []v32.ClusterCondition{
			{
				Type:   "Ready",
				Status: corev1.ConditionTrue,
			},
		}
		c, err = clusters.UpdateStatus(c)
		if err != nil {
			return err
		}
	}

	// The "local" namespace is the cluster namespace of the local cluster and holds its
	// namespaced management.cattle.io resources. A downstream cluster (MCMAgent enabled)
	// does not need this namespace.
	if features.MCMAgent.Enabled() {
		return nil
	}

	_, err = namespaces.Create(&corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: "local",
			OwnerReferences: []v1.OwnerReference{
				{
					APIVersion: v32.SchemeGroupVersion.String(),
					Kind:       c.Kind,
					Name:       c.Name,
					UID:        c.UID,
				},
			},
		},
	})
	if apierrors.IsAlreadyExists(err) {
		return nil
	}

	return err
}

func hasReadyCondition(conditions []v32.ClusterCondition) bool {
	for _, c := range conditions {
		if c.Type == "Ready" {
			return true
		}
	}
	return false
}

func removeLocalCluster(clusters mgmtcontrollers.ClusterClient) error {
	// Ignore error
	_ = clusters.Delete("local", &v1.DeleteOptions{})
	return nil
}
