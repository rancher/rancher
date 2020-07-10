package auth

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const clusterID = "test-cluster"

func TestEnqueueCrtbsOnProjectCreation(t *testing.T) {
	existingCrtbs := []*v3.ClusterRoleTemplateBinding{
		{ObjectMeta: v1.ObjectMeta{
			Name:      "crtb-1",
			Namespace: clusterID,
		}},
		{ObjectMeta: v1.ObjectMeta{
			Name:      "crtb-2",
			Namespace: clusterID,
		}},
	}

	mockedClusterRoleTemplateBindingController := fakes.ClusterRoleTemplateBindingControllerMock{
		EnqueueFunc: func(namespace string, name string) {},
	}
	c := projectLifecycle{
		mgr: &mgr{
			crtbLister: &fakes.ClusterRoleTemplateBindingListerMock{
				ListFunc: func(namespace string, selector labels.Selector) ([]*v3.ClusterRoleTemplateBinding, error) {
					return existingCrtbs, nil
				},
			},
			crtbClient: &fakes.ClusterRoleTemplateBindingInterfaceMock{
				ControllerFunc: func() v3.ClusterRoleTemplateBindingController {
					return &mockedClusterRoleTemplateBindingController
				},
			},
		},
	}

	newProject := v3.Project{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-project",
			Namespace: clusterID,
		},
	}
	c.enqueueCrtbs(&newProject)
	assert.Equal(t, len(existingCrtbs), len(mockedClusterRoleTemplateBindingController.EnqueueCalls()))
}
