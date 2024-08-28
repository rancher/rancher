package jobs

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// JobGroupVersionResource is the required Group Version Resource for accessing jobs in a cluster,
// using the dynamic client.
var JobGroupVersionResource = schema.GroupVersionResource{
	Group:    "batch",
	Version:  "v1",
	Resource: "jobs",
}
