package fleet

import (
	"context"

	v1alpha1 "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Create is ClusterGroupInterface's Create function, that is being overwritten to register its delete function to the session.Session
// that is being reference.
func (c *ClusterGroup) Create(ctx context.Context, clusterGroup *v1alpha1.ClusterGroup, opts metav1.CreateOptions) (*v1alpha1.ClusterGroup, error) {
	c.ts.RegisterCleanupFunc(func() error {
		err := c.Delete(context.TODO(), clusterGroup.GetName(), metav1.DeleteOptions{})
		if errors.IsNotFound(err) {
			return nil
		}

		return err
	})
	return c.ClusterGroupInterface.Create(ctx, clusterGroup, opts)
}
