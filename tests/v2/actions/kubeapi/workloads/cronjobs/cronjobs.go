package cronjobs

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// CronJobGroupVersionResource is the required Group Version Resource for accessing cron jobs in a cluster,
// using the dynamic client.
var CronJobGroupVersionResource = schema.GroupVersionResource{
	Group:    "batch",
	Version:  "v1beta1",
	Resource: "cronjobs",
}
