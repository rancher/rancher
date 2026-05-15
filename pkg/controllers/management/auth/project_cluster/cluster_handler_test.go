package project_cluster

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/labels"

	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	userID            = "u-abcdef"
	userPrincipalName = "keycloak_user://12345"
	projectName       = "test-project"
	clusterName       = "test-cluster"
)

func getExpectedVerbs(roleName string) []string {
	if roleName == "test-cluster-clusterowner" {
		return []string{"get", "update", "delete", "patch", "create", "list", "watch", "deletecollection"}
	}
	return []string{"get"}
}

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
	clusterClient.EXPECT().UpdateStatus(gomock.Any()).DoAndReturn(func(obj *apisv3.Cluster) (*apisv3.Cluster, error) {
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

func TestSyncPersistsCreatorConditionsViaUpdateStatus(t *testing.T) {
	clusterName := "test-cluster"
	userID := "u-abcdef"
	userPrincipalName := "keycloak_user://12345"

	cluster := &apisv3.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name: clusterName,
			Annotations: map[string]string{
				roleTemplatesRequiredAnnotation: `{"created":[],"required":["cluster-owner"]}`,
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
		return obj, nil
	}).AnyTimes()

	clusterClient := fake.NewMockNonNamespacedControllerInterface[*apisv3.Cluster, *apisv3.ClusterList](ctrl)
	// Get is called by updateClusterAnnotation and by Sync's status update block
	clusterClient.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(func(name string, opts v1.GetOptions) (*apisv3.Cluster, error) {
		return cluster.DeepCopy(), nil
	}).AnyTimes()
	// Update is called by updateClusterAnnotation for the annotation change
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

	// Verify conditions are set on the in-memory object so Sync's
	// status diff will detect them and call UpdateStatus
	result := obj.(*apisv3.Cluster)
	orig := &apisv3.Cluster{}
	require.False(t, reflect.DeepEqual(orig.Status, result.Status),
		"status should have changed with CreatorMadeOwner and InitialRolesPopulated conditions")
	assert.True(t, apisv3.CreatorMadeOwner.IsTrue(result),
		"CreatorMadeOwner condition should be set on returned object")
	assert.True(t, apisv3.ClusterConditionInitialRolesPopulated.IsTrue(result),
		"InitialRolesPopulated condition should be set on returned object")
}

func TestSyncCopiesOwnedConditions(t *testing.T) {
	clusterName := "test-cluster"

	cluster := &apisv3.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name: clusterName,
			Annotations: map[string]string{
				roleTemplatesRequiredAnnotation: `{"created":[],"required":["cluster-owner"]}`,
				CreatorIDAnnotation:             "u-abcdef",
				creatorPrincipalNameAnnotation:  "keycloak_user://12345",
			},
		},
	}

	ctrl := gomock.NewController(t)

	// Mock namespace operations
	nsLister := fake.NewMockNonNamespacedCacheInterface[*corev1.Namespace](ctrl)
	nsLister.EXPECT().Get(clusterName).Return(nil, apierrors.NewNotFound(corev1.Resource("namespace"), clusterName)).AnyTimes()
	nsClient := fake.NewMockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList](ctrl)
	nsClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(ns *corev1.Namespace) (*corev1.Namespace, error) {
		return ns, nil
	}).AnyTimes()

	// Mock project operations - no projects exist
	projectLister := fake.NewMockCacheInterface[*apisv3.Project](ctrl)
	projectLister.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

	projects := fake.NewMockClientInterface[*apisv3.Project, *apisv3.ProjectList](ctrl)
	projects.EXPECT().List(gomock.Any(), gomock.Any()).Return(&apisv3.ProjectList{}, nil).AnyTimes()
	projects.EXPECT().Create(gomock.Any()).DoAndReturn(func(p *apisv3.Project) (*apisv3.Project, error) {
		return p, nil
	}).AnyTimes()

	roleTemplateLister := fake.NewMockNonNamespacedCacheInterface[*apisv3.RoleTemplate](ctrl)
	roleTemplateLister.EXPECT().List(gomock.Any()).Return([]*apisv3.RoleTemplate{
		{
			ObjectMeta:            v1.ObjectMeta{Name: "cluster-owner"},
			ClusterCreatorDefault: true,
		},
	}, nil).AnyTimes()

	// Mock CRTB operations
	crtbLister := fake.NewMockCacheInterface[*apisv3.ClusterRoleTemplateBinding](ctrl)
	crtbLister.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

	crtbClient := fake.NewMockControllerInterface[*apisv3.ClusterRoleTemplateBinding, *apisv3.ClusterRoleTemplateBindingList](ctrl)
	crtbClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(obj *apisv3.ClusterRoleTemplateBinding) (*apisv3.ClusterRoleTemplateBinding, error) {
		return obj, nil
	}).AnyTimes()

	// Mock cluster role operations
	crClient := fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl)
	crClient.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, apierrors.NewNotFound(rbacv1.Resource("clusterrole"), "")).AnyTimes()
	crClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(cr *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
		return cr, nil
	}).AnyTimes()

	// Mock cluster operations
	clusterClient := fake.NewMockNonNamespacedControllerInterface[*apisv3.Cluster, *apisv3.ClusterList](ctrl)
	clusterClient.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(func(name string, opts v1.GetOptions) (*apisv3.Cluster, error) {
		return cluster.DeepCopy(), nil
	}).AnyTimes()

	clusterClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(obj *apisv3.Cluster) (*apisv3.Cluster, error) {
		return obj, nil
	}).AnyTimes()

	var statusUpdateCluster *apisv3.Cluster
	clusterClient.EXPECT().UpdateStatus(gomock.Any()).DoAndReturn(func(obj *apisv3.Cluster) (*apisv3.Cluster, error) {
		statusUpdateCluster = obj.DeepCopy()
		return obj, nil
	}).Times(1)

	lifecycle := &clusterLifecycle{
		clusterClient:      clusterClient,
		crtbLister:         crtbLister,
		crtbClient:         crtbClient,
		nsLister:           nsLister,
		nsClient:           nsClient,
		projects:           projects,
		projectLister:      projectLister,
		roleTemplateLister: roleTemplateLister,
		crClient:           crClient,
	}

	_, err := lifecycle.Sync(clusterName, cluster)
	require.NoError(t, err)

	// Verify that UpdateStatus was called
	require.NotNil(t, statusUpdateCluster, "UpdateStatus should have been called")

	expectedConditionTypes := map[string]bool{
		"BackingNamespaceCreated": true,
		"DefaultProjectCreated":   true,
		"SystemProjectCreated":    true,
		"CreatorMadeOwner":        true,
		"InitialRolesPopulated":   true,
	}

	actualConditionTypes := make(map[string]bool)
	for _, cond := range statusUpdateCluster.Status.Conditions {
		actualConditionTypes[string(cond.Type)] = true
	}

	assert.Equal(t, expectedConditionTypes, actualConditionTypes,
		"UpdateStatus should receive all conditions set by controller logic - if this fails, a CopyCondition call is missing")
}

func TestSyncNoUpdateWhenSteadyState(t *testing.T) {
	clusterName := "test-cluster"

	cluster := &apisv3.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name: clusterName,
			Annotations: map[string]string{
				roleTemplatesRequiredAnnotation: `{"created":["cluster-owner"],"required":["cluster-owner"]}`,
				CreatorIDAnnotation:             "u-abcdef",
				NoCreatorRBACAnnotation:         "true", // skip creator RTB path
			},
		},
	}
	// Pre-set all conditions to True
	apisv3.NamespaceBackedResource.True(cluster)
	apisv3.ClusterConditionDefaultProjectCreated.True(cluster)
	apisv3.ClusterConditionSystemProjectCreated.True(cluster)

	ctrl := gomock.NewController(t)

	// Mock namespace operations - namespace already exists
	nsLister := fake.NewMockNonNamespacedCacheInterface[*corev1.Namespace](ctrl)
	nsLister.EXPECT().Get(clusterName).Return(&corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{Name: clusterName},
	}, nil).AnyTimes()
	nsClient := fake.NewMockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList](ctrl)

	// Mock project operations - projects already exist
	projectLister := fake.NewMockCacheInterface[*apisv3.Project](ctrl)
	projectLister.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(namespace string, selector labels.Selector) ([]*apisv3.Project, error) {
			// Return existing projects so createProject is skipped
			return []*apisv3.Project{{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-project",
					Namespace: namespace,
				},
			}}, nil
		}).AnyTimes()

	projects := fake.NewMockClientInterface[*apisv3.Project, *apisv3.ProjectList](ctrl)
	roleTemplateLister := fake.NewMockNonNamespacedCacheInterface[*apisv3.RoleTemplate](ctrl)
	roleTemplateLister.EXPECT().List(gomock.Any()).Return([]*apisv3.RoleTemplate{}, nil).AnyTimes()

	// Mock cluster role operations - return roles that match what the controller expects
	crClient := fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl)
	crClient.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(
		func(name string, opts v1.GetOptions) (*rbacv1.ClusterRole, error) {
			// Return a cluster role that matches what createClusterMembershipRoles expects
			// so no update is triggered
			return &rbacv1.ClusterRole{
				ObjectMeta: v1.ObjectMeta{
					Name: name,
					Annotations: map[string]string{
						"cluster.cattle.io/name": clusterName,
					},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups:     []string{"management.cattle.io"},
						Resources:     []string{"clusters"},
						ResourceNames: []string{clusterName},
						Verbs:         getExpectedVerbs(name),
					},
				},
			}, nil
		}).AnyTimes()
	// Update should NOT be called in steady state
	crClient.EXPECT().Update(gomock.Any()).Times(0)

	// Mock cluster operations - should NOT be called
	clusterClient := fake.NewMockNonNamespacedControllerInterface[*apisv3.Cluster, *apisv3.ClusterList](ctrl)
	clusterClient.EXPECT().Update(gomock.Any()).Times(0)
	clusterClient.EXPECT().UpdateStatus(gomock.Any()).Times(0)

	lifecycle := &clusterLifecycle{
		clusterClient:      clusterClient,
		nsLister:           nsLister,
		nsClient:           nsClient,
		projects:           projects,
		projectLister:      projectLister,
		roleTemplateLister: roleTemplateLister,
		crClient:           crClient,
	}

	_, err := lifecycle.Sync(clusterName, cluster)
	require.NoError(t, err)
}
