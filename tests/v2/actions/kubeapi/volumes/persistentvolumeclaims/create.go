package persistentvolumeclaims

import (
	"context"
	"strconv"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/unstructured"
	"github.com/rancher/shepherd/pkg/api/scheme"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1Unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// CreatePersistentVolumeClaim is a helper function that uses the dynamic client to create a persistent
// volume claim on a namespace for a specific cluster.
// If you pass a PersistentVolume then `storageClass` and `storage` would be optional, otherwise `persistentVolume`
// would be optional and `storage` and` storageClass` are needed.
// The function registers a delete fuction.
func CreatePersistentVolumeClaim(client *rancher.Client, clusterName, persistentVolumeClaimName, description, namespace string, storage int, accessModes []corev1.PersistentVolumeAccessMode, persistentVolume *corev1.PersistentVolume, storageClass *storagev1.StorageClass) (*corev1.PersistentVolumeClaim, error) {
	var unstructuredVolumeClaim *metav1Unstructured.Unstructured

	annotations := map[string]string{
		"field.cattle.io/description": description,
	}

	persistentVolumeClaim := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        persistentVolumeClaimName,
			Annotations: annotations,
			Namespace:   namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: accessModes,
		},
	}

	if persistentVolume != nil {
		storageClassName := ""
		persistentVolumeClaim.Spec.Resources = corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: *persistentVolume.Spec.Capacity.Storage(),
			},
		}
		persistentVolumeClaim.Spec.StorageClassName = &storageClassName
		persistentVolumeClaim.Spec.VolumeName = persistentVolume.Name
		unstructuredVolumeClaim = unstructured.MustToUnstructured(persistentVolumeClaim)
	} else if storageClass != nil {
		persistentVolumeClaim.Spec.VolumeName = ""
		persistentVolumeClaim.Spec.StorageClassName = &storageClass.Name

		stringStorage := strconv.Itoa(storage) + "Gi"
		unstructuredVolumeClaim = unstructured.MustToUnstructured(persistentVolumeClaim)
		specMap := unstructuredVolumeClaim.Object["spec"].(map[string]interface{})
		specMap["resources"] = map[string]interface{}{
			"requests": map[string]interface{}{
				"storage": stringStorage,
			},
		}
	}

	dynamicClient, err := client.GetDownStreamClusterClient(clusterName)
	if err != nil {
		return nil, err
	}

	PersistentVolumeClaimResource := dynamicClient.Resource(PersistentVolumeClaimGroupVersionResource).Namespace(namespace)

	unstructuredResp, err := PersistentVolumeClaimResource.Create(context.TODO(), unstructuredVolumeClaim, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	newPersistentVolumeClaim := &corev1.PersistentVolumeClaim{}

	err = scheme.Scheme.Convert(unstructuredResp, newPersistentVolumeClaim, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	return newPersistentVolumeClaim, nil
}
