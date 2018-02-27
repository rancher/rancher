package app

import (
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

func addClusters(addLocal, embedded bool, adminName string, management *config.ManagementContext) error {
	if addLocal {
		// Ignore error
		c := &v3.Cluster{
			ObjectMeta: v1.ObjectMeta{
				Name: "local",
				Annotations: map[string]string{
					"field.cattle.io/creatorId": adminName,
				},
			},
			Spec: v3.ClusterSpec{
				Internal:    true,
				DisplayName: "local",
			},
			Status: v3.ClusterStatus{
				Driver: v3.ClusterDriverImported,
			},
		}
		if embedded {
			c.Status.Driver = v3.ClusterDriverLocal
		}

		management.Management.Clusters("").Create(c)
	}

	return nil
}
