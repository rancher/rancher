package rbac

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/apis/management.cattle.io"
	apisV3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/managementuser/resourcequota"
	coreFakes "github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	v3fakes "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	wfakes "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

func TestReconcileNamespaceProjectClusterRole(t *testing.T) {
	const namespaceName = "test-ns"
	ctrl := gomock.NewController(t)
	tests := []struct {
		name                string
		indexedRoles        []*rbacv1.ClusterRole
		currentRoles        []*rbacv1.ClusterRole
		projectNSAnnotation string
		indexerError        error
		updateError         error
		createError         error
		deleteError         error
		getError            error

		wantRoles           []*rbacv1.ClusterRole
		wantDeleteRoleNames []string
		wantError           bool
	}{
		{
			name:                "create read-only",
			projectNSAnnotation: "c-123xyz:p-123xyz",
			indexedRoles: []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", namespaceName, "*"),
			},
			wantRoles: []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", namespaceName, "get"),
			},
			wantError: false,
		},
		{
			name:                "update old create new read-only",
			projectNSAnnotation: "c-123xyz:p-123xyz",
			indexedRoles: []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", namespaceName, "*"),
				addNamespaceToClusterRole("otherNamespace", "get", createClusterRoleForProject("p-123abc", namespaceName, "get")),
			},
			wantRoles: []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", namespaceName, "get"),
				createClusterRoleForProject("p-123abc", "otherNamespace", "get"),
			},
			wantError: false,
		},
		{
			name:                "delete old create new read-only",
			projectNSAnnotation: "c-123xyz:p-123xyz",
			indexedRoles: []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", namespaceName, "*"),
				createClusterRoleForProject("p-123abc", namespaceName, "get"),
			},
			wantRoles: []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", namespaceName, "get"),
			},
			wantDeleteRoleNames: []string{createRoleName("p-123abc", "get")},
			wantError:           false,
		},
		{
			name:                "delete old update new read-only",
			projectNSAnnotation: "c-123xyz:p-123xyz",
			indexedRoles: []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", namespaceName, "*"),
				createClusterRoleForProject("p-123abc", namespaceName, "get"),
			},
			currentRoles: []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", "otherNamespace", "get"),
			},
			wantRoles: []*rbacv1.ClusterRole{
				addNamespaceToClusterRole(namespaceName, "get", createClusterRoleForProject("p-123xyz", "otherNamespace", "get")),
			},
			wantDeleteRoleNames: []string{createRoleName("p-123abc", "get")},
			wantError:           false,
		},
		{
			name:                "create edit",
			projectNSAnnotation: "c-123xyz:p-123xyz",
			indexedRoles: []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", namespaceName, "get"),
			},
			wantRoles: []*rbacv1.ClusterRole{
				addManagePermissionToClusterRole("p-123xyz", createClusterRoleForProject("p-123xyz", namespaceName, "*")),
			},
			wantError: false,
		},
		{
			name:                "update old create new edit",
			projectNSAnnotation: "c-123xyz:p-123xyz",
			indexedRoles: []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", namespaceName, "get"),
				addManagePermissionToClusterRole("p-123abc", addNamespaceToClusterRole("otherNamespace", "*", createClusterRoleForProject("p-123abc", namespaceName, "*"))),
			},
			wantRoles: []*rbacv1.ClusterRole{
				addManagePermissionToClusterRole("p-123xyz", createClusterRoleForProject("p-123xyz", namespaceName, "*")),
				addManagePermissionToClusterRole("p-123abc", createClusterRoleForProject("p-123abc", "otherNamespace", "*")),
			},
			wantError: false,
		},
		{
			name:                "update old update new edit",
			projectNSAnnotation: "c-123xyz:p-123xyz",
			indexedRoles: []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", namespaceName, "get"),
				addManagePermissionToClusterRole("p-123abc", createClusterRoleForProject("p-123abc", namespaceName, "*")),
			},
			currentRoles: []*rbacv1.ClusterRole{
				addManagePermissionToClusterRole("p-123xyz", createClusterRoleForProject("p-123xyz", "otherNamespace", "*")),
			},
			wantRoles: []*rbacv1.ClusterRole{
				addManagePermissionToClusterRole("p-123xyz", addNamespaceToClusterRole(namespaceName, "*", createClusterRoleForProject("p-123xyz", "otherNamespace", "*"))),
				addManagePermissionToClusterRole("p-123abc", createBaseClusterRoleForProject("p-123abc", "*")),
			},
			wantError: false,
		},
		{
			name:                "indexer error",
			projectNSAnnotation: "c-123xyz:p-123xyz",
			indexerError:        fmt.Errorf("unable to read from indexer"),
			wantError:           true,
		},
		{
			name:                "update error",
			projectNSAnnotation: "c-123xyz:p-123xyz",
			indexedRoles: []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", namespaceName, "*"),
				addNamespaceToClusterRole("otherNamespace", "get", createClusterRoleForProject("p-123abc", namespaceName, "get")),
			},
			updateError: fmt.Errorf("unable to update"),
			wantError:   true,
		},
		{
			name:                "create error",
			projectNSAnnotation: "c-123xyz:p-123xyz",
			createError:         fmt.Errorf("unable to create"),
			wantError:           true,
		},
		{
			name:                "delete error",
			projectNSAnnotation: "c-123xyz:p-123xyz",
			indexedRoles: []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", namespaceName, "*"),
				createClusterRoleForProject("p-123abc", namespaceName, "get"),
			},
			deleteError: fmt.Errorf("unable to delete"),
			wantError:   true,
		},
		{
			name:                "get error",
			projectNSAnnotation: "c-123xyz:p-123xyz",
			getError:            fmt.Errorf("unable to get"),
			wantError:           true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var newRoles []*rbacv1.ClusterRole
			var deletedRoleNames []string
			indexer := FakeResourceIndexer[*rbacv1.ClusterRole]{
				index: crByNSIndex,
				resources: map[string][]*rbacv1.ClusterRole{
					namespaceName: test.indexedRoles,
				},
				err: test.indexerError,
			}
			fakeLister := wfakes.NewMockNonNamespacedCacheInterface[*v1.ClusterRole](ctrl)
			fakeLister.EXPECT().Get(gomock.Any()).DoAndReturn(
				func(in1 string) (*v1.ClusterRole, error) {
					if test.getError != nil {
						return nil, test.getError
					}
					for _, role := range test.currentRoles {
						if role.Name == in1 {
							return role, nil
						}
					}
					return nil, apierror.NewNotFound(schema.GroupResource{
						Group:    "rbac.authorization.k8s.io",
						Resource: "ClusterRoles",
					}, in1)
				},
			).AnyTimes()
			fakeClusterRoles := wfakes.NewMockNonNamespacedControllerInterface[*v1.ClusterRole, *v1.ClusterRoleList](ctrl)
			fakeClusterRoles.EXPECT().Create(gomock.Any()).DoAndReturn(
				func(in *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
					newRoles = append(newRoles, in)
					if test.createError != nil {
						return nil, test.createError
					}
					return in, nil
				},
			).AnyTimes()
			fakeClusterRoles.EXPECT().Update(gomock.Any()).DoAndReturn(
				func(in *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
					newRoles = append(newRoles, in)
					if test.updateError != nil {
						return nil, test.updateError
					}
					return in, nil
				},
			).AnyTimes()
			fakeClusterRoles.EXPECT().Delete(gomock.Any(), gomock.Any()).DoAndReturn(
				func(name string, options *metav1.DeleteOptions) error {
					deletedRoleNames = append(deletedRoleNames, name)
					if test.deleteError != nil {
						return test.deleteError
					}
					return nil
				},
			).AnyTimes()
			lifecycle := nsLifecycle{
				m: &manager{
					crLister:     fakeLister,
					crIndexer:    &indexer,
					clusterRoles: fakeClusterRoles,
				},
			}
			err := lifecycle.reconcileNamespaceProjectClusterRole(&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName,
					Annotations: map[string]string{
						projectIDAnnotation: test.projectNSAnnotation,
					},
				},
			})
			if test.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, newRoles, len(test.wantRoles))
				for _, role := range test.wantRoles {
					assert.Contains(t, newRoles, role)
				}
				assert.Len(t, deletedRoleNames, len(test.wantDeleteRoleNames))
				for _, roleName := range test.wantDeleteRoleNames {
					assert.Contains(t, deletedRoleNames, roleName)
				}
			}
		})
	}

}

func TestCreateProjectNSRole(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	type testCase struct {
		description string
		verb        string
		namespace   string
		projectName string
		crSetup     func()
		startingCR  *v1.ClusterRole
		expectedCR  *v1.ClusterRole
		createError error
		expectedErr string
	}
	testCases := []testCase{
		{
			description: "create get role",
			verb:        "get",
			projectName: "p-123xyz",
			expectedCR: &v1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "p-123xyz-namespaces-readonly",
					Annotations: map[string]string{
						projectNSAnn: "p-123xyz-namespaces-readonly",
					},
				},
			},
		},
		{
			description: "create edit role",
			verb:        "*",
			projectName: "p-123xyz",
			expectedCR: &v1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "p-123xyz-namespaces-edit",
					Annotations: map[string]string{
						projectNSAnn: "p-123xyz-namespaces-edit",
					},
				},
				Rules: []v1.PolicyRule{
					{
						APIGroups:     []string{"management.cattle.io"},
						Verbs:         []string{"manage-namespaces"},
						Resources:     []string{"projects"},
						ResourceNames: []string{"p-123xyz"},
					},
				},
			},
		},
		{
			description: "do not change role if already exists and return AlreadyExists error",
			verb:        "*",
			projectName: "p-123xyz",
			expectedCR: &v1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "p-123xyz-namespaces-edit",
					Annotations: map[string]string{
						projectNSAnn: "p-123xyz-namespaces-edit",
					},
				},
				Rules: []v1.PolicyRule{
					{
						APIGroups:     []string{"management.cattle.io"},
						Verbs:         []string{"manage-namespaces"},
						Resources:     []string{"projects"},
						ResourceNames: []string{"p-123xyz"},
					},
				},
			},
			startingCR: &v1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "p-123xyz-namespaces-edit",
					Annotations: map[string]string{
						projectNSAnn: "p-123xyz-namespaces-edit",
					},
				},
				Rules: []v1.PolicyRule{
					{
						APIGroups:     []string{"management.cattle.io"},
						Verbs:         []string{"manage-namespaces"},
						Resources:     []string{"projects"},
						ResourceNames: []string{"p-123xyz"},
					},
				},
			},
			expectedErr: `roletemplates.management.cattle.io "p-123xyz-namespaces-edit" already exists`,
		},
		{
			description: "test should return non-AlreadyExists error",
			verb:        "*",
			projectName: "p-123xyz",
			createError: errors.NewInternalError(fmt.Errorf("some error")),
			expectedErr: "Internal error occurred: some error",
		},
	}
	for _, test := range testCases {
		clusterRoles := map[string]*v1.ClusterRole{}
		if test.startingCR != nil {
			clusterRoles = map[string]*v1.ClusterRole{
				test.startingCR.Name: test.startingCR,
			}
		}

		m := newManager(withClusterRoles(clusterRoles, &clientErrs{createError: test.createError}, ctrl), func(m *manager) {
			clusterRoles := wfakes.NewMockNonNamespacedControllerInterface[*v1.ClusterRole, *v1.ClusterRoleList](ctrl)
			clusterRoles.EXPECT().Create(gomock.Any()).DoAndReturn(
				func(In1 *v1.ClusterRole) (*v1.ClusterRole, error) {
					if test.expectedErr != "" {
						return nil, fmt.Errorf("%v", test.expectedErr)
					}
					return In1, nil
				},
			).Times(1)
			m.clusterRoles = clusterRoles
		})

		roleName := fmt.Sprintf(projectNSGetClusterRoleNameFmt, test.projectName, projectNSVerbToSuffix[test.verb])
		err := m.createProjectNSRole(roleName, test.verb, test.namespace, test.projectName)

		if test.expectedErr != "" {
			assert.ErrorContains(t, err, test.expectedErr, test.description)
		} else {
			assert.NoError(t, err)
		}
	}
}

func createClusterRoleForProject(projectName string, namespace string, verb string) *rbacv1.ClusterRole {
	cr := createBaseClusterRoleForProject(projectName, verb)
	return addNamespaceToClusterRole(namespace, verb, cr)
}

func createBaseClusterRoleForProject(projectName string, verb string) *rbacv1.ClusterRole {
	roleName := createRoleName(projectName, verb)
	newCR := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
			Annotations: map[string]string{
				projectNSAnn: roleName,
			},
		},
	}
	return newCR.DeepCopy()
}

func addNamespaceToClusterRole(namespace string, verb string, clusterRole *rbacv1.ClusterRole) *rbacv1.ClusterRole {
	if clusterRole.Rules == nil {
		clusterRole.Rules = []rbacv1.PolicyRule{}
	}
	foundIdx := -1
	// aggregate to a single rule for all NS if we can
	for i, rule := range clusterRole.Rules {
		hasApiGroup := slices.Contains(rule.APIGroups, "")
		if !hasApiGroup {
			continue
		}

		hasNamespaces := slices.Contains(rule.Resources, "namespaces")
		if !hasNamespaces {
			continue
		}
		foundIdx = i
	}
	if foundIdx >= 0 {
		clusterRole.Rules[foundIdx].ResourceNames = append(clusterRole.Rules[foundIdx].ResourceNames, namespace)
		return clusterRole
	}
	rule := rbacv1.PolicyRule{
		APIGroups:     []string{""},
		Verbs:         []string{verb},
		Resources:     []string{"namespaces"},
		ResourceNames: []string{namespace},
	}
	clusterRole.Rules = append(clusterRole.Rules, rule)
	return clusterRole
}

func addManagePermissionToClusterRole(projectName string, clusterRole *rbacv1.ClusterRole) *rbacv1.ClusterRole {
	if clusterRole.Rules == nil {
		clusterRole.Rules = []rbacv1.PolicyRule{}
	}
	rule := rbacv1.PolicyRule{
		APIGroups:     []string{management.GroupName},
		Verbs:         []string{manageNSVerb},
		Resources:     []string{apisV3.ProjectResourceName},
		ResourceNames: []string{projectName},
	}
	clusterRole.Rules = append(clusterRole.Rules, rule)
	return clusterRole
}

func createRoleName(projectName string, verb string) string {
	return fmt.Sprintf(projectNSGetClusterRoleNameFmt, projectName, projectNSVerbToSuffix[verb])
}

type FakeResourceIndexer[T runtime.Object] struct {
	cache.Store
	resources map[string][]T
	err       error
	index     string
}

func (d *FakeResourceIndexer[T]) Index(indexName string, obj interface{}) ([]interface{}, error) {
	return nil, nil
}

func (d *FakeResourceIndexer[T]) IndexKeys(indexName, indexKey string) ([]string, error) {
	return []string{}, nil
}

func (d *FakeResourceIndexer[T]) ListIndexFuncValues(indexName string) []string {
	return []string{}
}

func (d *FakeResourceIndexer[T]) ByIndex(indexName, indexKey string) ([]interface{}, error) {
	if indexName != d.index {
		return nil, fmt.Errorf("dummy indexer only supports %s for index name, %s given", d.index, indexName)
	}
	if d.err != nil {
		return nil, d.err
	}

	resources := d.resources[indexKey]
	var interfaces []interface{}
	for _, resource := range resources {
		interfaces = append(interfaces, resource)
	}

	return interfaces, nil
}

func (d *FakeResourceIndexer[T]) GetIndexers() cache.Indexers {
	return nil
}

func (d *FakeResourceIndexer[T]) AddIndexers(newIndexers cache.Indexers) error {
	return nil
}

func TestAsyncCleanupRBAC_NamespaceDeleted(t *testing.T) {
	tests := []struct {
		name                  string
		indexedRoles          []*rbacv1.ClusterRole
		nsGetCalls            int
		nsGetTerminatingCalls int
	}{
		{
			name:                  "namespace already deleted",
			nsGetCalls:            1,
			nsGetTerminatingCalls: 0,
		},
		{
			name:                  "namespace still terminating need to wait",
			nsGetCalls:            2,
			nsGetTerminatingCalls: 1,
		},
	}

	namespaceName := "test-namespace"
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			timesCalled := 0
			nsTerminatingCount := 0

			namespaceListerMock := &coreFakes.NamespaceListerMock{
				GetFunc: func(namespace string, name string) (*corev1.Namespace, error) {
					timesCalled++
					if nsTerminatingCount < test.nsGetTerminatingCalls {
						nsTerminatingCount++
						return &corev1.Namespace{
							TypeMeta: metav1.TypeMeta{},
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-namespace",
							},
							Status: corev1.NamespaceStatus{
								Phase: corev1.NamespaceTerminating,
							},
						}, nil
					}
					// indicate deleted namespace
					return nil, apierrors.NewNotFound(schema.GroupResource{}, "")
				},
			}

			indexedRoles := []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", namespaceName, "*"),
			}
			indexer := FakeResourceIndexer[*rbacv1.ClusterRole]{
				index: crByNSIndex,
				resources: map[string][]*rbacv1.ClusterRole{
					namespaceName: indexedRoles,
				},
				err: nil,
			}

			fakeClusterRoles := wfakes.NewMockNonNamespacedControllerInterface[*v1.ClusterRole, *v1.ClusterRoleList](ctrl)
			fakeClusterRoles.EXPECT().Update(gomock.Any()).DoAndReturn(
				func(in *v1.ClusterRole) (*v1.ClusterRole, error) {
					return in, nil
				},
			).AnyTimes()
			fakeClusterRoles.EXPECT().Delete(gomock.Any(), gomock.Any()).DoAndReturn(
				func(name string, options *metav1.DeleteOptions) error {
					return nil
				},
			).AnyTimes()

			fakeLister := wfakes.NewMockNonNamespacedCacheInterface[*v1.ClusterRole](ctrl)
			fakeLister.EXPECT().Get(gomock.Any()).DoAndReturn(
				func(int1 string) (*v1.ClusterRole, error) {
					return &v1.ClusterRole{}, nil
				},
			).AnyTimes()
			nsLifecycle := &nsLifecycle{
				m: &manager{
					nsLister:     namespaceListerMock,
					crLister:     fakeLister,
					crIndexer:    &indexer,
					clusterRoles: fakeClusterRoles,
				},
			}

			nsLifecycle.asyncCleanupRBAC(namespaceName)

			waitForCondition(t, func() bool {
				return timesCalled == test.nsGetCalls
			}, 15*time.Second, time.Second)
		})
	}
}

func waitForCondition(t *testing.T, condition func() bool, timeout time.Duration, interval time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(interval)
	}
	t.Fatalf("condition not met within %v", timeout)
}

func TestEnsurePRTBAddToNamespace(t *testing.T) {
	const namespaceName = "test-ns"
	ctrl := gomock.NewController(t)
	tests := []struct {
		name                string
		indexedRoles        []*v1.ClusterRole
		currentRoles        []*v1.ClusterRole
		indexedPRTBs        []*v3.ProjectRoleTemplateBinding
		projectNSAnnotation string
		indexerError        error
		updateError         error
		createError         error
		deleteError         error
		getError            error

		wantError    string
		wantHasPRTBs bool
	}{
		{
			name:                "update namespace with missing namespace",
			projectNSAnnotation: "c-123xyz:p-123xyz",
			indexedRoles: []*v1.ClusterRole{
				createClusterRoleForProject("p-123xyz", namespaceName, "*"),
				addNamespaceToClusterRole("otherNamespace", "get", createClusterRoleForProject("p-123abc", namespaceName, "get")),
			},
			indexedPRTBs: []*v3.ProjectRoleTemplateBinding{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespaceName,
						Name:      "test-prtb",
					},
					UserName:         "test-user",
					ProjectName:      "test-cluster:test-project",
					RoleTemplateName: "test-rt",
				},
			},
			wantHasPRTBs: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			crIndexer := &FakeResourceIndexer[*v1.ClusterRole]{
				resources: map[string][]*v1.ClusterRole{
					namespaceName: test.indexedRoles,
				},
				index: crByNSIndex,
				err:   test.indexerError,
			}
			prtbIndexer := &FakeResourceIndexer[*v3.ProjectRoleTemplateBinding]{
				resources: map[string][]*v3.ProjectRoleTemplateBinding{
					"c-123xyz:p-123xyz": test.indexedPRTBs,
				},
				err:   test.indexerError,
				index: prtbByProjectIndex,
			}

			rtLister := wfakes.NewMockNonNamespacedCacheInterface[*v3.RoleTemplate](ctrl)
			rtLister.EXPECT().Get(gomock.Any()).DoAndReturn(
				func(name string) (*v3.RoleTemplate, error) {
					return nil, apierrors.NewNotFound(schema.GroupResource{
						Group:    "management.cattle.io",
						Resource: "roletemplates",
					}, name)
				},
			)
			lifecycle := nsLifecycle{
				m: &manager{
					crIndexer:   crIndexer,
					prtbIndexer: prtbIndexer,
					rtLister:    rtLister,
				},
				rq: &resourcequota.SyncController{
					ProjectLister: &v3fakes.ProjectListerMock{
						GetFunc: func(namespace string, name string) (*v3.Project, error) {
							return nil, apierrors.NewNotFound(schema.GroupResource{
								Group:    "management.cattle.io",
								Resource: "projects",
							}, name)
						},
					},
				},
			}
			hasPRTBs, err := lifecycle.ensurePRTBAddToNamespace(&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName,
					Annotations: map[string]string{
						projectIDAnnotation: test.projectNSAnnotation,
					},
				},
			})
			assert.Equal(t, test.wantHasPRTBs, hasPRTBs)
			if test.wantError != "" {
				assert.ErrorContains(t, err, test.wantError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
