package project_cluster

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	ctrl := gomock.NewController(t)

	crtbLister := fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
	crtbLister.EXPECT().List(gomock.Any(), gomock.Any()).Return(existingCrtbs, nil)

	crtbClient := fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
	crtbClient.EXPECT().Enqueue(gomock.Any(), gomock.Any()).Return().AnyTimes()

	c := projectLifecycle{
		crtbLister: crtbLister,
		crtbClient: crtbClient,
	}

	newProject := v3.Project{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test-project",
			Namespace: clusterID,
		},
	}
	c.enqueueCrtbs(&newProject)
	assert.Equal(t, len(existingCrtbs), 2)
}

func TestReconcileProjectCreatorRTBRespectsUserPrincipalName(t *testing.T) {
	var prtbs []*v3.ProjectRoleTemplateBinding

	ctrl := gomock.NewController(t)

	prtbLister := fake.NewMockCacheInterface[*v3.ProjectRoleTemplateBinding](ctrl)
	prtbLister.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

	prtbClient := fake.NewMockControllerInterface[*v3.ProjectRoleTemplateBinding, *v3.ProjectRoleTemplateBindingList](ctrl)
	prtbClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(obj *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
		prtbs = append(prtbs, obj)
		return obj, nil
	}).AnyTimes()

	projects := fake.NewMockControllerInterface[*v3.Project, *v3.ProjectList](ctrl)
	projects.EXPECT().Update(gomock.Any()).DoAndReturn(func(obj *v3.Project) (*v3.Project, error) {
		return obj, nil
	}).AnyTimes()

	lifecycle := &projectLifecycle{
		prtbLister: prtbLister,
		prtbClient: prtbClient,
		projects:   projects,
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

	obj, err := lifecycle.reconcileProjectCreatorRTB(project)
	require.NoError(t, err)
	require.NotNil(t, obj)

	require.Len(t, prtbs, 1)
	assert.Equal(t, "creator-project-owner", prtbs[0].Name)
	assert.Equal(t, "p-abcdef", prtbs[0].Namespace)
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
	obj, err := lifecycle.reconcileProjectCreatorRTB(project)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}
