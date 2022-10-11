package awsebs

import (
	"context"
	"strconv"

	"github.com/rancher/rancher/pkg/api/scheme"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/kubeapi/storageclasses"
	"github.com/rancher/rancher/tests/framework/extensions/unstructured"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateAWSEBSPersistentVolume is a helper function that uses the dynamic client to create an aws ebs persistent volume for a specific cluster.
// It registers a delete fuction. `iopsPerGB` and `encryptionKey` are optional parameters, and can just take ""
func CreateAWSEBSStorageClass(client *rancher.Client, clusterName, fsType, encryptionKey, iopsPerGB string, volumeType VolumeType, encryption bool, storageClass *storagev1.StorageClass) (*storagev1.StorageClass, error) {
	storageClass.Provisioner = "kubernetes.io/aws-ebs"
	storageClass.Parameters = map[string]string{
		"encrypted": strconv.FormatBool(encryption),
		"fsType":    fsType,
		"type":      string(volumeType),
	}

	if encryptionKey != "" {
		storageClass.Parameters["kmsKeyId"] = encryptionKey
	}

	if iopsPerGB != "" {
		storageClass.Parameters["iopsPerGB"] = iopsPerGB
	} else {
		storageClass.Parameters["iopsPerGB"] = "0"
	}

	unstructuredStorageClass := unstructured.MustToUnstructured(storageClass)

	dynamicClient, err := client.GetDownStreamClusterClient(clusterName)
	if err != nil {
		return nil, err
	}

	storageClassVolumesResource := dynamicClient.Resource(storageclasses.StorageClassGroupVersionResource).Namespace("")

	unstructuredResp, err := storageClassVolumesResource.Create(context.TODO(), unstructuredStorageClass, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	newStorageClass := &storagev1.StorageClass{}
	err = scheme.Scheme.Convert(unstructuredResp, newStorageClass, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	return newStorageClass, nil
}
