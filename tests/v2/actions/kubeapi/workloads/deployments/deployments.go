package deployments

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// DeploymentGroupVersionResource is the required Group Version Resource for accessing deployments in a cluster,
// using the dynamic client.
var DeploymentGroupVersionResource = schema.GroupVersionResource{
	Group:    "apps",
	Version:  "v1",
	Resource: "deployments",
}
