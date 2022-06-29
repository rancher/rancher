package auth

import (
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"testing"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const clusterID = "test-cluster"

func TestEnqueueCrtbsOnProjectCreation(t *testing.T) {
	existingCrtbs := []*v32.ClusterRoleTemplateBinding{
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
				ListFunc: func(namespace string, selector labels.Selector) ([]*v32.ClusterRoleTemplateBinding, error) {
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

	newProject := v32.Project{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-project",
			Namespace: clusterID,
		},
	}
	assert.NoError(t, c.enqueueCrtbs(&newProject))
	assert.Equal(t, len(existingCrtbs), len(mockedClusterRoleTemplateBindingController.EnqueueCalls()))
}
