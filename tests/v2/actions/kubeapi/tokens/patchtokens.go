package tokens

import (
	"context"
	"fmt"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/api/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

var TokenGroupVersionResource = schema.GroupVersionResource{
	Group:    "management.cattle.io",
	Version:  "v3",
	Resource: "tokens",
}

// PatchToken is a helper function that uses the dynamic client to patch a token by its name.
// Different token operations are supported: add, replace, remove.
func PatchToken(client *rancher.Client, clusterID, tokenName, patchOp, patchPath, patchData string) (*v3.Token, *unstructured.Unstructured, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return nil, nil, err
	}

	tokenResource := dynamicClient.Resource(TokenGroupVersionResource)

	patchJSONOperation := fmt.Sprintf(`
	[
	  { "op": "%v", "path": "%v", "value": "%v" }
	]
	`, patchOp, patchPath, patchData)

	unstructuredResp, err := tokenResource.Patch(context.TODO(), tokenName, types.JSONPatchType, []byte(patchJSONOperation), metav1.PatchOptions{})
	if err != nil {
		return nil, nil, err
	}

	newToken := &v3.Token{}
	err = scheme.Scheme.Convert(unstructuredResp, newToken, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, nil, err
	}
	return newToken, unstructuredResp, nil
}
