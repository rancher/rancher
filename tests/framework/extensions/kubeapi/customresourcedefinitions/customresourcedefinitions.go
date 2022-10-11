package customresourcedefinitions

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var CustomResourceDefinitions = schema.GroupVersionResource{
	Group:    "apiextensions.k8s.io",
	Version:  "v1",
	Resource: "customresourcedefinitions",
}

// gets a list of names of custom resource definitions that contain the input string name from an Unstructured List
func GetCustomResourceDefinitionsListByName(CRDList *unstructured.UnstructuredList, name string) []string {
	var CRDNameList []string
	CRDs := *CRDList
	for _, unstructuredCRD := range CRDs.Items {
		CRDName := unstructuredCRD.GetName()
		if strings.Contains(CRDName, name) {
			CRDNameList = append(CRDNameList, CRDName)
		}
	}

	return CRDNameList
}
