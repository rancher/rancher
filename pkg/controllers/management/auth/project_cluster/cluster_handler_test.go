package project_cluster

import (
	"testing"

	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	userID            = "u-abcdef"
	userPrincipalName = "keycloak_user://12345"
	projectName       = "test-project"
	clusterName       = "test-cluster"
)

func TestClusterLifeCycleCreateProjectAnnotations(t *testing.T) {
	tests := []struct {
		name                      string
		clusterAnnotations        map[string]string
		expectedProjectAnnotation map[string]string
	}{
		{
			name: "create respects principal name",
			clusterAnnotations: map[string]string{
				CreatorIDAnnotation:            userID,
				creatorPrincipalNameAnnotation: userPrincipalName,
			},
			expectedProjectAnnotation: map[string]string{
				CreatorIDAnnotation:            userID,
				creatorPrincipalNameAnnotation: userPrincipalName,
			},
		},
		{
			name: "create propagates noCreatorRBAC annotation",
			clusterAnnotations: map[string]string{
				CreatorIDAnnotation:            userID,
				creatorPrincipalNameAnnotation: userPrincipalName,
				NoCreatorRBACAnnotation:        "true",
			},
			expectedProjectAnnotation: map[string]string{
				NoCreatorRBACAnnotation: "true",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var project *apisv3.Project

			ctrl := gomock.NewController(t)
			projects := fake.NewMockClientInterface[*apisv3.Project, *apisv3.ProjectList](ctrl)
			projects.EXPECT().List(gomock.Any(), gomock.Any()).Return(&apisv3.ProjectList{}, nil).Times(1)
			projects.EXPECT().Create(gomock.Any()).DoAndReturn(func(p *apisv3.Project) (*apisv3.Project, error) {
				project = p.DeepCopy()
				return project, nil
			})

			projectLister := fake.NewMockCacheInterface[*apisv3.Project](ctrl)
			projectLister.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, nil)

			roleTemplateLister := fake.NewMockNonNamespacedCacheInterface[*apisv3.RoleTemplate](ctrl)
			roleTemplateLister.EXPECT().List(gomock.Any()).Return(nil, nil)

			lifecycle := &clusterLifecycle{
				projects:           projects,
				projectLister:      projectLister,
				roleTemplateLister: roleTemplateLister,
			}

			cluster := &apisv3.Cluster{
				ObjectMeta: v1.ObjectMeta{
					Name:        clusterName,
					Annotations: test.clusterAnnotations,
				},
			}

			obj, err := lifecycle.createProject(projectName, apisv3.ClusterConditionSystemProjectCreated, cluster, defaultProjectLabels)
			require.NoError(t, err)
			require.NotNil(t, obj)

			require.NotNil(t, project)
			assert.Equal(t, clusterName, project.Spec.ClusterName)
			assert.Equal(t, projectName, project.Spec.DisplayName)
			assert.Subset(t, project.Annotations, test.expectedProjectAnnotation)
		})
	}
}

func TestReconcileClusterCreatorRTBRespectsUserPrincipalName(t *testing.T) {
	var crtbs []*apisv3.ClusterRoleTemplateBinding

	clusterName := "test-cluster"
	userID := "u-abcdef"
	userPrincipalName := "keycloak_user://12345"

	cluster := &apisv3.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name: clusterName,
			Annotations: map[string]string{
				roleTemplatesRequiredAnnotation: `{"created":["cluster-owner"],"required":["cluster-owner"]}`,
				CreatorIDAnnotation:             userID,
				creatorPrincipalNameAnnotation:  userPrincipalName,
			},
		},
	}

	ctrl := gomock.NewController(t)

	crtbLister := fake.NewMockCacheInterface[*apisv3.ClusterRoleTemplateBinding](ctrl)
	crtbLister.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

	crtbClient := fake.NewMockControllerInterface[*apisv3.ClusterRoleTemplateBinding, *apisv3.ClusterRoleTemplateBindingList](ctrl)
	crtbClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(obj *apisv3.ClusterRoleTemplateBinding) (*apisv3.ClusterRoleTemplateBinding, error) {
		crtbs = append(crtbs, obj)
		return obj, nil
	}).AnyTimes()

	clusterClient := fake.NewMockNonNamespacedControllerInterface[*apisv3.Cluster, *apisv3.ClusterList](ctrl)
	clusterClient.EXPECT().Get(gomock.Any(), gomock.Any()).Return(cluster, nil).AnyTimes()
	clusterClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(obj *apisv3.Cluster) (*apisv3.Cluster, error) {
		return obj, nil
	}).AnyTimes()

	lifecycle := &clusterLifecycle{
		crtbLister:    crtbLister,
		crtbClient:    crtbClient,
		clusterClient: clusterClient,
	}

	obj, err := lifecycle.reconcileClusterCreatorRTB(cluster)
	require.NoError(t, err)
	require.NotNil(t, obj)

	require.Len(t, crtbs, 1)
	assert.Equal(t, "creator-cluster-owner", crtbs[0].Name)
	assert.Equal(t, clusterName, crtbs[0].Namespace)
	assert.Equal(t, clusterName, crtbs[0].ClusterName)
	assert.Equal(t, "", crtbs[0].UserName)
	assert.Equal(t, userPrincipalName, crtbs[0].UserPrincipalName)
}

func TestReconcileClusterCreatorRTBNoCreatorRBAC(t *testing.T) {
	// When NoCreatorRBACAnnotation is set, nothing in the lifecycle will be called
	lifecycle := &clusterLifecycle{}

	cluster := &apisv3.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Annotations: map[string]string{
				NoCreatorRBACAnnotation: "true",
			},
		},
	}
	obj, err := lifecycle.reconcileClusterCreatorRTB(cluster)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
}
