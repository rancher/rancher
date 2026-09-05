package rbac

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/apis/management.cattle.io"
	apisV3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/managementuser/resourcequota"
	"github.com/rancher/rancher/pkg/features"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	namespaceutil "github.com/rancher/rancher/pkg/namespace"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	wfakes "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
				createClusterRoleForProject("p-123xyz", namespaceName, editVerb),
				createClusterRoleForProject("p-123xyz", namespaceName, deleteVerb),
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
				createClusterRoleForProject("p-123xyz", namespaceName, editVerb),
				createClusterRoleForProject("p-123xyz", namespaceName, deleteVerb),
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
				createClusterRoleForProject("p-123xyz", namespaceName, editVerb),
				createClusterRoleForProject("p-123xyz", namespaceName, deleteVerb),
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
				createClusterRoleForProject("p-123xyz", namespaceName, editVerb),
				createClusterRoleForProject("p-123xyz", namespaceName, deleteVerb),
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
				createClusterRoleForProject("p-123xyz", namespaceName, deleteVerb),
			},
			wantRoles: []*rbacv1.ClusterRole{
				addManagePermissionToClusterRole("p-123xyz", createClusterRoleForProject("p-123xyz", namespaceName, editVerb)),
			},
			wantError: false,
		},
		{
			name:                "update old create new edit",
			projectNSAnnotation: "c-123xyz:p-123xyz",
			indexedRoles: []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", namespaceName, "get"),
				createClusterRoleForProject("p-123xyz", namespaceName, deleteVerb),
				addManagePermissionToClusterRole("p-123abc", addNamespaceToClusterRole("otherNamespace", editVerb, createClusterRoleForProject("p-123abc", namespaceName, editVerb))),
			},
			wantRoles: []*rbacv1.ClusterRole{
				addManagePermissionToClusterRole("p-123xyz", createClusterRoleForProject("p-123xyz", namespaceName, editVerb)),
				addManagePermissionToClusterRole("p-123abc", createClusterRoleForProject("p-123abc", "otherNamespace", editVerb)),
			},
			wantError: false,
		},
		{
			name:                "update old update new edit",
			projectNSAnnotation: "c-123xyz:p-123xyz",
			indexedRoles: []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", namespaceName, "get"),
				createClusterRoleForProject("p-123xyz", namespaceName, deleteVerb),
				addManagePermissionToClusterRole("p-123abc", createClusterRoleForProject("p-123abc", namespaceName, editVerb)),
			},
			currentRoles: []*rbacv1.ClusterRole{
				addManagePermissionToClusterRole("p-123xyz", createClusterRoleForProject("p-123xyz", "otherNamespace", editVerb)),
			},
			wantRoles: []*rbacv1.ClusterRole{
				addManagePermissionToClusterRole("p-123xyz", addNamespaceToClusterRole(namespaceName, editVerb, createClusterRoleForProject("p-123xyz", "otherNamespace", editVerb))),
				addManagePermissionToClusterRole("p-123abc", createBaseClusterRoleForProject("p-123abc", editVerb)),
			},
			wantError: false,
		},
		{
			name:                "create delete",
			projectNSAnnotation: "c-123xyz:p-123xyz",
			indexedRoles: []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", namespaceName, "get"),
				createClusterRoleForProject("p-123xyz", namespaceName, editVerb),
			},
			wantRoles: []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", namespaceName, deleteVerb),
			},
			wantError: false,
		},
		{
			name:                "update old create new delete",
			projectNSAnnotation: "c-123xyz:p-123xyz",
			indexedRoles: []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", namespaceName, "get"),
				createClusterRoleForProject("p-123xyz", namespaceName, editVerb),
				addNamespaceToClusterRole("otherNamespace", deleteVerb, createClusterRoleForProject("p-123abc", namespaceName, deleteVerb)),
			},
			wantRoles: []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", namespaceName, deleteVerb),
				createClusterRoleForProject("p-123abc", "otherNamespace", deleteVerb),
			},
			wantError: false,
		},
		{
			name:                "update old update new delete",
			projectNSAnnotation: "c-123xyz:p-123xyz",
			indexedRoles: []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", namespaceName, "get"),
				createClusterRoleForProject("p-123xyz", namespaceName, editVerb),
				createClusterRoleForProject("p-123abc", namespaceName, deleteVerb),
			},
			currentRoles: []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", "otherNamespace", deleteVerb),
			},
			wantRoles: []*rbacv1.ClusterRole{
				addNamespaceToClusterRole(namespaceName, deleteVerb, createClusterRoleForProject("p-123xyz", "otherNamespace", deleteVerb)),
			},
			// unlike edit roles, delete roles never carry a bundled manage-namespaces rule to keep them
			// alive, so removing the only namespace from a stale delete role deletes it outright.
			wantDeleteRoleNames: []string{createRoleName("p-123abc", deleteVerb)},
			wantError:           false,
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
				createClusterRoleForProject("p-123xyz", namespaceName, editVerb),
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
				createClusterRoleForProject("p-123xyz", namespaceName, editVerb),
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
		test := test
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
			fakeLister := wfakes.NewMockNonNamespacedCacheInterface[*rbacv1.ClusterRole](ctrl)
			fakeLister.EXPECT().Get(gomock.Any()).DoAndReturn(
				func(in1 string) (*rbacv1.ClusterRole, error) {
					if test.getError != nil {
						return nil, test.getError
					}
					for _, role := range test.currentRoles {
						if role.Name == in1 {
							return role, nil
						}
					}
					return nil, apierrors.NewNotFound(schema.GroupResource{
						Group:    "rbac.authorization.k8s.io",
						Resource: "ClusterRoles",
					}, in1)
				},
			).AnyTimes()
			fakeClusterRoles := wfakes.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl)
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
		startingCR  *rbacv1.ClusterRole
		expectedCR  *rbacv1.ClusterRole
		createError error
		expectedErr string
	}
	testCases := []testCase{
		{
			description: "create get role",
			verb:        "get",
			projectName: "p-123xyz",
			expectedCR: &rbacv1.ClusterRole{
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
			verb:        editVerb,
			projectName: "p-123xyz",
			expectedCR: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "p-123xyz-namespaces-edit",
					Annotations: map[string]string{
						projectNSAnn: "p-123xyz-namespaces-edit",
					},
				},
				Rules: []rbacv1.PolicyRule{
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
			description: "create delete role",
			verb:        deleteVerb,
			projectName: "p-123xyz",
			expectedCR: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "p-123xyz-namespaces-delete",
					Annotations: map[string]string{
						projectNSAnn: "p-123xyz-namespaces-delete",
					},
				},
			},
		},
		{
			description: "do not change role if already exists and return AlreadyExists error",
			verb:        editVerb,
			projectName: "p-123xyz",
			expectedCR: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "p-123xyz-namespaces-edit",
					Annotations: map[string]string{
						projectNSAnn: "p-123xyz-namespaces-edit",
					},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups:     []string{"management.cattle.io"},
						Verbs:         []string{"manage-namespaces"},
						Resources:     []string{"projects"},
						ResourceNames: []string{"p-123xyz"},
					},
				},
			},
			startingCR: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "p-123xyz-namespaces-edit",
					Annotations: map[string]string{
						projectNSAnn: "p-123xyz-namespaces-edit",
					},
				},
				Rules: []rbacv1.PolicyRule{
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
			verb:        editVerb,
			projectName: "p-123xyz",
			createError: apierrors.NewInternalError(fmt.Errorf("some error")),
			expectedErr: "Internal error occurred: some error",
		},
	}
	for _, test := range testCases {
		clusterRoles := map[string]*rbacv1.ClusterRole{}
		if test.startingCR != nil {
			clusterRoles = map[string]*rbacv1.ClusterRole{
				test.startingCR.Name: test.startingCR,
			}
		}

		m := newManager(withClusterRoles(clusterRoles, &clientErrs{createError: test.createError}, ctrl), func(m *manager) {
			clusterRoles := wfakes.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl)
			clusterRoles.EXPECT().Create(gomock.Any()).DoAndReturn(
				func(In1 *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
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
		hasApiGroup := false
		for _, apiGroup := range rule.APIGroups {
			if apiGroup == "" {
				hasApiGroup = true
				break
			}
		}
		if !hasApiGroup {
			continue
		}

		hasNamespaces := false
		for _, resource := range rule.Resources {
			if resource == "namespaces" {
				hasNamespaces = true
				break
			}
		}
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
		Verbs:         verbsFor(verb),
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
		test := test
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			timesCalled := 0
			nsTerminatingCount := 0

			namespaceListerMock := wfakes.NewMockNonNamespacedCacheInterface[*corev1.Namespace](ctrl)
			namespaceListerMock.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*corev1.Namespace, error) {
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
			}).AnyTimes()

			indexedRoles := []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", namespaceName, editVerb),
			}
			indexer := FakeResourceIndexer[*rbacv1.ClusterRole]{
				index: crByNSIndex,
				resources: map[string][]*rbacv1.ClusterRole{
					namespaceName: indexedRoles,
				},
				err: nil,
			}

			fakeClusterRoles := wfakes.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl)
			fakeClusterRoles.EXPECT().Update(gomock.Any()).DoAndReturn(
				func(in *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
					return in, nil
				},
			).AnyTimes()
			fakeClusterRoles.EXPECT().Delete(gomock.Any(), gomock.Any()).DoAndReturn(
				func(name string, options *metav1.DeleteOptions) error {
					return nil
				},
			).AnyTimes()

			fakeLister := wfakes.NewMockNonNamespacedCacheInterface[*rbacv1.ClusterRole](ctrl)
			fakeLister.EXPECT().Get(gomock.Any()).DoAndReturn(
				func(int1 string) (*rbacv1.ClusterRole, error) {
					return &rbacv1.ClusterRole{}, nil
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
		indexedRoles        []*rbacv1.ClusterRole
		currentRoles        []*rbacv1.ClusterRole
		indexedPRTBs        []*v3.ProjectRoleTemplateBinding
		projectNSAnnotation string
		aggregationEnabled  bool
		indexerError        error
		updateError         error
		createError         error
		deleteError         error
		getError            error

		wantError string
	}{
		{
			name:                "update namespace with missing namespace",
			projectNSAnnotation: "c-123xyz:p-123xyz",
			indexedRoles: []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", namespaceName, editVerb),
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
		},
		{
			name:                "aggregation enabled skips legacy binding creation",
			projectNSAnnotation: "c-123xyz:p-123xyz",
			aggregationEnabled:  true,
			indexedRoles: []*rbacv1.ClusterRole{
				createClusterRoleForProject("p-123xyz", namespaceName, editVerb),
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
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			prev := features.AggregatedRoleTemplates.Enabled()
			features.AggregatedRoleTemplates.Set(test.aggregationEnabled)
			t.Cleanup(func() { features.AggregatedRoleTemplates.Set(prev) })

			crIndexer := &FakeResourceIndexer[*rbacv1.ClusterRole]{
				resources: map[string][]*rbacv1.ClusterRole{
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
			// When aggregation is enabled, the legacy binding creation loop is skipped,
			// so the RoleTemplate lister is never consulted.
			if !test.aggregationEnabled {
				rtLister.EXPECT().Get(gomock.Any()).DoAndReturn(
					func(name string) (*v3.RoleTemplate, error) {
						return nil, apierrors.NewNotFound(schema.GroupResource{
							Group:    "management.cattle.io",
							Resource: "roletemplates",
						}, name)
					},
				)
			}
			pGetter := wfakes.NewMockCacheInterface[*apisV3.Project](ctrl)
			pGetter.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(
				func(namespace string, name string) (*v3.Project, error) {
					return nil, apierrors.NewNotFound(schema.GroupResource{
						Group:    "management.cattle.io",
						Resource: "projects",
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
					ProjectCache: pGetter,
				},
			}
			err := lifecycle.ensurePRTBAddToNamespace(&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName,
					Annotations: map[string]string{
						projectIDAnnotation: test.projectNSAnnotation,
					},
				},
			})
			if test.wantError != "" {
				assert.ErrorContains(t, err, test.wantError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// aggregationRoleBinding builds a RoleBinding carrying the aggregation feature label plus the given
// owner label, mirroring what the roletemplate-aggregation PRTB handler creates in each namespace.
func aggregationRoleBinding(name, namespace, ownerLabel string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				pkgrbac.AggregationFeatureLabel: "true",
				ownerLabel:                      "true",
			},
		},
	}
}

// legacyRoleBinding builds a RoleBinding carrying a single legacy rtb-owner label (key/value),
// mirroring what the legacy PRTB handler creates in each namespace.
func legacyRoleBinding(name, namespace, label, value string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{label: value},
		},
	}
}

// prtb builds a minimal PRTB; only its Namespace matters for legacy owner resolution.
func prtb(namespace, name string) *v3.ProjectRoleTemplateBinding {
	return &v3.ProjectRoleTemplateBinding{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
	}
}

// multiIndexPRTBIndexer is a cache.Indexer that resolves PRTBs by (indexName, indexKey), supporting
// the multiple indexes the unified cleanup consults (prtbByUIDIndex, prtbByNsAndNameIndex). Only
// ByIndex is implemented; the other cache.Indexer methods are never called by the code under test.
type multiIndexPRTBIndexer struct {
	cache.Indexer
	byIndex map[string]map[string][]*v3.ProjectRoleTemplateBinding
	err     error
}

func (m *multiIndexPRTBIndexer) ByIndex(indexName, indexKey string) ([]interface{}, error) {
	if m.err != nil {
		return nil, m.err
	}
	var out []interface{}
	for _, p := range m.byIndex[indexName][indexKey] {
		out = append(out, p)
	}
	return out, nil
}

func TestRemovePRTBRoleBindingsNotInProject(t *testing.T) {
	const (
		namespaceName    = "test-ns"
		projectID        = "c-123xyz:p-current"
		backingNamespace = "p-current"
		otherNamespace   = "p-other"
	)
	currentPRTBLabel := pkgrbac.GetPRTBOwnerLabel("current-prtb")
	otherPRTBLabel := pkgrbac.GetPRTBOwnerLabel("other-prtb")
	crtbLabel := pkgrbac.GetCRTBOwnerLabel("some-crtb")

	currentProject := &apisV3.Project{
		Status: apisV3.ProjectStatus{BackingNamespace: backingNamespace},
	}

	tests := []struct {
		name          string
		projectID     string
		project       *apisV3.Project
		projectGetErr error
		currentPRTBs  []*v3.ProjectRoleTemplateBinding
		existingRBs   []*rbacv1.RoleBinding
		prtbsByUID    map[string][]*v3.ProjectRoleTemplateBinding
		prtbsByNsName map[string][]*v3.ProjectRoleTemplateBinding
		indexerErr    error
		listError     error
		deleteError   error

		wantDeleted []string
		wantError   string
	}{
		{
			name:         "aggregation: stale binding from another project deleted, current kept",
			projectID:    projectID,
			project:      currentProject,
			currentPRTBs: []*v3.ProjectRoleTemplateBinding{prtb(backingNamespace, "current-prtb")},
			existingRBs: []*rbacv1.RoleBinding{
				aggregationRoleBinding("rb-keep", namespaceName, currentPRTBLabel),
				aggregationRoleBinding("rb-stale", namespaceName, otherPRTBLabel),
			},
			wantDeleted: []string{"rb-stale"},
		},
		{
			name:      "legacy: stale binding from another project deleted, current kept",
			projectID: projectID,
			project:   currentProject,
			existingRBs: []*rbacv1.RoleBinding{
				legacyRoleBinding("rb-legkeep", namespaceName, rtbOwnerLabelLegacy, "uid-current"),
				legacyRoleBinding("rb-legstale", namespaceName, rtbOwnerLabelLegacy, "uid-other"),
			},
			prtbsByUID: map[string][]*v3.ProjectRoleTemplateBinding{
				"uid-current": {prtb(backingNamespace, "cur")},
				"uid-other":   {prtb(otherNamespace, "oth")},
			},
			wantDeleted: []string{"rb-legstale"},
		},
		{
			name:      "legacy: rtb-owner-updated label resolved via ns-and-name index",
			projectID: projectID,
			project:   currentProject,
			existingRBs: []*rbacv1.RoleBinding{
				legacyRoleBinding("rb-updstale", namespaceName, rtbOwnerLabel, "p-other_oth"),
			},
			prtbsByNsName: map[string][]*v3.ProjectRoleTemplateBinding{
				"p-other_oth": {prtb(otherNamespace, "oth")},
			},
			wantDeleted: []string{"rb-updstale"},
		},
		{
			name:      "legacy: orphaned binding whose PRTB no longer exists is deleted",
			projectID: projectID,
			project:   currentProject,
			existingRBs: []*rbacv1.RoleBinding{
				legacyRoleBinding("rb-orphan", namespaceName, rtbOwnerLabelLegacy, "uid-gone"),
			},
			// uid-gone resolves to nothing: the owning PRTB is gone, so the binding is stale.
			wantDeleted: []string{"rb-orphan"},
		},
		{
			name:         "crtb-owned aggregation binding is left alone",
			projectID:    projectID,
			project:      currentProject,
			currentPRTBs: []*v3.ProjectRoleTemplateBinding{prtb(backingNamespace, "current-prtb")},
			existingRBs: []*rbacv1.RoleBinding{
				aggregationRoleBinding("rb-crtb", namespaceName, crtbLabel),
			},
			wantDeleted: nil,
		},
		{
			name:      "no project: every prtb-owned binding (legacy and aggregation) deleted",
			projectID: "",
			existingRBs: []*rbacv1.RoleBinding{
				aggregationRoleBinding("rb-agg", namespaceName, otherPRTBLabel),
				legacyRoleBinding("rb-leg", namespaceName, rtbOwnerLabelLegacy, "uid-any"),
			},
			prtbsByUID: map[string][]*v3.ProjectRoleTemplateBinding{
				"uid-any": {prtb(otherNamespace, "any")},
			},
			wantDeleted: []string{"rb-agg", "rb-leg"},
		},
		{
			name:          "project not found: cleanup skipped",
			projectID:     projectID,
			projectGetErr: apierrors.NewNotFound(schema.GroupResource{Group: "management.cattle.io", Resource: "projects"}, "p-current"),
			existingRBs: []*rbacv1.RoleBinding{
				aggregationRoleBinding("rb-stale", namespaceName, otherPRTBLabel),
			},
			wantDeleted: nil,
		},
		{
			name:      "malformed project id: cleanup skipped",
			projectID: "no-colon",
			existingRBs: []*rbacv1.RoleBinding{
				aggregationRoleBinding("rb-stale", namespaceName, otherPRTBLabel),
			},
			wantDeleted: nil,
		},
		{
			name:          "project get error is surfaced",
			projectID:     projectID,
			projectGetErr: fmt.Errorf("boom"),
			wantError:     "boom",
		},
		{
			name:         "list error is surfaced",
			projectID:    projectID,
			project:      currentProject,
			currentPRTBs: []*v3.ProjectRoleTemplateBinding{prtb(backingNamespace, "current-prtb")},
			listError:    fmt.Errorf("boom"),
			wantError:    "couldn't list role bindings",
		},
		{
			name:      "prtb lookup error is surfaced",
			projectID: projectID,
			project:   currentProject,
			existingRBs: []*rbacv1.RoleBinding{
				legacyRoleBinding("rb-leg", namespaceName, rtbOwnerLabelLegacy, "uid-any"),
			},
			indexerErr: fmt.Errorf("boom"),
			wantError:  "couldn't find prtb",
		},
		{
			name:         "delete error is surfaced",
			projectID:    projectID,
			project:      currentProject,
			currentPRTBs: []*v3.ProjectRoleTemplateBinding{prtb(backingNamespace, "current-prtb")},
			existingRBs: []*rbacv1.RoleBinding{
				aggregationRoleBinding("rb-stale", namespaceName, otherPRTBLabel),
			},
			wantDeleted: []string{"rb-stale"},
			deleteError: fmt.Errorf("boom"),
			wantError:   "failed to delete",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			wellFormed := false
			if test.projectID != "" {
				parts := strings.SplitN(test.projectID, ":", 2)
				wellFormed = len(parts) == 2 && parts[1] != ""
			}
			// The project is only resolved when a well-formed project id is set; cleanup only reaches
			// the RoleBinding list when there's no project or the project resolved successfully.
			reachedList := test.projectID == "" || (wellFormed && test.projectGetErr == nil)

			pGetter := wfakes.NewMockCacheInterface[*apisV3.Project](ctrl)
			if wellFormed {
				pGetter.EXPECT().Get(gomock.Any(), gomock.Any()).Return(test.project, test.projectGetErr)
			}

			rbLister := wfakes.NewMockCacheInterface[*rbacv1.RoleBinding](ctrl)
			if reachedList {
				rbLister.EXPECT().List(namespaceName, gomock.Any()).Return(test.existingRBs, test.listError)
			}

			roleBindings := wfakes.NewMockControllerInterface[*rbacv1.RoleBinding, *rbacv1.RoleBindingList](ctrl)
			for _, name := range test.wantDeleted {
				roleBindings.EXPECT().Delete(namespaceName, name, gomock.Any()).Return(test.deleteError)
			}

			prtbIndexer := &multiIndexPRTBIndexer{
				byIndex: map[string]map[string][]*v3.ProjectRoleTemplateBinding{
					prtbByUIDIndex:       test.prtbsByUID,
					prtbByNsAndNameIndex: test.prtbsByNsName,
				},
				err: test.indexerErr,
			}

			var prtbs []interface{}
			for _, p := range test.currentPRTBs {
				prtbs = append(prtbs, p)
			}

			lifecycle := nsLifecycle{
				m: &manager{
					rbLister:     rbLister,
					roleBindings: roleBindings,
					prtbIndexer:  prtbIndexer,
				},
				rq: &resourcequota.SyncController{
					ProjectCache: pGetter,
				},
			}

			err := lifecycle.removePRTBRoleBindingsNotInProject(namespaceName, test.projectID, prtbs)
			if test.wantError != "" {
				assert.ErrorContains(t, err, test.wantError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsPRTBOwnedRoleBinding(t *testing.T) {
	const namespaceName = "test-ns"
	prtbLabel := pkgrbac.GetPRTBOwnerLabel("some-prtb")
	crtbLabel := pkgrbac.GetCRTBOwnerLabel("some-crtb")

	tests := []struct {
		name string
		rb   *rbacv1.RoleBinding
		want bool
	}{
		{
			name: "aggregation prtb-owner labelled binding is owned",
			rb:   aggregationRoleBinding("rb", namespaceName, prtbLabel),
			want: true,
		},
		{
			name: "legacy rtb-owner labelled binding is owned",
			rb:   legacyRoleBinding("rb", namespaceName, rtbOwnerLabelLegacy, "uid-1"),
			want: true,
		},
		{
			name: "legacy rtb-owner-updated labelled binding is owned",
			rb:   legacyRoleBinding("rb", namespaceName, rtbOwnerLabel, "p-proj_name"),
			want: true,
		},
		{
			name: "crtb-owner labelled binding is not owned",
			rb:   aggregationRoleBinding("rb", namespaceName, crtbLabel),
			want: false,
		},
		{
			name: "unlabelled binding is not owned",
			rb:   &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "rb", Namespace: namespaceName}},
			want: false,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, isPRTBOwnedRoleBinding(test.rb))
		})
	}
}

func TestRoleBindingEnqueueNamespace(t *testing.T) {
	const namespaceName = "test-ns"
	prtbLabel := pkgrbac.GetPRTBOwnerLabel("some-prtb")

	t.Run("prtb-owned binding enqueues its namespace", func(t *testing.T) {
		keys, err := roleBindingEnqueueNamespace("", "", aggregationRoleBinding("rb", namespaceName, prtbLabel))
		assert.NoError(t, err)
		assert.Equal(t, []relatedresource.Key{{Name: namespaceName}}, keys)
	})

	t.Run("non-prtb binding enqueues nothing", func(t *testing.T) {
		keys, err := roleBindingEnqueueNamespace("", "", &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "rb", Namespace: namespaceName},
		})
		assert.NoError(t, err)
		assert.Empty(t, keys)
	})

	t.Run("non-rolebinding object enqueues nothing", func(t *testing.T) {
		keys, err := roleBindingEnqueueNamespace("", "", &rbacv1.ClusterRole{})
		assert.NoError(t, err)
		assert.Empty(t, keys)
	})
}

func TestClusterRoleEnqueueNamespace(t *testing.T) {
	const (
		namespaceName = "test-ns"
		projectName   = "p-123xyz"
	)

	t.Run("project namespace cluster role enqueues its namespaces", func(t *testing.T) {
		cr := createClusterRoleForProject(projectName, namespaceName, "get")
		keys, err := clusterRoleEnqueueNamespace("", "", cr)
		assert.NoError(t, err)
		assert.Contains(t, keys, relatedresource.Key{Name: namespaceName})
	})

	t.Run("unrelated cluster role enqueues nothing", func(t *testing.T) {
		keys, err := clusterRoleEnqueueNamespace("", "", &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "unrelated"},
		})
		assert.NoError(t, err)
		assert.Empty(t, keys)
	})
}

func TestInitialRolesReady(t *testing.T) {
	const (
		namespaceName = "test-ns"
		projectID     = "c-123xyz:p-123xyz"
		creator       = "creator-user"
	)
	twoRoles := []*rbacv1.ClusterRole{
		{ObjectMeta: metav1.ObjectMeta{Name: "p-123xyz-namespaces-readonly"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "p-123xyz-namespaces-edit"}},
	}

	tests := []struct {
		name          string
		annotations   map[string]string
		clusterRoles  []*rbacv1.ClusterRole
		creatorCRBs   []*rbacv1.ClusterRoleBinding
		prtbs         []*v3.ProjectRoleTemplateBinding
		bindings      []*rbacv1.RoleBinding
		expectListRBs bool
		want          bool
	}{
		{
			name:        "no project is always ready",
			annotations: map[string]string{},
			want:        true,
		},
		{
			name:         "fewer than two cluster roles is not ready",
			annotations:  map[string]string{projectIDAnnotation: projectID},
			clusterRoles: twoRoles[:1],
			want:         false,
		},
		{
			name:         "two cluster roles, no creator, no prtbs is ready",
			annotations:  map[string]string{projectIDAnnotation: projectID},
			clusterRoles: twoRoles,
			want:         true,
		},
		{
			name:         "creator without a binding is not ready",
			annotations:  map[string]string{projectIDAnnotation: projectID, "field.cattle.io/creatorId": creator},
			clusterRoles: twoRoles,
			creatorCRBs:  nil,
			want:         false,
		},
		{
			name:         "creator with a binding, no prtbs is ready",
			annotations:  map[string]string{projectIDAnnotation: projectID, "field.cattle.io/creatorId": creator},
			clusterRoles: twoRoles,
			creatorCRBs:  []*rbacv1.ClusterRoleBinding{{ObjectMeta: metav1.ObjectMeta{Name: "crb"}}},
			want:         true,
		},
		{
			name:          "project has prtbs but no bindings yet is not ready",
			annotations:   map[string]string{projectIDAnnotation: projectID},
			clusterRoles:  twoRoles,
			prtbs:         []*v3.ProjectRoleTemplateBinding{prtb("p-123xyz", "member-prtb")},
			bindings:      nil,
			expectListRBs: true,
			want:          false,
		},
		{
			name:          "project has prtbs and a binding is ready",
			annotations:   map[string]string{projectIDAnnotation: projectID},
			clusterRoles:  twoRoles,
			prtbs:         []*v3.ProjectRoleTemplateBinding{prtb("p-123xyz", "member-prtb")},
			bindings:      []*rbacv1.RoleBinding{aggregationRoleBinding("rb", namespaceName, pkgrbac.GetPRTBOwnerLabel("member-prtb"))},
			expectListRBs: true,
			want:          true,
		},
		{
			name:          "project has prtbs but only an unrelated binding is not ready",
			annotations:   map[string]string{projectIDAnnotation: projectID},
			clusterRoles:  twoRoles,
			prtbs:         []*v3.ProjectRoleTemplateBinding{prtb("p-123xyz", "member-prtb")},
			bindings:      []*rbacv1.RoleBinding{{ObjectMeta: metav1.ObjectMeta{Name: "rb", Namespace: namespaceName}}},
			expectListRBs: true,
			want:          false,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			crIndexer := &FakeResourceIndexer[*rbacv1.ClusterRole]{
				resources: map[string][]*rbacv1.ClusterRole{namespaceName: test.clusterRoles},
				index:     crByNSIndex,
			}
			crbKey := rbRoleSubjectKey(twoRoles[0].Name, rbacv1.Subject{Kind: "User", Name: creator})
			crbIndexer := &FakeResourceIndexer[*rbacv1.ClusterRoleBinding]{
				resources: map[string][]*rbacv1.ClusterRoleBinding{crbKey: test.creatorCRBs},
				index:     crbByRoleAndSubjectIndex,
			}
			prtbIndexer := &FakeResourceIndexer[*v3.ProjectRoleTemplateBinding]{
				resources: map[string][]*v3.ProjectRoleTemplateBinding{projectID: test.prtbs},
				index:     prtbByProjectIndex,
			}
			rbLister := wfakes.NewMockCacheInterface[*rbacv1.RoleBinding](ctrl)
			if test.expectListRBs {
				rbLister.EXPECT().List(namespaceName, gomock.Any()).Return(test.bindings, nil)
			}

			lifecycle := nsLifecycle{m: &manager{
				crIndexer:   crIndexer,
				crbIndexer:  crbIndexer,
				prtbIndexer: prtbIndexer,
				rbLister:    rbLister,
			}}

			ready, err := lifecycle.initialRolesReady(&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: namespaceName, Annotations: test.annotations},
			})
			assert.NoError(t, err)
			assert.Equal(t, test.want, ready)
		})
	}
}

func TestReconcileInitialRolesCondition(t *testing.T) {
	const (
		namespaceName = "test-ns"
		projectID     = "c-123xyz:p-123xyz"
	)

	newNS := func() *corev1.Namespace {
		return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name:        namespaceName,
			Annotations: map[string]string{projectIDAnnotation: projectID},
		}}
	}
	twoRoles := []*rbacv1.ClusterRole{
		{ObjectMeta: metav1.ObjectMeta{Name: "readonly"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "edit"}},
	}

	t.Run("already-set condition short-circuits without lookups or writes", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ns := newNS()
		assert.NoError(t, namespaceutil.SetNamespaceCondition(ns, time.Second, initialRoleCondition, true, ""))
		// nil indexers/namespaces client would panic if the readiness path or an update ran.
		namespaces := wfakes.NewMockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList](ctrl)

		lifecycle := nsLifecycle{m: &manager{namespaces: namespaces}}
		got, err := lifecycle.reconcileInitialRolesCondition(ns)
		assert.NoError(t, err)
		assert.Same(t, ns, got)
	})

	t.Run("not ready requeues via EnqueueAfter and does not update", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		crIndexer := &FakeResourceIndexer[*rbacv1.ClusterRole]{
			resources: map[string][]*rbacv1.ClusterRole{namespaceName: twoRoles[:1]}, // <2 => not ready
			index:     crByNSIndex,
		}
		namespaces := wfakes.NewMockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList](ctrl)
		namespaces.EXPECT().EnqueueAfter(namespaceName, initialRolesRequeueInterval)

		lifecycle := nsLifecycle{m: &manager{crIndexer: crIndexer, namespaces: namespaces}}
		_, err := lifecycle.reconcileInitialRolesCondition(newNS())
		assert.NoError(t, err)
	})

	t.Run("ready sets the condition and updates the namespace", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		crIndexer := &FakeResourceIndexer[*rbacv1.ClusterRole]{
			resources: map[string][]*rbacv1.ClusterRole{namespaceName: twoRoles},
			index:     crByNSIndex,
		}
		prtbIndexer := &FakeResourceIndexer[*v3.ProjectRoleTemplateBinding]{
			resources: map[string][]*v3.ProjectRoleTemplateBinding{projectID: nil}, // no prtbs => no binding wait
			index:     prtbByProjectIndex,
		}
		namespaces := wfakes.NewMockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList](ctrl)
		namespaces.EXPECT().Update(gomock.Any()).DoAndReturn(func(ns *corev1.Namespace) (*corev1.Namespace, error) {
			set, err := namespaceutil.IsNamespaceConditionSet(ns, initialRoleCondition, true)
			assert.NoError(t, err)
			assert.True(t, set, "expected %s condition set to true on the updated namespace", initialRoleCondition)
			return ns, nil
		})

		lifecycle := nsLifecycle{m: &manager{crIndexer: crIndexer, prtbIndexer: prtbIndexer, namespaces: namespaces}}
		_, err := lifecycle.reconcileInitialRolesCondition(newNS())
		assert.NoError(t, err)
	})
}
