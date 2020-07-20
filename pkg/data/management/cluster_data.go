package management

import (
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func addLocalCluster(embedded bool, adminName string, management *config.ManagementContext) error {
	c := &v3.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name: "local",
			Annotations: map[string]string{
				"field.cattle.io/creatorId": adminName,
			},
		},
		Spec: v32.ClusterSpec{
			Internal:    true,
			DisplayName: "local",
			ClusterSpecBase: v32.ClusterSpecBase{
				DockerRootDir: settings.InitialDockerRootDir.Get(),
			},
		},
		Status: v32.ClusterStatus{
			Driver: v32.ClusterDriverImported,
		},
	}
	if embedded {
		c.Status.Driver = v32.ClusterDriverLocal
	}

	// Ignore error
	management.Management.Clusters("").Create(c)
	return nil
}

func removeLocalCluster(management *config.ManagementContext) error {
	management.Management.Clusters("").Delete("local", &v1.DeleteOptions{})
	return nil
}
