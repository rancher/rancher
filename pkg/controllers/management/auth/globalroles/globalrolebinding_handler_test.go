package globalroles

import (
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers"
	"github.com/rancher/rancher/pkg/controllers/status"
	userMocks "github.com/rancher/rancher/pkg/user/mocks"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	inheritedTestGr = v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inherit-test-gr",
		},
		InheritedClusterRoles: []string{"cluster-owner"},
	}
	noInheritTestGR = v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "noinherit-test-gr",
		},
		InheritedClusterRoles: []string{},
	}
	missingInheritTestGR = v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "missing-test-gr",
		},
	}
	purgeTestGR = v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "purge-inherit-test-gr",
		},
		InheritedClusterRoles: []string{"already-exists", "missing",
			"wrong-cluster-name", "wrong-user-name", "wrong-group-name",
			"deleting", "duplicate"},
	}
	notLocalCluster = v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "not-local",
		},
	}
	errorCluster = v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "error",
		},
	}
	localCluster = v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "local",
		},
	}
	namespacedRulesGRB = v3.GlobalRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "namespacedRulesGRB",
			UID:  "1234",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "fake-kind",
			APIVersion: "fake-version",
		},
		UserName:       "username",
		GlobalRoleName: "namespacedRulesGR",
	}
)

func Test_crtbGrbOwnerIndexer(t *testing.T) {
	t.Parallel()
	grbOwnedCRTB := &v3.ClusterRoleTemplateBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "crtb-grb-alread-exists",
			Labels: map[string]string{
				grbOwnerLabel: "test-grb",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "management.cattle.io/v3",
					Kind:       "GlobalRoleBinding",
					UID:        "1234",
					Name:       "other-grb",
				},
			},
			GenerateName: "crtb-grb-",
			Namespace:    "some-cluster",
		},
		RoleTemplateName:   "already-exists",
		ClusterName:        "other-cluster",
		UserName:           "test-user",
		GroupPrincipalName: "",
	}
	keys, err := crtbGrbOwnerIndexer(grbOwnedCRTB)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Equal(t, "other-cluster/test-grb", keys[0])

	noLabelCRTB := &v3.ClusterRoleTemplateBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "crtb-grb-alread-exists",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "management.cattle.io/v3",
					Kind:       "GlobalRoleBinding",
					UID:        "1234",
					Name:       "other-grb",
				},
			},
			GenerateName: "crtb-grb-",
			Namespace:    "some-cluster",
		},
		RoleTemplateName:   "already-exists",
		ClusterName:        "other-cluster",
		UserName:           "test-user",
		GroupPrincipalName: "",
	}
	keys, err = crtbGrbOwnerIndexer(noLabelCRTB)
	require.NoError(t, err)
	require.Len(t, keys, 0)

	standardCRTB := &v3.ClusterRoleTemplateBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "crtb-grb-123xyz",
			Namespace: "some-cluster",
		},
		RoleTemplateName:   "already-exists",
		ClusterName:        "some-cluster",
		UserName:           "test-user",
		GroupPrincipalName: "",
	}
	keys, err = crtbGrbOwnerIndexer(standardCRTB)
	require.NoError(t, err)
	require.Len(t, keys, 0)
}

func TestReconcileClusterPermissions(t *testing.T) {
	t.Parallel()
	defaultCRTB := v3.ClusterRoleTemplateBinding{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "crtb-grb-",
			Namespace:    "not-local",
			Labels: map[string]string{
				grbOwnerLabel:               "test-grb",
				controllers.K8sManagedByKey: controllers.ManagerValue,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "management.cattle.io/v3",
					Kind:       "GlobalRoleBinding",
					Name:       "test-grb",
					UID:        "1234",
				},
			},
		},
		ClusterName:      "not-local",
		RoleTemplateName: "cluster-owner",
		UserName:         "test-user",
	}

	type controllers struct {
		crtbCache      *fake.MockCacheInterface[*v3.ClusterRoleTemplateBinding]
		crtbController *fake.MockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList]
		grCache        *fake.MockNonNamespacedCacheInterface[*v3.GlobalRole]
		clusterCache   *fake.MockNonNamespacedCacheInterface[*v3.Cluster]
	}

	tests := []struct {
		name             string
		setupControllers func(controllers)
		inputObject      *v3.GlobalRoleBinding
		wantError        bool
	}{
		{
			name: "no inherited roles",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(noInheritTestGR.Name).Return(noInheritTestGR.DeepCopy(), nil)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "not-local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil).Times(2)
				c.clusterCache.EXPECT().List(labels.Everything()).Return([]*v3.Cluster{&notLocalCluster, &localCluster}, nil).AnyTimes()
			},
			inputObject: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					UID:  "1234",
				},
				GlobalRoleName: noInheritTestGR.Name,
				UserName:       "test-user",
			},
			wantError: false,
		},
		{
			name: "missing inherited roles",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(missingInheritTestGR.Name).Return(missingInheritTestGR.DeepCopy(), nil)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "not-local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil).Times(2)
				c.clusterCache.EXPECT().List(labels.Everything()).Return([]*v3.Cluster{&notLocalCluster, &localCluster}, nil).AnyTimes()
			},
			inputObject: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					UID:  "1234",
				},
				GlobalRoleName: missingInheritTestGR.Name,
				UserName:       "test-user",
			},
			wantError: false,
		},
		{
			name: "inherited cluster roles",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(inheritedTestGr.Name).Return(inheritedTestGr.DeepCopy(), nil)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "not-local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil).Times(2)
				c.clusterCache.EXPECT().List(labels.Everything()).Return([]*v3.Cluster{&notLocalCluster, &localCluster}, nil).AnyTimes()
				c.crtbController.EXPECT().Create(defaultCRTB.DeepCopy()).Return(nil, nil)
			},
			inputObject: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					UID:  "1234",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "management.cattle.io/v3",
					Kind:       "GlobalRoleBinding",
				},
				GlobalRoleName: inheritedTestGr.Name,
				UserName:       "test-user",
			},
			wantError: false,
		},
		{
			name: "cluster lister error",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(inheritedTestGr.Name).Return(inheritedTestGr.DeepCopy(), nil)
				c.clusterCache.EXPECT().List(labels.Everything()).Return(nil, fmt.Errorf("server unavailable"))
			},
			inputObject: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					UID:  "1234",
				},
				GlobalRoleName: inheritedTestGr.Name,
				UserName:       "test-user",
			},
			wantError: true,
		},
		{
			name: "crtb creation error",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(inheritedTestGr.Name).Return(inheritedTestGr.DeepCopy(), nil)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "not-local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil).Times(2)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "error/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil).Times(2)
				c.clusterCache.EXPECT().List(labels.Everything()).Return([]*v3.Cluster{&errorCluster, &notLocalCluster, &localCluster}, nil)
				c.crtbController.EXPECT().Create(defaultCRTB.DeepCopy()).Return(nil, nil)
				errorCRTB := defaultCRTB.DeepCopy()
				errorCRTB.Namespace = "error"
				errorCRTB.ClusterName = "error"
				c.crtbController.EXPECT().Create(errorCRTB).Return(nil, fmt.Errorf("server unavailable"))
			},
			inputObject: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					UID:  "1234",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "management.cattle.io/v3",
					Kind:       "GlobalRoleBinding",
				},
				GlobalRoleName: inheritedTestGr.Name,
				UserName:       "test-user",
			},
			wantError: true,
		},
		{
			name: "indexer error",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(inheritedTestGr.Name).Return(inheritedTestGr.DeepCopy(), nil)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "not-local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{}, nil).Times(2)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "error/test-grb").Return(nil, fmt.Errorf("indexer error")).Times(2)
				c.clusterCache.EXPECT().List(labels.Everything()).Return([]*v3.Cluster{&errorCluster, &notLocalCluster, &localCluster}, nil).AnyTimes()
				c.crtbController.EXPECT().Create(defaultCRTB.DeepCopy()).Return(nil, nil)
			},
			inputObject: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					UID:  "1234",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "management.cattle.io/v3",
					Kind:       "GlobalRoleBinding",
				},
				GlobalRoleName: inheritedTestGr.Name,
				UserName:       "test-user",
			},
			wantError: true,
		},
		{
			name: "crtb delete error",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(inheritedTestGr.Name).Return(inheritedTestGr.DeepCopy(), nil)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "local/test-grb").Return([]*v3.ClusterRoleTemplateBinding{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:         "crtb-grb-delete-local",
							GenerateName: "crtb-grb-",
							Namespace:    "local",
						},
						RoleTemplateName:   "not-valid",
						ClusterName:        "local",
						UserName:           "test-user",
						GroupPrincipalName: "",
					},
				}, nil)
				c.crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "error/test-grb").Return([]*v3.ClusterRoleTemplateBinding{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:         "crtb-grb-delete",
							GenerateName: "crtb-grb-",
							Namespace:    "error",
						},
						RoleTemplateName:   "not-valid",
						ClusterName:        "error",
						UserName:           "test-user",
						GroupPrincipalName: "",
					},
				}, nil).Times(2)
				c.clusterCache.EXPECT().List(labels.Everything()).Return([]*v3.Cluster{&errorCluster, &localCluster}, nil).AnyTimes()
				c.crtbController.EXPECT().Delete("local", "crtb-grb-delete-local", gomock.Any()).Return(fmt.Errorf("server unavailable"))
				c.crtbController.EXPECT().Delete("error", "crtb-grb-delete", gomock.Any()).Return(fmt.Errorf("server unavailable"))
				errorCRTB := defaultCRTB.DeepCopy()
				errorCRTB.Namespace = "error"
				errorCRTB.ClusterName = "error"
				c.crtbController.EXPECT().Create(errorCRTB).Return(nil, nil)
			},
			inputObject: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					UID:  "1234",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "management.cattle.io/v3",
					Kind:       "GlobalRoleBinding",
				},
				GlobalRoleName: inheritedTestGr.Name,
				UserName:       "test-user",
			},
			wantError: true,
		},
		{
			name: "no global role",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get("error").Return(nil, fmt.Errorf("error"))
			},
			inputObject: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					UID:  "1234",
				},
				GlobalRoleName: "error",
				UserName:       "test-user",
			},
			wantError: true,
		},
	}
	ctrl := gomock.NewController(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			controllers := controllers{
				grCache:        fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](ctrl),
				crtbCache:      fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl),
				clusterCache:   fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl),
				crtbController: fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl),
			}
			if test.setupControllers != nil {
				test.setupControllers(controllers)
			}

			grbLifecycle := globalRoleBindingLifecycle{
				grLister:      controllers.grCache,
				crtbCache:     controllers.crtbCache,
				clusterLister: controllers.clusterCache,
				crtbClient:    controllers.crtbController,
				status:        status.NewStatus(),
			}
			var conditions []metav1.Condition
			resErr := grbLifecycle.reconcileClusterPermissions(test.inputObject, &conditions)
			if test.wantError {
				require.Error(t, resErr)
			} else {
				require.NoError(t, resErr)
			}
		})
	}

}

func TestReconcileGlobalRoleBinding(t *testing.T) {
	t.Parallel()

	testGR := v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-gr",
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get", "list", "watch"},
				APIGroups: []string{""},
				Resources: []string{"pods"},
			},
		},
	}

	testGRWithAnnotation := v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-gr",
			Annotations: map[string]string{
				crNameAnnotation: "custom-cr-name",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get", "list", "watch"},
				APIGroups: []string{""},
				Resources: []string{"pods"},
			},
		},
	}

	testGRB := v3.GlobalRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-grb",
			UID:  "1234",
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "management.cattle.io/v3",
			Kind:       "GlobalRoleBinding",
		},
		GlobalRoleName: "test-gr",
		UserName:       "test-user",
	}

	testGRBWithAnnotation := v3.GlobalRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-grb",
			UID:  "1234",
			Annotations: map[string]string{
				crbNameAnnotation: "custom-crb-name",
			},
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "management.cattle.io/v3",
			Kind:       "GlobalRoleBinding",
		},
		GlobalRoleName: "test-gr",
		UserName:       "test-user",
	}

	expectedCRB := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: crbNamePrefix + "test-grb",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "management.cattle.io/v3",
					Kind:       "GlobalRoleBinding",
					Name:       "test-grb",
					UID:        "1234",
				},
			},
			Labels: globalRoleBindingLabel,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "User",
				Name:     "test-user",
				APIGroup: rbacv1.GroupName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Name: generateCRName("test-gr"),
			Kind: clusterRoleKind,
		},
	}

	type controllers struct {
		crbCache      *fake.MockNonNamespacedCacheInterface[*rbacv1.ClusterRoleBinding]
		crbController *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]
		grCache       *fake.MockNonNamespacedCacheInterface[*v3.GlobalRole]
	}

	tests := []struct {
		name             string
		setupControllers func(controllers)
		inputObject      *v3.GlobalRoleBinding
		wantError        bool
		wantAnnotation   string
	}{
		{
			name: "create new clusterRoleBinding",
			setupControllers: func(c controllers) {
				c.crbCache.EXPECT().Get(crbNamePrefix+"test-grb").Return(nil, nil)
				c.grCache.EXPECT().Get("test-gr").Return(testGR.DeepCopy(), nil)
				c.crbController.EXPECT().Create(expectedCRB.DeepCopy()).Return(&expectedCRB, nil)
			},
			inputObject:    testGRB.DeepCopy(),
			wantError:      false,
			wantAnnotation: crbNamePrefix + "test-grb",
		},
		{
			name: "create new clusterRoleBinding with custom annotation",
			setupControllers: func(c controllers) {
				c.crbCache.EXPECT().Get("custom-crb-name").Return(nil, nil)
				c.grCache.EXPECT().Get("test-gr").Return(testGR.DeepCopy(), nil)
				customCRB := expectedCRB.DeepCopy()
				customCRB.Name = "custom-crb-name"
				c.crbController.EXPECT().Create(customCRB).Return(customCRB, nil)
			},
			inputObject:    testGRBWithAnnotation.DeepCopy(),
			wantError:      false,
			wantAnnotation: "custom-crb-name",
		},
		{
			name: "create new clusterRoleBinding uses CR name from GlobalRole annotation",
			setupControllers: func(c controllers) {
				c.crbCache.EXPECT().Get(crbNamePrefix+"test-grb").Return(nil, nil)
				c.grCache.EXPECT().Get("test-gr").Return(testGRWithAnnotation.DeepCopy(), nil)
				crbWithCustomCR := expectedCRB.DeepCopy()
				crbWithCustomCR.RoleRef.Name = "custom-cr-name"
				c.crbController.EXPECT().Create(crbWithCustomCR).Return(crbWithCustomCR, nil)
			},
			inputObject:    testGRB.DeepCopy(),
			wantError:      false,
			wantAnnotation: crbNamePrefix + "test-grb",
		},
		{
			name: "clusterRoleBinding creation fails",
			setupControllers: func(c controllers) {
				c.crbCache.EXPECT().Get(crbNamePrefix+"test-grb").Return(nil, nil)
				c.grCache.EXPECT().Get("test-gr").Return(testGR.DeepCopy(), nil)
				c.crbController.EXPECT().Create(expectedCRB.DeepCopy()).Return(nil, fmt.Errorf("server unavailable"))
			},
			inputObject: testGRB.DeepCopy(),
			wantError:   true,
		},
		{
			name: "clusterRoleBinding already exists no update needed",
			setupControllers: func(c controllers) {
				existingCRB := expectedCRB.DeepCopy()
				c.crbCache.EXPECT().Get(crbNamePrefix+"test-grb").Return(existingCRB, nil)
				c.grCache.EXPECT().Get("test-gr").Return(testGR.DeepCopy(), nil)
			},
			inputObject: testGRB.DeepCopy(),
			wantError:   false,
		},
		{
			name: "update clusterRoleBinding subject",
			setupControllers: func(c controllers) {
				existingCRB := expectedCRB.DeepCopy()
				existingCRB.Subjects = []rbacv1.Subject{
					{
						Kind:     "User",
						Name:     "old-user",
						APIGroup: rbacv1.GroupName,
					},
				}
				c.crbCache.EXPECT().Get(crbNamePrefix+"test-grb").Return(existingCRB, nil)
				c.grCache.EXPECT().Get("test-gr").Return(testGR.DeepCopy(), nil)
				updatedCRB := expectedCRB.DeepCopy()
				c.crbController.EXPECT().Update(updatedCRB).Return(updatedCRB, nil)
			},
			inputObject: testGRB.DeepCopy(),
			wantError:   false,
		},
		{
			name: "update clusterRoleBinding roleRef",
			setupControllers: func(c controllers) {
				existingCRB := expectedCRB.DeepCopy()
				existingCRB.RoleRef = rbacv1.RoleRef{
					Name: "old-role",
					Kind: clusterRoleKind,
				}
				c.crbCache.EXPECT().Get(crbNamePrefix+"test-grb").Return(existingCRB, nil)
				c.grCache.EXPECT().Get("test-gr").Return(testGR.DeepCopy(), nil)
				updatedCRB := expectedCRB.DeepCopy()
				c.crbController.EXPECT().Update(updatedCRB).Return(updatedCRB, nil)
			},
			inputObject: testGRB.DeepCopy(),
			wantError:   false,
		},
		{
			name: "update clusterRoleBinding subject and roleRef",
			setupControllers: func(c controllers) {
				existingCRB := expectedCRB.DeepCopy()
				existingCRB.Subjects = []rbacv1.Subject{
					{
						Kind:     "User",
						Name:     "old-user",
						APIGroup: rbacv1.GroupName,
					},
				}
				existingCRB.RoleRef = rbacv1.RoleRef{
					Name: "old-role",
					Kind: clusterRoleKind,
				}
				c.crbCache.EXPECT().Get(crbNamePrefix+"test-grb").Return(existingCRB, nil)
				c.grCache.EXPECT().Get("test-gr").Return(testGR.DeepCopy(), nil)
				updatedCRB := expectedCRB.DeepCopy()
				c.crbController.EXPECT().Update(updatedCRB).Return(updatedCRB, nil)
			},
			inputObject: testGRB.DeepCopy(),
			wantError:   false,
		},
		{
			name: "update clusterRoleBinding fails",
			setupControllers: func(c controllers) {
				existingCRB := expectedCRB.DeepCopy()
				existingCRB.Subjects = []rbacv1.Subject{
					{
						Kind:     "User",
						Name:     "old-user",
						APIGroup: rbacv1.GroupName,
					},
				}
				c.crbCache.EXPECT().Get(crbNamePrefix+"test-grb").Return(existingCRB, nil)
				c.grCache.EXPECT().Get("test-gr").Return(testGR.DeepCopy(), nil)
				updatedCRB := expectedCRB.DeepCopy()
				c.crbController.EXPECT().Update(updatedCRB).Return(nil, fmt.Errorf("server unavailable"))
			},
			inputObject: testGRB.DeepCopy(),
			wantError:   true,
		},
		{
			name: "globalRole not found uses generated name",
			setupControllers: func(c controllers) {
				c.crbCache.EXPECT().Get(crbNamePrefix+"test-grb").Return(nil, nil)
				c.grCache.EXPECT().Get("test-gr").Return(nil, apierrors.NewNotFound(schema.GroupResource{
					Group:    "management.cattle.io",
					Resource: "GlobalRole",
				}, "test-gr"))
				c.crbController.EXPECT().Create(expectedCRB.DeepCopy()).Return(&expectedCRB, nil)
			},
			inputObject:    testGRB.DeepCopy(),
			wantError:      false,
			wantAnnotation: crbNamePrefix + "test-grb",
		},
		{
			name: "group principal binding",
			setupControllers: func(c controllers) {
				c.crbCache.EXPECT().Get(crbNamePrefix+"test-grb").Return(nil, nil)
				c.grCache.EXPECT().Get("test-gr").Return(testGR.DeepCopy(), nil)
				groupCRB := expectedCRB.DeepCopy()
				groupCRB.Subjects = []rbacv1.Subject{
					{
						Kind:     "Group",
						Name:     "test-group",
						APIGroup: rbacv1.GroupName,
					},
				}
				c.crbController.EXPECT().Create(groupCRB).Return(groupCRB, nil)
			},
			inputObject: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					UID:  "1234",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "management.cattle.io/v3",
					Kind:       "GlobalRoleBinding",
				},
				GlobalRoleName:     "test-gr",
				GroupPrincipalName: "test-group",
			},
			wantError:      false,
			wantAnnotation: crbNamePrefix + "test-grb",
		},
	}

	ctrl := gomock.NewController(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			controllers := controllers{
				crbCache:      fake.NewMockNonNamespacedCacheInterface[*rbacv1.ClusterRoleBinding](ctrl),
				crbController: fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList](ctrl),
				grCache:       fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](ctrl),
			}
			if test.setupControllers != nil {
				test.setupControllers(controllers)
			}

			grbLifecycle := globalRoleBindingLifecycle{
				crbLister: controllers.crbCache,
				crbClient: controllers.crbController,
				grLister:  controllers.grCache,
				status:    status.NewStatus(),
			}
			var conditions []metav1.Condition
			resErr := grbLifecycle.reconcileGlobalRoleBinding(test.inputObject, &conditions)
			if test.wantError {
				require.Error(t, resErr)
			} else {
				require.NoError(t, resErr)
			}

			if test.wantAnnotation != "" {
				require.Equal(t, test.wantAnnotation, test.inputObject.Annotations[crbNameAnnotation])
			}
		})
	}
}

func Test_reconcileNamespacedPermissions(t *testing.T) {
	t.Parallel()
	activeNamespace := &corev1.Namespace{
		Status: corev1.NamespaceStatus{
			Phase: corev1.NamespaceActive,
		},
	}
	terminatingNamespace := &corev1.Namespace{
		Status: corev1.NamespaceStatus{
			Phase: corev1.NamespaceTerminating,
		},
	}
	errRoleNotFound := apierrors.NewNotFound(schema.GroupResource{
		Group:    "rbac.authorization.k8s.io",
		Resource: "RoleBinding",
	}, "")
	rb1 := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "namespacedRulesGRB-namespace1",
			Namespace: "namespace1",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "fake-version",
					Kind:       "fake-kind",
					Name:       "namespacedRulesGRB",
					UID:        "1234",
				},
			},
			Labels: map[string]string{
				grbOwnerLabel: "namespacedRulesGRB",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     "namespacedRulesGR-namespace1",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "User",
				Name:     "username",
				APIGroup: "rbac.authorization.k8s.io",
			},
		},
	}
	rb2 := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "namespacedRulesGRB-namespace2",
			Namespace: "namespace2",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "fake-version",
					Kind:       "fake-kind",
					Name:       "namespacedRulesGRB",
					UID:        "1234",
				},
			},
			Labels: map[string]string{
				grbOwnerLabel: "namespacedRulesGRB",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     "namespacedRulesGR-namespace2",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "User",
				Name:     "username",
				APIGroup: "rbac.authorization.k8s.io",
			},
		},
	}
	badRB := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "badRB",
			Namespace: "namespace1",
			UID:       "666",
		},
	}

	type controllers struct {
		grCache      *fake.MockNonNamespacedCacheInterface[*v3.GlobalRole]
		nsCache      *fake.MockNonNamespacedCacheInterface[*corev1.Namespace]
		rbCache      *fake.MockCacheInterface[*rbacv1.RoleBinding]
		rbController *fake.MockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList]
	}
	tests := []struct {
		name              string
		setupControllers  func(controllers)
		globalRoleBinding *v3.GlobalRoleBinding
		wantError         bool
	}{
		{
			name: "global role not found",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(namespacedRulesGR.Name).Return(nil, fmt.Errorf("error"))
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         true,
		},
		{
			name: "getting namespace fails",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(namespacedRulesGR.Name).Return(namespacedRulesGR.DeepCopy(), nil)
				c.nsCache.EXPECT().Get(gomock.Any()).Return(activeNamespace, fmt.Errorf("error")).Times(2)
				c.rbCache.EXPECT().List("", gomock.Any()).Return([]*rbacv1.RoleBinding{}, nil)
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         true,
		},
		{
			name: "namespace is nil",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(namespacedRulesGR.Name).Return(namespacedRulesGR.DeepCopy(), nil)
				c.nsCache.EXPECT().Get(gomock.Any()).Return(nil, nil).Times(2)
				c.rbCache.EXPECT().List("", gomock.Any()).Return([]*rbacv1.RoleBinding{}, nil)
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         false,
		},
		{
			name: "getting roleBinding fails",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(namespacedRulesGR.Name).Return(namespacedRulesGR.DeepCopy(), nil)
				c.nsCache.EXPECT().Get(gomock.Any()).Return(activeNamespace, nil).Times(2)
				c.rbCache.EXPECT().Get("namespace1", "namespacedRulesGRB-namespace1").Return(nil, fmt.Errorf("error"))
				c.rbCache.EXPECT().Get("namespace2", "namespacedRulesGRB-namespace2").Return(nil, fmt.Errorf("error"))
				c.rbCache.EXPECT().List("", gomock.Any()).Return([]*rbacv1.RoleBinding{}, nil)
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         true,
		},
		{
			name: "creating roleBinding fails",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(namespacedRulesGR.Name).Return(namespacedRulesGR.DeepCopy(), nil)
				c.nsCache.EXPECT().Get(gomock.Any()).Return(activeNamespace, nil).Times(2)
				c.rbCache.EXPECT().Get("namespace1", "namespacedRulesGRB-namespace1").Return(nil, errRoleNotFound)
				c.rbCache.EXPECT().Get("namespace2", "namespacedRulesGRB-namespace2").Return(nil, errRoleNotFound)
				c.rbController.EXPECT().Create(rb1.DeepCopy()).Return(nil, fmt.Errorf("error"))
				c.rbController.EXPECT().Create(rb2.DeepCopy()).Return(nil, fmt.Errorf("error"))
				c.rbCache.EXPECT().List("", gomock.Any()).Return([]*rbacv1.RoleBinding{}, nil)
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         true,
		},
		{
			name: "roleBindings get created",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(namespacedRulesGR.Name).Return(namespacedRulesGR.DeepCopy(), nil)
				c.nsCache.EXPECT().Get(gomock.Any()).Return(activeNamespace, nil).Times(2)
				c.rbCache.EXPECT().Get("namespace1", "namespacedRulesGRB-namespace1").Return(nil, errRoleNotFound)
				c.rbCache.EXPECT().Get("namespace2", "namespacedRulesGRB-namespace2").Return(nil, errRoleNotFound)
				c.rbController.EXPECT().Create(rb1.DeepCopy()).Return(rb1.DeepCopy(), nil)
				c.rbController.EXPECT().Create(rb2.DeepCopy()).Return(rb2.DeepCopy(), nil)
				c.rbCache.EXPECT().List("", gomock.Any()).Return([]*rbacv1.RoleBinding{rb1.DeepCopy(), rb2.DeepCopy()}, nil)
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         false,
		},
		{
			name: "roleBindings don't get created in a terminating namespace",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(namespacedRulesGR.Name).Return(namespacedRulesGR.DeepCopy(), nil)
				c.nsCache.EXPECT().Get(gomock.Any()).Return(terminatingNamespace, nil).Times(2)
				c.rbCache.EXPECT().Get("namespace1", "namespacedRulesGRB-namespace1").Return(nil, errRoleNotFound)
				c.rbCache.EXPECT().Get("namespace2", "namespacedRulesGRB-namespace2").Return(nil, errRoleNotFound)
				c.rbCache.EXPECT().List("", gomock.Any()).Return([]*rbacv1.RoleBinding{}, nil)
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         false,
		},
		{
			name: "one NS not found, still creates other RB",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(namespacedRulesGR.Name).Return(namespacedRulesGR.DeepCopy(), nil)
				c.nsCache.EXPECT().Get("namespace1").Return(terminatingNamespace, nil)
				c.nsCache.EXPECT().Get("namespace2").Return(activeNamespace, nil)
				c.rbCache.EXPECT().Get("namespace1", "namespacedRulesGRB-namespace1").Return(nil, errRoleNotFound)
				c.rbCache.EXPECT().Get("namespace2", "namespacedRulesGRB-namespace2").Return(nil, errRoleNotFound)
				c.rbController.EXPECT().Create(rb2.DeepCopy()).Return(rb2.DeepCopy(), nil)
				c.rbCache.EXPECT().List("", gomock.Any()).Return([]*rbacv1.RoleBinding{rb2.DeepCopy()}, nil)
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         false,
		},
		{
			name: "delete roleBinding from terminating namespace",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(namespacedRulesGR.Name).Return(namespacedRulesGR.DeepCopy(), nil)
				c.nsCache.EXPECT().Get("namespace1").Return(terminatingNamespace, nil)
				c.nsCache.EXPECT().Get("namespace2").Return(activeNamespace, nil)
				c.rbCache.EXPECT().Get("namespace1", "namespacedRulesGRB-namespace1").Return(badRB.DeepCopy(), nil)
				c.rbController.EXPECT().Delete("namespace1", "badRB", gomock.Any()).Return(nil)
				c.rbCache.EXPECT().Get("namespace2", "namespacedRulesGRB-namespace2").Return(nil, errRoleNotFound)
				c.rbController.EXPECT().Create(rb2.DeepCopy()).Return(rb2.DeepCopy(), nil)
				c.rbCache.EXPECT().List("", gomock.Any()).Return([]*rbacv1.RoleBinding{rb2.DeepCopy()}, nil)
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         false,
		},
		{
			name: "update roleBindings with bad roleRef name",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(namespacedRulesGR.Name).Return(namespacedRulesGR.DeepCopy(), nil)
				c.nsCache.EXPECT().Get("namespace1").Return(activeNamespace, nil)
				c.nsCache.EXPECT().Get("namespace2").Return(activeNamespace, nil)
				c.rbCache.EXPECT().Get("namespace1", "namespacedRulesGRB-namespace1").Return(badRB.DeepCopy(), nil)
				c.rbController.EXPECT().Create(rb1.DeepCopy()).Return(rb1.DeepCopy(), nil)
				c.rbCache.EXPECT().Get("namespace2", "namespacedRulesGRB-namespace2").Return(badRB.DeepCopy(), nil)
				c.rbController.EXPECT().Create(rb2.DeepCopy()).Return(rb2.DeepCopy(), nil)
				c.rbCache.EXPECT().List("", gomock.Any()).Return([]*rbacv1.RoleBinding{rb1.DeepCopy(), rb2.DeepCopy()}, nil)
				c.rbController.EXPECT().Delete("namespace1", "badRB", gomock.Any()).Return(nil).Times(2)
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         false,
		},
		{
			name: "delete roleBindings fails",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(namespacedRulesGR.Name).Return(namespacedRulesGR.DeepCopy(), nil)
				c.nsCache.EXPECT().Get("namespace1").Return(activeNamespace, nil)
				c.nsCache.EXPECT().Get("namespace2").Return(activeNamespace, nil)
				c.rbCache.EXPECT().Get("namespace1", "namespacedRulesGRB-namespace1").Return(badRB.DeepCopy(), nil)
				c.rbController.EXPECT().Delete("namespace1", "badRB", gomock.Any()).Return(fmt.Errorf("error")).Times(2)
				c.rbCache.EXPECT().Get("namespace2", "namespacedRulesGRB-namespace2").Return(badRB.DeepCopy(), nil)
				c.rbCache.EXPECT().List("", gomock.Any()).Return([]*rbacv1.RoleBinding{}, nil)
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         true,
		},
		{
			name: "list RBs fails",
			setupControllers: func(c controllers) {
				c.grCache.EXPECT().Get(namespacedRulesGR.Name).Return(namespacedRulesGR.DeepCopy(), nil)
				c.nsCache.EXPECT().Get(gomock.Any()).Return(activeNamespace, nil).Times(2)
				c.rbCache.EXPECT().Get("namespace1", "namespacedRulesGRB-namespace1").Return(nil, errRoleNotFound)
				c.rbCache.EXPECT().Get("namespace2", "namespacedRulesGRB-namespace2").Return(nil, errRoleNotFound)
				c.rbController.EXPECT().Create(rb1.DeepCopy()).Return(rb1.DeepCopy(), nil)
				c.rbController.EXPECT().Create(rb2.DeepCopy()).Return(rb2.DeepCopy(), nil)
				c.rbCache.EXPECT().List("", gomock.Any()).Return(nil, fmt.Errorf("error"))
			},
			globalRoleBinding: namespacedRulesGRB.DeepCopy(),
			wantError:         true,
		},
	}

	ctrl := gomock.NewController(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			controllers := controllers{
				grCache:      fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](ctrl),
				nsCache:      fake.NewMockNonNamespacedCacheInterface[*corev1.Namespace](ctrl),
				rbCache:      fake.NewMockCacheInterface[*rbacv1.RoleBinding](ctrl),
				rbController: fake.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](ctrl),
			}

			if test.setupControllers != nil {
				test.setupControllers(controllers)
			}

			grbLifecycle := globalRoleBindingLifecycle{
				grLister:          controllers.grCache,
				nsCache:           controllers.nsCache,
				roleBindings:      controllers.rbController,
				roleBindingLister: controllers.rbCache,
				status:            status.NewStatus(),
			}

			var conditions []metav1.Condition
			err := grbLifecycle.reconcileNamespacedRoleBindings(test.globalRoleBinding, &conditions)
			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

type fleetPermissionsHandlerMock struct {
	reconcileFleetWorkspacePermissionsFunc func(globalRoleBinding *v3.GlobalRoleBinding, conditions *[]metav1.Condition) error
}

func (f *fleetPermissionsHandlerMock) reconcileFleetWorkspacePermissionsBindings(globalRoleBinding *v3.GlobalRoleBinding, conditions *[]metav1.Condition) error {
	return f.reconcileFleetWorkspacePermissionsFunc(globalRoleBinding, conditions)
}

func Test_reconcileSubject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		grb               *v3.GlobalRoleBinding
		setupUserManager  func(*userMocks.MockManager)
		setupUserLister   func(*fake.MockNonNamespacedCacheInterface[*v3.User])
		wantGRB           *v3.GlobalRoleBinding
		wantErr           bool
		wantConditionType string
	}{
		{
			name: "group principal name is set - no changes needed",
			grb: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
				},
				GroupPrincipalName: "test-group",
			},
			wantGRB: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
				},
				GroupPrincipalName: "test-group",
			},
			wantErr:           false,
			wantConditionType: subjectExists,
		},
		{
			name: "both user principal and user name set - no changes needed",
			grb: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
				},
				UserName:          "test-user",
				UserPrincipalName: "test-principal",
			},
			wantGRB: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
				},
				UserName:          "test-user",
				UserPrincipalName: "test-principal",
			},
			wantErr:           false,
			wantConditionType: subjectExists,
		},
		{
			name: "user principal name set but user name empty - creates user",
			grb: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					Annotations: map[string]string{
						"auth.cattle.io/principal-display-name": "Test User",
					},
				},
				UserPrincipalName: "test-principal",
			},
			setupUserManager: func(m *userMocks.MockManager) {
				m.EXPECT().EnsureUser("test-principal", "Test User").Return(&v3.User{
					ObjectMeta: metav1.ObjectMeta{Name: "test-user"},
				}, nil)
			},
			wantGRB: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					Annotations: map[string]string{
						"auth.cattle.io/principal-display-name": "Test User",
					},
				},
				UserPrincipalName: "test-principal",
				UserName:          "test-user",
			},
			wantErr:           false,
			wantConditionType: subjectExists,
		},
		{
			name: "user principal name set but user name empty - user creation fails",
			grb: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					Annotations: map[string]string{
						"auth.cattle.io/principal-display-name": "Test User",
					},
				},
				UserPrincipalName: "test-principal",
			},
			setupUserManager: func(m *userMocks.MockManager) {
				m.EXPECT().EnsureUser("test-principal", "Test User").Return(nil, fmt.Errorf("user creation failed"))
			},
			wantGRB: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
					Annotations: map[string]string{
						"auth.cattle.io/principal-display-name": "Test User",
					},
				},
				UserPrincipalName: "test-principal",
			},
			wantErr:           true,
			wantConditionType: failedToCreateUser,
		},
		{
			name: "user name set but user principal name empty - sets principal",
			grb: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
				},
				UserName: "test-user",
			},
			setupUserLister: func(m *fake.MockNonNamespacedCacheInterface[*v3.User]) {
				m.EXPECT().Get("test-user").Return(&v3.User{
					ObjectMeta: metav1.ObjectMeta{Name: "test-user"},
					PrincipalIDs: []string{
						"other-principal",
						"principal-test-user",
					},
				}, nil)
			},
			wantGRB: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
				},
				UserName:          "test-user",
				UserPrincipalName: "principal-test-user",
			},
			wantErr:           false,
			wantConditionType: subjectExists,
		},
		{
			name: "user name set but user principal name empty - fails to get user",
			grb: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
				},
				UserName: "test-user",
			},
			setupUserLister: func(m *fake.MockNonNamespacedCacheInterface[*v3.User]) {
				m.EXPECT().Get("test-user").Return(nil, fmt.Errorf("user not found"))
			},
			wantGRB: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
				},
				UserName: "test-user",
			},
			wantErr:           true,
			wantConditionType: failedToGetUser,
		},
		{
			name: "no subject specified - returns error",
			grb: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
				},
			},
			wantGRB: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-grb",
				},
			},
			wantErr:           true,
			wantConditionType: grbHasNoSubject,
		},
	}

	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			userManager := userMocks.NewMockManager(ctrl)
			if tt.setupUserManager != nil {
				tt.setupUserManager(userManager)
			}

			userLister := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
			if tt.setupUserLister != nil {
				tt.setupUserLister(userLister)
			}

			lifecycle := &globalRoleBindingLifecycle{
				userManager: userManager,
				userLister:  userLister,
				status:      status.NewStatus(),
			}

			var conditions []metav1.Condition
			got, err := lifecycle.reconcileSubject(tt.grb, &conditions)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tt.wantGRB, got)

			// Verify condition was set correctly
			require.Len(t, conditions, 1)
			require.Equal(t, subjectReconciled, conditions[0].Type)
			require.Equal(t, tt.wantConditionType, conditions[0].Reason)

			if tt.wantErr {
				require.Equal(t, metav1.ConditionFalse, conditions[0].Status)
			} else {
				require.Equal(t, metav1.ConditionTrue, conditions[0].Status)
			}
		})
	}
}

func Test_globalRoleBindingLifecycle_Create(t *testing.T) {
	t.Parallel()

	t.Run("successfully creates all resources and calls all reconciliation functions", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		testUser := "test-user"
		testPrincipal := "local://test-user"
		testGRBName := "test-grb"
		testGRName := "test-gr"
		notLocalCluster := v3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "not-local",
			},
		}
		localCluster := v3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: localClusterName,
			},
		}

		grb := &v3.GlobalRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: testGRBName,
				UID:  "test-uid",
			},
			TypeMeta: metav1.TypeMeta{
				APIVersion: "management.cattle.io/v3",
				Kind:       "GlobalRoleBinding",
			},
			UserName:       testUser,
			GlobalRoleName: testGRName,
		}

		gr := &v3.GlobalRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: testGRName,
			},
			InheritedClusterRoles: []string{"read-only"},
			NamespacedRules: map[string][]rbacv1.PolicyRule{
				"default": {{
					Verbs:     []string{"get"},
					APIGroups: []string{""},
					Resources: []string{"pods"},
				}},
			},
		}

		// Setup all mocks
		// reconcileSubject: Look up the user to populate the UserPrincipalName field from the User's PrincipalIDs
		userLister := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		userLister.EXPECT().Get(testUser).Return(&v3.User{
			ObjectMeta:   metav1.ObjectMeta{Name: testUser},
			PrincipalIDs: []string{testPrincipal},
		}, nil)

		// reconcileClusterPermissions: List all clusters to determine which need ClusterRoleTemplateBindings (CRTBs)
		clusterCache := fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl)
		clusterCache.EXPECT().List(labels.Everything()).Return([]*v3.Cluster{&notLocalCluster, &localCluster}, nil).AnyTimes()

		// reconcileClusterPermissions & reconcileNamespacedRoleBindings & reconcileGlobalRoleBinding:
		// Retrieve the GlobalRole to determine what permissions to grant
		grLister := fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](ctrl)
		grLister.EXPECT().Get(testGRName).Return(gr, nil).AnyTimes()

		// reconcileClusterPermissions: Check for existing CRTBs in each cluster that are owned by this GRB
		// to determine what needs to be created/updated/deleted
		crtbCache := fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
		crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "local/"+testGRBName).Return([]*v3.ClusterRoleTemplateBinding{}, nil)
		crtbCache.EXPECT().GetByIndex(crtbGrbOwnerIndex, "not-local/"+testGRBName).Return([]*v3.ClusterRoleTemplateBinding{}, nil).Times(2)

		// reconcileClusterPermissions: Create a CRTB for each InheritedClusterRole in the GlobalRole
		// This grants the user cluster-level permissions on downstream clusters
		crtbClient := fake.NewMockClientInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
		crtbClient.EXPECT().Create(gomock.Any()).Return(&v3.ClusterRoleTemplateBinding{}, nil)

		// reconcileGlobalRoleBinding: Check if a ClusterRoleBinding already exists for this GRB
		crbLister := fake.NewMockNonNamespacedCacheInterface[*rbacv1.ClusterRoleBinding](ctrl)
		crbLister.EXPECT().Get(gomock.Any()).Return(nil, apierrors.NewNotFound(rbacv1.Resource("clusterrolebinding"), "test"))

		// reconcileGlobalRoleBinding: Create a ClusterRoleBinding to bind the user to the GlobalRole's ClusterRole
		// This grants the user global-level permissions (e.g., cluster management, global resource access)
		crbClient := fake.NewMockNonNamespacedClientInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList](ctrl)
		crbClient.EXPECT().Create(gomock.Any()).Return(&rbacv1.ClusterRoleBinding{}, nil)

		// reconcileNamespacedRoleBindings: Look up namespaces referenced in the GlobalRole's NamespacedRules
		// to verify they exist before creating RoleBindings
		nsCache := fake.NewMockNonNamespacedCacheInterface[*corev1.Namespace](ctrl)
		nsCache.EXPECT().Get("default").Return(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "default"},
			Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
		}, nil)

		// reconcileNamespacedRoleBindings: Check if RoleBindings already exist in each namespace
		rbLister := fake.NewMockCacheInterface[*rbacv1.RoleBinding](ctrl)
		rbLister.EXPECT().Get("default", gomock.Any()).Return(nil, apierrors.NewNotFound(rbacv1.Resource("rolebinding"), "test"))
		rbLister.EXPECT().List("", gomock.Any()).Return([]*rbacv1.RoleBinding{}, nil)

		// reconcileNamespacedRoleBindings: Create RoleBindings for each namespace in the GlobalRole's NamespacedRules
		// This grants the user namespace-scoped permissions (e.g., access to pods in specific namespaces)
		rbClient := fake.NewMockClientInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](ctrl)
		rbClient.EXPECT().Create(gomock.Any()).Return(&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{UID: "rb-uid"},
		}, nil)

		// updateStatus: Retrieve the current GRB from the cluster to update its status
		grbLister := fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRoleBinding](ctrl)
		grbLister.EXPECT().Get(testGRBName).Return(grb, nil)

		// updateStatus: Update the GRB's status to reflect the success/failure of all reconciliation steps
		grbClient := fake.NewMockNonNamespacedControllerInterface[*v3.GlobalRoleBinding, *v3.GlobalRoleBindingList](ctrl)
		grbClient.EXPECT().UpdateStatus(gomock.Any()).DoAndReturn(func(obj *v3.GlobalRoleBinding) (*v3.GlobalRoleBinding, error) {
			// Verify that status was updated with all conditions
			require.Equal(t, status.SummaryCompleted, obj.Status.Summary)
			require.Equal(t, status.SummaryCompleted, obj.Status.SummaryLocal)
			require.Len(t, obj.Status.LocalConditions, 5)

			// Verify all conditions are present and successful
			conditionTypes := make(map[string]metav1.ConditionStatus)
			for _, cond := range obj.Status.LocalConditions {
				conditionTypes[cond.Type] = cond.Status
			}

			require.Equal(t, metav1.ConditionTrue, conditionTypes[subjectReconciled])
			require.Equal(t, metav1.ConditionTrue, conditionTypes[clusterPermissionsReconciled])
			require.Equal(t, metav1.ConditionTrue, conditionTypes[globalRoleBindingReconciled])
			require.Equal(t, metav1.ConditionTrue, conditionTypes[namespacedRoleBindingReconciled])
			require.Equal(t, metav1.ConditionTrue, conditionTypes["FleetWorkspacePermissionsReconciled"])

			return obj, nil
		})

		fleetHandlerCalled := false
		fwhMock := &fleetPermissionsHandlerMock{
			reconcileFleetWorkspacePermissionsFunc: func(grb *v3.GlobalRoleBinding, conditions *[]metav1.Condition) error {
				fleetHandlerCalled = true
				// Add fleet condition
				*conditions = append(*conditions, metav1.Condition{
					Type:   "FleetWorkspacePermissionsReconciled",
					Status: metav1.ConditionTrue,
					Reason: "FleetWorkspacePermissionsReconciled",
				})
				return nil
			},
		}

		// Create lifecycle with all mocks
		lifecycle := &globalRoleBindingLifecycle{
			userLister:              userLister,
			clusterLister:           clusterCache,
			grLister:                grLister,
			crtbCache:               crtbCache,
			crtbClient:              crtbClient,
			crbLister:               crbLister,
			crbClient:               crbClient,
			nsCache:                 nsCache,
			roleBindingLister:       rbLister,
			roleBindings:            rbClient,
			grbLister:               grbLister,
			grbClient:               grbClient,
			fleetPermissionsHandler: fwhMock,
			status:                  status.NewStatus(),
		}

		// Execute Create
		result, err := lifecycle.Create(grb)

		// Verify results
		require.NoError(t, err)
		require.NotNil(t, result)
		require.True(t, fleetHandlerCalled, "fleet permissions handler should have been called")

		resultGRB, ok := result.(*v3.GlobalRoleBinding)
		require.True(t, ok)
		require.Equal(t, testPrincipal, resultGRB.UserPrincipalName, "user principal should be set by reconcileSubject")
		require.Equal(t, crbNamePrefix+testGRBName, resultGRB.Annotations[crbNameAnnotation], "CRB annotation should be set by reconcileGlobalRoleBinding")
	})
}
