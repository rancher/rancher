package app

import (
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

func addClusters(addLocal bool, adminName string, management *config.ManagementContext) error {
	if addLocal {
		// Ignore error
		management.Management.Clusters("").Create(&v3.Cluster{
			ObjectMeta: v1.ObjectMeta{
				Name: "local",
				Annotations: map[string]string{
					"field.cattle.io/creatorId": adminName,
				},
			},
			Spec: v3.ClusterSpec{
				DisplayName: "local",
			},
			Status: v3.ClusterStatus{
				Driver: "local",
			},
		})
	}
	return nil
}
