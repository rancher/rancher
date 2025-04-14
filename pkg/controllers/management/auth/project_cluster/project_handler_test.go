package project_cluster

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestReconcileProjectCreatorRTBRespectsUserPrincipalName(t *testing.T) {
	var prtbs []*v3.ProjectRoleTemplateBinding

	lifecycle := &projectLifecycle{
		prtbLister: &fakes.ProjectRoleTemplateBindingListerMock{
			GetFunc: func(namespace string, name string) (*v3.ProjectRoleTemplateBinding, error) {
				return nil, nil
			},
		},
		prtbClient: &fakes.ProjectRoleTemplateBindingInterfaceMock{
			CreateFunc: func(obj *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
				prtbs = append(prtbs, obj)
				return obj, nil
			},
		},
		projects: &fakes.ProjectInterfaceMock{
			UpdateFunc: func(obj *v3.Project) (*v3.Project, error) {
				return obj, nil
			},
		},
	}

	userPrincipalName := "keycloak_user@12345"

	project := &v3.Project{
		ObjectMeta: v1.ObjectMeta{
			Name:      "p-abcdef",
			Namespace: clusterID,
			Annotations: map[string]string{
				CreatorIDAnnotation:             "u-abcdef",
				creatorPrincipalNameAnnotation:  userPrincipalName,
				roleTemplatesRequiredAnnotation: `{"created":["project-owner"],"required":["project-owner"]}`,
			},
		},
	}

	obj, err := lifecycle.reconcileProjectCreatorRTB(project, clusterID)
	require.NoError(t, err)
	require.NotNil(t, obj)

	require.Len(t, prtbs, 1)
	assert.Equal(t, "creator-project-owner", prtbs[0].Name)
	assert.Equal(t, clusterID, prtbs[0].Namespace)
	assert.Equal(t, clusterID+":p-abcdef", prtbs[0].ProjectName)
	assert.Equal(t, "", prtbs[0].UserName)
	assert.Equal(t, userPrincipalName, prtbs[0].UserPrincipalName)
}

func TestReconcileProjectCreatorRTBNoCreatorRBAC(t *testing.T) {
	// When NoCreatorRBACAnnotation is set, nothing in the lifecycle will be called
	lifecycle := &projectLifecycle{}
	project := &v3.Project{
		ObjectMeta: v1.ObjectMeta{
			Annotations: map[string]string{
				NoCreatorRBACAnnotation: "true",
			},
		},
	}
	obj, err := lifecycle.reconcileProjectCreatorRTB(project, clusterID)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}
