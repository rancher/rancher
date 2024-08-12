package persistentvolumes

import (
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// PersistentVolumesGroupVersionResource is the required Group Version Resource for accessing persistent volumes in a cluster,
// using the dynamic client.
var PersistentVolumesGroupVersionResource = schema.GroupVersionResource{
	Group:    "",
	Version:  "v1",
	Resource: "persistentvolumes",
}

// NewPersistentVolume is a constructor for a *PersistentVolume object
// It registers a delete fuction. `nodeSelectorRequirement`, `mountOptions`, `storageClass` are optional parameters if those are not needed pass nil for them will suffice
func NewPersistentVolume(volumeName, description string, accessModes []corev1.PersistentVolumeAccessMode, nodeSelectorRequirement []corev1.NodeSelectorRequirement, mountOptions []string, storageClass *storagev1.StorageClass) *corev1.PersistentVolume {
	annotations := map[string]string{
		"field.cattle.io/description": description,
	}

	persistentVolume := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        volumeName,
			Annotations: annotations,
		},
		Spec: corev1.PersistentVolumeSpec{
			AccessModes:  accessModes,
			MountOptions: mountOptions,
			NodeAffinity: &corev1.VolumeNodeAffinity{
				Required: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: nodeSelectorRequirement,
						},
					},
				},
			},
		},
	}
	if storageClass != nil {
		persistentVolume.Spec.StorageClassName = storageClass.Name
	}

	return persistentVolume
}
