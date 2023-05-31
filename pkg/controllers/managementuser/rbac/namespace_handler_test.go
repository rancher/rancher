package rbac

import (
	"fmt"
	"testing"

	"github.com/rancher/rancher/pkg/apis/management.cattle.io"
	apisV3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1/fakes"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

func TestReconcileNamespaceProjectClusterRole(t *testing.T) {
	const namespaceName = "test-ns"
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
		test := test
		t.Run(test.name, func(t *testing.T) {
			var newRoles []*rbacv1.ClusterRole
			var deletedRoleNames []string
			indexer := DummyIndexer{
				clusterRoles: map[string][]*rbacv1.ClusterRole{
					namespaceName: test.indexedRoles,
				},
				err: test.indexerError,
			}
			fakeRBACInterface := &fakeRBAC{
				clusterRoleFake: fakes.ClusterRoleInterfaceMock{
					CreateFunc: func(in *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
						newRoles = append(newRoles, in)
						if test.createError != nil {
							return nil, test.createError
						}
						return in, nil
					},
					UpdateFunc: func(in *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
						newRoles = append(newRoles, in)
						if test.updateError != nil {
							return nil, test.updateError
						}
						return in, nil
					},
					DeleteFunc: func(name string, options *metav1.DeleteOptions) error {
						deletedRoleNames = append(deletedRoleNames, name)
						if test.deleteError != nil {
							return test.deleteError
						}
						return nil
					},
				},
			}
			fakeLister := &fakes.ClusterRoleListerMock{
				GetFunc: func(namespace string, name string) (*rbacv1.ClusterRole, error) {
					if test.getError != nil {
						return nil, test.getError
					}
					for _, role := range test.currentRoles {
						if role.Name == name {
							return role, nil
						}
					}
					return nil, apierror.NewNotFound(schema.GroupResource{
						Group:    "rbac.authorization.k8s.io",
						Resource: "ClusterRoles",
					}, name)
				},
			}
			lifecycle := nsLifecycle{
				m: &manager{
					workload: &config.UserContext{
						RBAC: fakeRBACInterface,
					},
					crLister:  fakeLister,
					crIndexer: &indexer,
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

type DummyIndexer struct {
	cache.Store
	clusterRoles map[string][]*rbacv1.ClusterRole
	err          error
}

func (d *DummyIndexer) Index(indexName string, obj interface{}) ([]interface{}, error) {
	return nil, nil
}

func (d *DummyIndexer) IndexKeys(indexName, indexKey string) ([]string, error) {
	return []string{}, nil
}

func (d *DummyIndexer) ListIndexFuncValues(indexName string) []string {
	return []string{}
}

func (d *DummyIndexer) ByIndex(indexName, indexKey string) ([]interface{}, error) {
	if indexName != crByNSIndex {
		return nil, fmt.Errorf("dummy indexer only supports %s for index name, %s given", crByNSIndex, indexName)
	}
	if d.err != nil {
		return nil, d.err
	}
	crs := d.clusterRoles[indexKey]
	var interfaces []interface{}
	for _, cr := range crs {
		interfaces = append(interfaces, cr)
	}
	return interfaces, nil
}

func (d *DummyIndexer) GetIndexers() cache.Indexers {
	return nil
}

func (d *DummyIndexer) AddIndexers(newIndexers cache.Indexers) error {
	return nil
}
