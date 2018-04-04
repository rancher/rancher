package app

import (
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

func addLocalCluster(embedded bool, adminName string, management *config.ManagementContext) error {
	c := &v3.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name: "local",
			Annotations: map[string]string{
				"field.cattle.io/creatorId": adminName,
			},
		},
		Spec: v3.ClusterSpec{
			Internal: true,
		},
		Status: v3.ClusterStatus{
			Driver: v3.ClusterDriverImported,
		},
	}
	if embedded {
		c.Status.Driver = v3.ClusterDriverLocal
	}

	// Ignore error
	management.Management.Clusters("").Create(c)
	return nil
}
