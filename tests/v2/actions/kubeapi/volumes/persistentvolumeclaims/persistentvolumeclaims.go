package persistentvolumeclaims

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	PersistentVolumeClaimType            = "persistentvolumeclaim"
	PersistentVolumeBoundStatus          = "Bound"
	StevePersistentVolumeClaimVolumeName = "volumeName"

	AccessModeReadWriteOnce = "ReadWriteOnce"
	AccessModeReadWriteMany = "ReadWriteMany"
	AccessModeReadOnlyMany  = "ReadOnlyMany"
)

// PersistentVolumeClaimGroupVersionResource is the required Group Version Resource for accessing persistent
// volume claims in a cluster, using the dynamic client.
var PersistentVolumeClaimGroupVersionResource = schema.GroupVersionResource{
	Group:    "",
	Version:  "v1",
	Resource: "persistentvolumeclaims",
}
