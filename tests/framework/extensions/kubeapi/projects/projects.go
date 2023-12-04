package projects

import (
	"github.com/rancher/rancher/tests/framework/extensions/constants"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	GroupName    = "management.cattle.io"
	Version      = "v3"
	localCluster = constants.LocalCluster
)

// ProjectGroupVersionResource is the required Group Version Resource for accessing projects in a cluster, using the dynamic client.
var ProjectGroupVersionResource = schema.GroupVersionResource{
	Group:    GroupName,
	Version:  Version,
	Resource: constants.Projects,
}
