package auth

import (
	"testing"

	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

const testNamespace = "test-cluster"
const testRoleTemplateName = "fake-role-template"

func TestEnqueuePrtbsOnRoleTemplateUpdate(t *testing.T) {

	existingPrtbs := []*v3.ProjectRoleTemplateBinding{
		{
			ObjectMeta: v1.ObjectMeta{
				Name:      "prtb-1",
				Namespace: testNamespace,
			},
			RoleTemplateName: testRoleTemplateName,
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name:      "prtb-2",
				Namespace: testNamespace,
			},
			RoleTemplateName: "this-is-not-the-rt-you-are-looking-for",
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name:      "prtb-3",
				Namespace: testNamespace,
			},
			RoleTemplateName: testRoleTemplateName,
		},
	}

	// Mock prtb controller to catch enqueue calls
	mockedProjectRoleTemplateBindingController := fakes.ProjectRoleTemplateBindingControllerMock{
		EnqueueFunc: func(namespace string, name string) {},
	}

	// Setup a mock indexer that uses our custom index and method, then add test objects
	indexers := map[string]cache.IndexFunc{
		prtbByRoleTemplateIndex: prtbByRoleTemplate,
	}
	mockIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	mockIndexer.AddIndexers(indexers)
	for _, obj := range existingPrtbs {
		mockIndexer.Add(obj)
	}

	rtl := roleTemplateHandler{
		prtbIndexer: mockIndexer,
		prtbClient: &fakes.ProjectRoleTemplateBindingInterfaceMock{
			ControllerFunc: func() v3.ProjectRoleTemplateBindingController {
				return &mockedProjectRoleTemplateBindingController
			},
		},
	}

	updatedRT := v3.RoleTemplate{
		ObjectMeta: v1.ObjectMeta{
			Name:      testRoleTemplateName,
			Namespace: testNamespace,
		},
	}
	// Now pass in a roleTemplate with name that matches a subset of test objects
	err := rtl.enqueuePrtbs(&updatedRT)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(mockedProjectRoleTemplateBindingController.EnqueueCalls()))
}
