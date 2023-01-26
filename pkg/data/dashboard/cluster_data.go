package dashboard

import (
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	fleetconst "github.com/rancher/rancher/pkg/fleet"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func addLocalCluster(embedded bool, wrangler *wrangler.Context) error {
	c := &v3.Cluster{
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
		Status: v32.ClusterStatus{
			Driver: v32.ClusterDriverImported,
			Conditions: []v32.ClusterCondition{
				{
					Type:   "Ready",
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	if embedded {
		c.Status.Driver = v32.ClusterDriverLocal
	}

	var err error
	err = wait.PollImmediateInfinite(100*time.Millisecond, func() (bool, error) {
		temporaryCluster, err := wrangler.Mgmt.Cluster().Create(c)
		if err == nil {
			c = temporaryCluster
			return true, nil
		} else if apierrors.IsAlreadyExists(err) {
			temporaryCluster, err = wrangler.Mgmt.Cluster().Get("local", v1.GetOptions{})
			if err == nil {
				c = temporaryCluster
				return true, nil
			}
		}
		if apierrors.IsServiceUnavailable(err) {
			return false, nil
		}
		return false, err
	})
	if err != nil {
		return err
	}

	_, err = wrangler.Core.Namespace().Create(&corev1.Namespace{
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

func removeLocalCluster(wrangler *wrangler.Context) error {
	// Ignore error
	_ = wrangler.Mgmt.Cluster().Delete("local", &v1.DeleteOptions{})
	return nil
}
