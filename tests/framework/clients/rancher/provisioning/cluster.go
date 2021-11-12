package provisioning

import (
	"context"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Cluster) Create(ctx context.Context, cluster *apisV1.Cluster, opts metav1.CreateOptions) (*apisV1.Cluster, error) {
	c.ts.RegisterCleanupFunc(func() error {
		err := c.Delete(context.TODO(), cluster.GetName(), metav1.DeleteOptions{})
		if errors.IsNotFound(err) {
			return nil
		}

		return err
	})
	return c.ClusterInterface.Create(ctx, cluster, opts)
}
