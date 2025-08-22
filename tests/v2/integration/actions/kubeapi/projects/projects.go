package projects

import (
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	Admin               = "admin"
	StandardUser        = "user"
	DefaultNamespace    = "fleet-default"
	RancherNamespace    = "cattle-system"
	LocalCluster        = "local"
	Projects            = "projects"
	ProjectIDAnnotation = "field.cattle.io/projectId"
	GroupName           = "management.cattle.io"
	Version             = "v3"
)

// ProjectGroupVersionResource is the required Group Version Resource for accessing projects in a cluster, using the dynamic client.
var ProjectGroupVersionResource = schema.GroupVersionResource{
	Group:    GroupName,
	Version:  Version,
	Resource: Projects,
}

// NewProjectTemplate is a constructor that creates the project template
func NewProjectTemplate(clusterID string) *v3.Project {
	project := &v3.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:       namegen.AppendRandomString("testproject"),
			Namespace:  clusterID,
			Finalizers: []string{},
		},
		Spec: v3.ProjectSpec{
			ClusterName: clusterID,
			ResourceQuota: &v3.ProjectResourceQuota{
				Limit: v3.ResourceQuotaLimit{
					Pods: "",
				},
			},
			NamespaceDefaultResourceQuota: &v3.NamespaceResourceQuota{
				Limit: v3.ResourceQuotaLimit{
					Pods: "",
				},
			},
			ContainerDefaultResourceLimit: &v3.ContainerResourceLimit{
				RequestsCPU:    "",
				RequestsMemory: "",
				LimitsCPU:      "",
				LimitsMemory:   "",
			},
		},
	}
	return project
}
