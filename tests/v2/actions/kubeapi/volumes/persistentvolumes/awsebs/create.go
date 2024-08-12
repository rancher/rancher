package awsebs

import (
	"context"
	"strconv"

	"github.com/rancher/rancher/tests/v2/actions/kubeapi/volumes/persistentvolumes"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/unstructured"
	"github.com/rancher/shepherd/pkg/api/scheme"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateAWSEBSPersistentVolume is a helper function that uses the dynamic client to create an aws ebs persistent volume for a specific cluster.
// It registers a delete fuction.
func CreateAWSEBSPersistentVolume(client *rancher.Client, clusterName, fsType, volumeID string, storage int, partition int32, readOnly bool, accessModes []corev1.PersistentVolumeAccessMode, persistentVolume *corev1.PersistentVolume) (*corev1.PersistentVolume, error) {
	stringStorage := strconv.Itoa(storage) + "Gi"
	unstructuredPersistentVolume := unstructured.MustToUnstructured(persistentVolume)

	specMap := unstructuredPersistentVolume.Object["spec"].(map[string]interface{})
	specMap["awsElasticBlockStore"] = corev1.AWSElasticBlockStoreVolumeSource{
		FSType:    fsType,
		Partition: partition,
		ReadOnly:  readOnly,
		VolumeID:  volumeID,
	}
	specMap["capacity"] = map[string]string{
		"storage": stringStorage,
	}

	unstructuredPersistentVolume.Object["spec"] = specMap

	dynamicClient, err := client.GetDownStreamClusterClient(clusterName)
	if err != nil {
		return nil, err
	}

	persistentVolumesResource := dynamicClient.Resource(persistentvolumes.PersistentVolumesGroupVersionResource).Namespace("")

	unstructuredResp, err := persistentVolumesResource.Create(context.TODO(), unstructuredPersistentVolume, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	newPersistentVolume := &corev1.PersistentVolume{}
	err = scheme.Scheme.Convert(unstructuredResp, newPersistentVolume, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	return newPersistentVolume, nil
}
