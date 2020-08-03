package projectsetter

import (
	"net/http"
	"testing"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/project"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Tests that getMatchingNamespaces returns the proper amount namespaces given duplicates, empty slices, and multiple
// matching namespaces.
func TestOptionsCorrectNamespaces(t *testing.T) {
	dummyAPIContext := &types.APIContext{
		Method:     http.MethodGet,
		SubContext: map[string]string{"/v3/schemas/project": "p-test123"},
	}

	namespaceList := []*v1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "ns1",
				Annotations: map[string]string{project.ProjectIDAnn: "p-test123"},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "ns2",
				Annotations: map[string]string{project.ProjectIDAnn: "p-test1234"},
			},
		},
	}

	namespaces := getMatchingNamespaces(*dummyAPIContext, namespaceList)

	if len(namespaces) == 0 {
		t.Error("Matching namespace was not returned")
	}

	namespaceList = []*v1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "ns1",
				Annotations: map[string]string{project.ProjectIDAnn: "p-test123"},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "ns2",
				Annotations: map[string]string{project.ProjectIDAnn: "p-test1234"},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "ns3",
				Annotations: map[string]string{project.ProjectIDAnn: "p-test123"},
			},
		},
	}

	namespaces = getMatchingNamespaces(*dummyAPIContext, namespaceList)
	if len(namespaces) != 2 {
		t.Error("Should be able to find multiple namespaces")
	}

	namespaceList = []*v1.Namespace{}
	namespaces = getMatchingNamespaces(*dummyAPIContext, namespaceList)

	if len(namespaces) > 0 {
		t.Error("Namespaces should not be returned from empty namespace list")
	}
}
