package secrets

import (
	"context"
	"fmt"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/api/scheme"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
)

type PatchOP string

const (
	AddPatchOP     PatchOP = "add"
	ReplacePatchOP PatchOP = "replace"
	RemovePatchOP  PatchOP = "remove"
)

// PatchSecret is a helper function that uses the dynamic client to patch a secret in a namespace for a specific cluster.
// Different secret operations are supported: add, replace, remove.
func PatchSecret(client *rancher.Client, clusterID, secretName, namespace string, patchType types.PatchType, patchOp PatchOP, patchPath, patchData string, patchOpts metav1.PatchOptions) (*coreV1.Secret, error) {
	patchJSONOperation := fmt.Sprintf(`
	[
	  { "op": "%v", "path": "%v", "value": "%v" }
	]
	`, patchOp, patchPath, patchData)

	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return nil, err
	}

	secretResource := dynamicClient.Resource(SecretGroupVersionResource).Namespace(namespace)

	unstructuredResp, err := secretResource.Patch(context.TODO(), secretName, patchType, []byte(patchJSONOperation), patchOpts)
	if err != nil {
		return nil, err
	}

	newSecret := &coreV1.Secret{}
	err = scheme.Scheme.Convert(unstructuredResp, newSecret, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}
	return newSecret, nil
}
