package auth

import (
	"fmt"
	"testing"

	normanFakes "github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	fakes "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	rbacFakes "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1/fakes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
)

var roles = map[string]*v3.RoleTemplate{
	"recursive1": {
		RoleTemplateNames: []string{"recursive2"},
	},
	"recursive2": {
		RoleTemplateNames: []string{"recursive1"},
	},
	"non-recursive": {},
	"inherit non-recursive": {
		RoleTemplateNames: []string{"non-recursive"},
	},
}

func Test_checkReferencedRoles(t *testing.T) {
	manager := &manager{
		rtLister: &fakes.RoleTemplateListerMock{
			GetFunc: roleListerGetFunc,
		},
	}

	type args struct {
		rtName       string
		rtContext    string
		depthCounter int
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Non-recursive role, none inherited",
			args: args{
				rtName:       "non-recursive",
				rtContext:    "",
				depthCounter: 0,
			},
			wantErr: false,
		},
		{
			name: "Non-recursive role, inherits another",
			args: args{
				rtName:       "inherit non-recursive",
				rtContext:    "",
				depthCounter: 0,
			},
			wantErr: false,
		},
		{
			name: "Recursive role",
			args: args{
				rtName:       "recursive1",
				rtContext:    "",
				depthCounter: 0,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.checkReferencedRoles(tt.args.rtName, tt.args.rtContext, tt.args.depthCounter)
			if tt.wantErr {
				assert.Error(t, err, "expected an error, got none")
			} else {
				assert.NoError(t, err, fmt.Sprintf("expected no error, got: %v", err))
			}
		})
	}
}

func roleListerGetFunc(ns, name string) (*v3.RoleTemplate, error) {
	role, ok := roles[name]
	if !ok {
		return nil, errors.NewNotFound(schema.GroupResource{
			Group:    v3.RoleTemplateGroupVersionKind.Group,
			Resource: v3.RoleTemplateGroupVersionResource.Resource,
		}, name)
	}
	return role, nil
}

func Test_reconcileDesiredMGMTPlaneRoleBindings(t *testing.T) {
	t.Parallel()

	type StateChanges struct {
		t          *testing.T
		createdRBs map[string]*rbacv1.RoleBinding
		deletedRBs map[string]bool
	}

	type State struct {
		nsListerMock *normanFakes.NamespaceListerMock
		rbClientMock *rbacFakes.RoleBindingInterfaceMock
		stateChanges *StateChanges
	}

	rb1 := &rbacv1.RoleBinding{
		ObjectMeta: v1.ObjectMeta{
			Name:      "rb1",
			Namespace: "ns1",
		},
		RoleRef: rbacv1.RoleRef{
			Name: "roleRef1",
		},
		Subjects: []rbacv1.Subject{{Name: "subject1"}},
	}
	rb2 := &rbacv1.RoleBinding{
		ObjectMeta: v1.ObjectMeta{
			Name:      "rb2",
			Namespace: "ns2",
		},
		RoleRef: rbacv1.RoleRef{
			Name: "roleRef2",
		},
		Subjects: []rbacv1.Subject{{Name: "subject2"}},
	}
	rb3 := &rbacv1.RoleBinding{
		ObjectMeta: v1.ObjectMeta{
			Name:      "rb3",
			Namespace: "ns3",
		},
		RoleRef: rbacv1.RoleRef{
			Name: "roleRef3",
		},
		Subjects: []rbacv1.Subject{{Name: "subject3"}},
	}

	tests := []struct {
		name            string
		currentRBs      map[string]*rbacv1.RoleBinding
		desiredRBs      map[string]*rbacv1.RoleBinding
		stateSetup      func(State)
		stateAssertions func(StateChanges)
		wantError       bool
	}{
		{
			name: "get namespace fails",
			stateSetup: func(state State) {
				state.nsListerMock.GetFunc = func(namespace string, name string) (*corev1.Namespace, error) {
					return nil, fmt.Errorf("error")
				}
			},
			wantError: true,
		},
		{
			name: "namespace is terminating",
			stateSetup: func(state State) {
				state.nsListerMock.GetFunc = func(namespace string, name string) (*corev1.Namespace, error) {
					return &corev1.Namespace{
						Status: corev1.NamespaceStatus{
							Phase: corev1.NamespaceTerminating,
						},
					}, nil
				}
			},
			wantError: false,
		},
		{
			name: "create rb fails",
			stateSetup: func(state State) {
				state.nsListerMock.GetFunc = func(namespace string, name string) (*corev1.Namespace, error) {
					return &corev1.Namespace{
						Status: corev1.NamespaceStatus{
							Phase: corev1.NamespaceActive,
						},
					}, nil
				}
				state.rbClientMock.CreateFunc = func(rb *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
					return nil, fmt.Errorf("error")
				}
				state.rbClientMock.DeleteNamespacedFunc = func(_, name string, _ *v1.DeleteOptions) error {
					return nil
				}
			},
			currentRBs: map[string]*rbacv1.RoleBinding{"rb1": rb1},
			desiredRBs: map[string]*rbacv1.RoleBinding{"rb1": rb1, "rb2": rb2},
			wantError:  true,
		},
		{
			name: "delete rb fails",
			stateSetup: func(state State) {
				state.nsListerMock.GetFunc = func(namespace string, name string) (*corev1.Namespace, error) {
					return &corev1.Namespace{
						Status: corev1.NamespaceStatus{
							Phase: corev1.NamespaceActive,
						},
					}, nil
				}
				state.rbClientMock.CreateFunc = func(rb *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
					return nil, nil
				}
				state.rbClientMock.DeleteNamespacedFunc = func(_, name string, _ *v1.DeleteOptions) error {
					return fmt.Errorf("error")
				}
			},
			currentRBs: map[string]*rbacv1.RoleBinding{"rb1": rb1, "rb2": rb2},
			desiredRBs: map[string]*rbacv1.RoleBinding{"rb1": rb1},
			wantError:  true,
		},
		{
			name: "add new rb",
			stateSetup: func(state State) {
				state.nsListerMock.GetFunc = func(namespace string, name string) (*corev1.Namespace, error) {
					return &corev1.Namespace{
						Status: corev1.NamespaceStatus{
							Phase: corev1.NamespaceActive,
						},
					}, nil
				}
				state.rbClientMock.CreateFunc = func(rb *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
					state.stateChanges.createdRBs[rb.Name] = rb
					return nil, nil
				}
				state.rbClientMock.DeleteNamespacedFunc = func(_, name string, _ *v1.DeleteOptions) error {
					state.stateChanges.deletedRBs[name] = true
					return nil
				}
			},
			stateAssertions: func(stateChanges StateChanges) {
				require.Len(stateChanges.t, stateChanges.createdRBs, 1)
				require.Contains(stateChanges.t, stateChanges.createdRBs, "rb2")
				require.Len(stateChanges.t, stateChanges.deletedRBs, 0)
			},
			currentRBs: map[string]*rbacv1.RoleBinding{"rb1": rb1},
			desiredRBs: map[string]*rbacv1.RoleBinding{"rb1": rb1, "rb2": rb2},
			wantError:  false,
		},
		{
			name: "delete unwanted rb",
			stateSetup: func(state State) {
				state.nsListerMock.GetFunc = func(namespace string, name string) (*corev1.Namespace, error) {
					return &corev1.Namespace{
						Status: corev1.NamespaceStatus{
							Phase: corev1.NamespaceActive,
						},
					}, nil
				}
				state.rbClientMock.CreateFunc = func(rb *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
					state.stateChanges.createdRBs[rb.Name] = rb
					return nil, nil
				}
				state.rbClientMock.DeleteNamespacedFunc = func(_, name string, _ *v1.DeleteOptions) error {
					state.stateChanges.deletedRBs[name] = true
					return nil
				}
			},
			stateAssertions: func(stateChanges StateChanges) {
				require.Len(stateChanges.t, stateChanges.createdRBs, 0)
				require.Len(stateChanges.t, stateChanges.deletedRBs, 1)
				require.Contains(stateChanges.t, stateChanges.deletedRBs, "rb2")
			},
			currentRBs: map[string]*rbacv1.RoleBinding{"rb1": rb1, "rb2": rb2},
			desiredRBs: map[string]*rbacv1.RoleBinding{"rb1": rb1},
			wantError:  false,
		},
		{
			name: "delete unwanted rb and add new rb",
			stateSetup: func(state State) {
				state.nsListerMock.GetFunc = func(namespace string, name string) (*corev1.Namespace, error) {
					return &corev1.Namespace{
						Status: corev1.NamespaceStatus{
							Phase: corev1.NamespaceActive,
						},
					}, nil
				}
				state.rbClientMock.CreateFunc = func(rb *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
					state.stateChanges.createdRBs[rb.Name] = rb
					return nil, nil
				}
				state.rbClientMock.DeleteNamespacedFunc = func(_, name string, _ *v1.DeleteOptions) error {
					state.stateChanges.deletedRBs[name] = true
					return nil
				}
			},
			stateAssertions: func(stateChanges StateChanges) {
				require.Len(stateChanges.t, stateChanges.createdRBs, 1)
				require.Contains(stateChanges.t, stateChanges.createdRBs, "rb3")
				require.Len(stateChanges.t, stateChanges.deletedRBs, 1)
				require.Contains(stateChanges.t, stateChanges.deletedRBs, "rb2")
			},
			currentRBs: map[string]*rbacv1.RoleBinding{"rb1": rb1, "rb2": rb2},
			desiredRBs: map[string]*rbacv1.RoleBinding{"rb1": rb1, "rb3": rb3},
			wantError:  false,
		},
		{
			name: "ignore duplicate current rbs",
			stateSetup: func(state State) {
				state.nsListerMock.GetFunc = func(namespace string, name string) (*corev1.Namespace, error) {
					return &corev1.Namespace{
						Status: corev1.NamespaceStatus{
							Phase: corev1.NamespaceActive,
						},
					}, nil
				}
				state.rbClientMock.CreateFunc = func(rb *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
					state.stateChanges.createdRBs[rb.Name] = rb
					return nil, nil
				}
				state.rbClientMock.DeleteNamespacedFunc = func(_, name string, _ *v1.DeleteOptions) error {
					state.stateChanges.deletedRBs[name] = true
					return nil
				}
			},
			stateAssertions: func(stateChanges StateChanges) {
				require.Len(stateChanges.t, stateChanges.createdRBs, 0)
				require.Len(stateChanges.t, stateChanges.deletedRBs, 0)
			},
			currentRBs: map[string]*rbacv1.RoleBinding{"rb1": rb1, "rb2": rb1},
			desiredRBs: map[string]*rbacv1.RoleBinding{"rb1": rb1},
			wantError:  false,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			manager := manager{}
			nsLister := normanFakes.NamespaceListerMock{}
			rbClient := rbacFakes.RoleBindingInterfaceMock{}

			stateChanges := StateChanges{
				t:          t,
				createdRBs: map[string]*rbacv1.RoleBinding{},
				deletedRBs: map[string]bool{},
			}
			state := State{
				nsListerMock: &nsLister,
				rbClientMock: &rbClient,
				stateChanges: &stateChanges,
			}
			if test.stateSetup != nil {
				test.stateSetup(state)
			}
			manager.nsLister = &nsLister
			manager.rbClient = &rbClient

			err := manager.reconcileDesiredMGMTPlaneRoleBindings(test.currentRBs, test.desiredRBs, "")
			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if test.stateAssertions != nil {
				test.stateAssertions(*state.stateChanges)
			}
		})
	}
}

func Test_reconcileManagementPlaneRole(t *testing.T) {
	t.Parallel()

	type StateChanges struct {
		t       *testing.T
		newRole *rbacv1.Role
	}

	type State struct {
		nsListerMock *normanFakes.NamespaceListerMock
		rListerMock  *rbacFakes.RoleListerMock
		rClientMock  *rbacFakes.RoleInterfaceMock
		stateChanges *StateChanges
	}

	rules := map[string]map[string]string{
		"resource1": {
			"verb1": "group1",
			"verb2": "group1",
		},
		"resource2": {
			"verb3": "group2",
			"verb4": "group2",
		},
	}
	rule1 := rbacv1.PolicyRule{
		Resources: []string{"resource1"},
		Verbs:     []string{"verb1", "verb2"},
		APIGroups: []string{"group1"},
	}
	rule2 := rbacv1.PolicyRule{
		Resources: []string{"resource2"},
		Verbs:     []string{"verb3", "verb4"},
		APIGroups: []string{"group2"},
	}
	rule3 := rbacv1.PolicyRule{
		Resources: []string{"resource3"},
		Verbs:     []string{"verb3", "verb4"},
		APIGroups: []string{"group3"},
	}
	roleTemplate := &v3.RoleTemplate{
		ObjectMeta: v1.ObjectMeta{
			Name: "roleTemplate",
		},
	}
	activeNamespace := &corev1.Namespace{
		Status: corev1.NamespaceStatus{
			Phase: corev1.NamespaceActive,
		},
	}

	tests := []struct {
		name            string
		namespace       string
		resourceToVerbs map[string]map[string]string
		roleTemplate    *v3.RoleTemplate
		stateSetup      func(State)
		stateAssertions func(StateChanges)
		wantError       bool
	}{
		{
			name: "get namespace fails",
			stateSetup: func(state State) {
				state.nsListerMock.GetFunc = func(_, _ string) (*corev1.Namespace, error) {
					return nil, fmt.Errorf("error")
				}
				state.rListerMock.GetFunc = func(_, _ string) (*rbacv1.Role, error) {
					return nil, nil
				}
			},
			roleTemplate:    roleTemplate,
			resourceToVerbs: rules,
			wantError:       true,
		},
		{
			name: "namespace is terminating",
			stateSetup: func(state State) {
				state.nsListerMock.GetFunc = func(_, _ string) (*corev1.Namespace, error) {
					return &corev1.Namespace{
						Status: corev1.NamespaceStatus{
							Phase: corev1.NamespaceTerminating,
						},
					}, nil
				}
				state.rListerMock.GetFunc = func(_, _ string) (*rbacv1.Role, error) {
					return nil, nil
				}
			},
			roleTemplate:    roleTemplate,
			resourceToVerbs: rules,
			wantError:       false,
		},
		{
			name: "create role fails",
			stateSetup: func(state State) {
				state.nsListerMock.GetFunc = func(_, _ string) (*corev1.Namespace, error) {
					return activeNamespace, nil
				}
				state.rListerMock.GetFunc = func(_, _ string) (*rbacv1.Role, error) {
					return nil, nil
				}
				state.rClientMock.CreateFunc = func(role *rbacv1.Role) (*rbacv1.Role, error) {
					return nil, fmt.Errorf("error")
				}
			},
			roleTemplate:    roleTemplate,
			resourceToVerbs: rules,
			wantError:       true,
		},
		{
			name: "role already has the right verbs",
			stateSetup: func(state State) {
				state.rListerMock.GetFunc = func(_, _ string) (*rbacv1.Role, error) {
					role := &rbacv1.Role{
						Rules: []rbacv1.PolicyRule{rule1, rule2},
					}
					return role, nil
				}
				// it should not create a role
				state.rClientMock.CreateFunc = func(role *rbacv1.Role) (*rbacv1.Role, error) {
					state.stateChanges.newRole = role
					return nil, nil
				}
			},
			stateAssertions: func(stateChanges StateChanges) {
				require.NotNil(stateChanges.t, stateChanges.newRole)
				require.Len(stateChanges.t, stateChanges.newRole.Rules, 0)
			},
			roleTemplate:    roleTemplate,
			resourceToVerbs: rules,
			wantError:       false,
		},
		{
			name: "role does not exist",
			stateSetup: func(state State) {
				state.nsListerMock.GetFunc = func(_, _ string) (*corev1.Namespace, error) {
					return activeNamespace, nil
				}
				state.rListerMock.GetFunc = func(_, _ string) (*rbacv1.Role, error) {
					return nil, nil
				}
				state.rClientMock.CreateFunc = func(role *rbacv1.Role) (*rbacv1.Role, error) {
					state.stateChanges.newRole = role
					return nil, nil
				}
			},
			stateAssertions: func(stateChanges StateChanges) {
				require.NotNil(stateChanges.t, stateChanges.newRole)
				require.Len(stateChanges.t, stateChanges.newRole.Rules, 2)
				require.Contains(stateChanges.t, stateChanges.newRole.Rules, rule1)
				require.Contains(stateChanges.t, stateChanges.newRole.Rules, rule2)
				require.Equal(stateChanges.t, "roleTemplate", stateChanges.newRole.Name)
			},
			roleTemplate:    roleTemplate,
			resourceToVerbs: rules,
			wantError:       false,
		},
		{
			name: "role is missing a rule",
			stateSetup: func(state State) {
				state.rListerMock.GetFunc = func(_, _ string) (*rbacv1.Role, error) {
					role := &rbacv1.Role{
						ObjectMeta: v1.ObjectMeta{
							Name: "role",
						},
						Rules: []rbacv1.PolicyRule{rule1},
					}
					return role, nil
				}
				state.rClientMock.UpdateFunc = func(role *rbacv1.Role) (*rbacv1.Role, error) {
					state.stateChanges.newRole = role
					return nil, nil
				}
			},
			stateAssertions: func(stateChanges StateChanges) {
				require.NotNil(stateChanges.t, stateChanges.newRole)
				require.Len(stateChanges.t, stateChanges.newRole.Rules, 2)
				require.Contains(stateChanges.t, stateChanges.newRole.Rules, rule1)
				require.Contains(stateChanges.t, stateChanges.newRole.Rules, rule2)
				require.Equal(stateChanges.t, "role", stateChanges.newRole.Name)
			},
			roleTemplate:    roleTemplate,
			resourceToVerbs: rules,
			wantError:       false,
		},
		{
			name: "role has no rules",
			stateSetup: func(state State) {
				state.rListerMock.GetFunc = func(_, _ string) (*rbacv1.Role, error) {
					role := &rbacv1.Role{
						ObjectMeta: v1.ObjectMeta{
							Name: "role",
						},
						Rules: []rbacv1.PolicyRule{},
					}
					return role, nil
				}
				state.rClientMock.UpdateFunc = func(role *rbacv1.Role) (*rbacv1.Role, error) {
					state.stateChanges.newRole = role
					return nil, nil
				}
			},
			stateAssertions: func(stateChanges StateChanges) {
				require.NotNil(stateChanges.t, stateChanges.newRole)
				require.Len(stateChanges.t, stateChanges.newRole.Rules, 2)
				require.Contains(stateChanges.t, stateChanges.newRole.Rules, rule1)
				require.Contains(stateChanges.t, stateChanges.newRole.Rules, rule2)
				require.Equal(stateChanges.t, "role", stateChanges.newRole.Name)
			},
			roleTemplate:    roleTemplate,
			resourceToVerbs: rules,
			wantError:       false,
		},
		{
			name: "role has rule that is missing verb",
			stateSetup: func(state State) {
				state.rListerMock.GetFunc = func(_, _ string) (*rbacv1.Role, error) {
					role := &rbacv1.Role{
						ObjectMeta: v1.ObjectMeta{
							Name: "role",
						},
						Rules: []rbacv1.PolicyRule{
							rule1,
							{
								Resources: []string{"resource2"},
								Verbs:     []string{"verb3"},
								APIGroups: []string{"group2"},
							},
						},
					}
					return role, nil
				}
				state.rClientMock.UpdateFunc = func(role *rbacv1.Role) (*rbacv1.Role, error) {
					state.stateChanges.newRole = role
					return nil, nil
				}
			},
			stateAssertions: func(stateChanges StateChanges) {
				require.NotNil(stateChanges.t, stateChanges.newRole)
				require.Len(stateChanges.t, stateChanges.newRole.Rules, 2)
				require.Contains(stateChanges.t, stateChanges.newRole.Rules, rule1)
				require.Contains(stateChanges.t, stateChanges.newRole.Rules, rule2)
				require.Equal(stateChanges.t, "role", stateChanges.newRole.Name)
			},
			roleTemplate:    roleTemplate,
			resourceToVerbs: rules,
			wantError:       false,
		},
		{
			name: "existing role rules are a superset of resourceToVerbs",
			stateSetup: func(state State) {
				state.rListerMock.GetFunc = func(_, _ string) (*rbacv1.Role, error) {
					role := &rbacv1.Role{
						ObjectMeta: v1.ObjectMeta{
							Name: "role",
						},
						Rules: []rbacv1.PolicyRule{
							{
								Resources: []string{"*"},
								Verbs:     []string{"verb1", "verb2"},
								APIGroups: []string{"group1"},
							},
							{
								Resources: []string{"resource2"},
								Verbs:     []string{"verb3", "verb4"},
								APIGroups: []string{"*"},
							},
						},
					}
					return role, nil
				}
				state.rClientMock.UpdateFunc = func(role *rbacv1.Role) (*rbacv1.Role, error) {
					state.stateChanges.newRole = role
					return nil, nil
				}
			},
			stateAssertions: func(stateChanges StateChanges) {
				require.NotNil(stateChanges.t, stateChanges.newRole)
				require.Len(stateChanges.t, stateChanges.newRole.Rules, 0)
			},
			roleTemplate:    roleTemplate,
			resourceToVerbs: rules,
			wantError:       false,
		},
		{
			name: "role have an extra rule, which means a Rule was removed from the RoleTemplate and should be removed",
			stateSetup: func(state State) {
				state.rListerMock.GetFunc = func(_, _ string) (*rbacv1.Role, error) {
					role := &rbacv1.Role{
						ObjectMeta: v1.ObjectMeta{
							Name: "role",
						},
						Rules: []rbacv1.PolicyRule{rule1, rule2, rule3},
					}
					return role, nil
				}
				state.rClientMock.UpdateFunc = func(role *rbacv1.Role) (*rbacv1.Role, error) {
					state.stateChanges.newRole = role
					return nil, nil
				}
			},
			stateAssertions: func(stateChanges StateChanges) {
				require.NotNil(stateChanges.t, stateChanges.newRole)
				require.Len(stateChanges.t, stateChanges.newRole.Rules, 2)
				require.Contains(stateChanges.t, stateChanges.newRole.Rules, rule1)
				require.Contains(stateChanges.t, stateChanges.newRole.Rules, rule2)
				require.Equal(stateChanges.t, "role", stateChanges.newRole.Name)
			},
			roleTemplate:    roleTemplate,
			resourceToVerbs: rules,
			wantError:       false,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			manager := manager{}
			nsLister := normanFakes.NamespaceListerMock{}
			rLister := rbacFakes.RoleListerMock{}
			rClient := rbacFakes.RoleInterfaceMock{}

			stateChanges := StateChanges{
				t:       t,
				newRole: &rbacv1.Role{},
			}
			state := State{
				nsListerMock: &nsLister,
				rListerMock:  &rLister,
				rClientMock:  &rClient,
				stateChanges: &stateChanges,
			}
			if test.stateSetup != nil {
				test.stateSetup(state)
			}
			manager.nsLister = &nsLister
			manager.rLister = &rLister
			manager.rClient = &rClient

			err := manager.reconcileManagementPlaneRole(test.namespace, test.resourceToVerbs, test.roleTemplate)
			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if test.stateAssertions != nil {
				test.stateAssertions(*state.stateChanges)
			}
		})
	}
}

func Test_gatherAndDedupeRoles(t *testing.T) {
	tests := []struct {
		name             string
		roleTemplateName string
		wantRTErr        bool
		wantErr          bool
		want             map[string]*v3.RoleTemplate
	}{
		{
			name:             "Role with no inheritance",
			roleTemplateName: "non-recursive",
			wantErr:          false,
			want: map[string]*v3.RoleTemplate{
				"non-recursive": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "non-recursive",
					},
				},
			},
		},
		{
			name:      "RT get error",
			wantRTErr: true,
			wantErr:   true,
		},
		{
			name:             "Role with dupe roletemplates",
			roleTemplateName: "rolewithdupes",
			want: map[string]*v3.RoleTemplate{
				"rolewithdupes": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "rolewithdupes",
					},
					RoleTemplateNames: []string{"rt1", "rt2", "rt1"},
				},
				"rt1": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "rt1",
					},
				},
				"rt2": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "rt2",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &manager{
				rtLister: &fakes.RoleTemplateListerMock{
					GetFunc: func(namespace, name string) (*v3.RoleTemplate, error) {
						if tt.wantRTErr {
							return nil, fmt.Errorf("error getting role template")
						}
						if name == "rolewithdupes" {
							return &v3.RoleTemplate{
								ObjectMeta: metav1.ObjectMeta{
									Name: "rolewithdupes",
								},
								RoleTemplateNames: []string{"rt1", "rt2", "rt1"},
							}, nil
						}
						return &v3.RoleTemplate{
							ObjectMeta: metav1.ObjectMeta{
								Name: name,
							},
						}, nil
					},
				},
			}
			got, err := manager.gatherAndDedupeRoles(tt.roleTemplateName)
			if tt.wantErr {
				assert.Error(t, err, "expected an error, got none")
			} else {
				assert.NoError(t, err, fmt.Sprintf("expected no error, got: %v", err))
				assert.Equal(t, tt.want, got, "expected roles to be %v, got: %v", tt.want, got)
			}
		})
	}
}

func Test_gatherRoleTemplates(t *testing.T) {
	roleTemplates := map[string]*v3.RoleTemplate{
		"root": {
			ObjectMeta: metav1.ObjectMeta{
				Name: "root",
			},
			RoleTemplateNames: []string{"child1"},
		},
		"child1": {
			ObjectMeta: metav1.ObjectMeta{
				Name: "child1",
			},
			RoleTemplateNames: []string{"child2"},
		},
		"child2": {
			ObjectMeta: metav1.ObjectMeta{
				Name: "child2",
			},
			RoleTemplateNames: []string{},
		},
	}

	tests := []struct {
		name             string
		roleTemplateName string
		wantErr          bool
		want             map[string]*v3.RoleTemplate
	}{
		{
			name:             "hierarchy of roletemplates",
			roleTemplateName: "root",
			wantErr:          false,
			want: map[string]*v3.RoleTemplate{
				"root":   roleTemplates["root"],
				"child1": roleTemplates["child1"],
				"child2": roleTemplates["child2"],
			},
		},
		{
			name:             "error getting roletemplate",
			roleTemplateName: "root",
			wantErr:          true,
			want:             nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &manager{
				rtLister: &fakes.RoleTemplateListerMock{
					GetFunc: func(namespace, name string) (*v3.RoleTemplate, error) {
						rt, _ := roleTemplates[name]
						if tt.wantErr {
							return nil, fmt.Errorf("RoleTemplate not found")
						}
						return rt, nil
					},
				},
			}
			got := map[string]*v3.RoleTemplate{}
			err := manager.gatherRoleTemplates(roleTemplates[tt.roleTemplateName], got)
			if tt.wantErr {
				assert.Error(t, err, "expected an error, got none")
			} else {
				assert.NoError(t, err, fmt.Sprintf("expected no error, got: %v", err))
				assert.Equal(t, tt.want, got, "expected roles to be %v, got: %v", tt.want, got)
			}
		})
	}
}

// TestGrantManagementPlanePrivilegesSkipsCRTTokenReaderRoleBinding verifies that the
// crt-token-reader RoleBinding (managed independently in crtb_handler.go) is never deleted as
// undesired here, while other stale RoleBindings still are.
func TestGrantManagementPlanePrivilegesSkipsCRTTokenReaderRoleBinding(t *testing.T) {
	const bindingUID = types.UID("test-uid")

	binding := &v3.ClusterRoleTemplateBinding{
		ObjectMeta: v1.ObjectMeta{
			Name:      "crtb1",
			Namespace: "ns1",
			UID:       bindingUID,
		},
		RoleTemplateName: "test-rt",
	}
	ownerRef := v1.OwnerReference{UID: bindingUID}
	subject := rbacv1.Subject{Name: "test-user"}

	crtTokenReaderRB := &rbacv1.RoleBinding{
		ObjectMeta: v1.ObjectMeta{
			Name:            crtTokenReaderRoleBindingName("ns1", subject),
			Namespace:       "ns1",
			OwnerReferences: []v1.OwnerReference{ownerRef},
		},
		RoleRef: rbacv1.RoleRef{Kind: "Role", Name: "crt-token-reader"},
	}
	staleRB := &rbacv1.RoleBinding{
		ObjectMeta: v1.ObjectMeta{
			Name:            "crtb1-old-role",
			Namespace:       "ns1",
			OwnerReferences: []v1.OwnerReference{ownerRef},
		},
		RoleRef: rbacv1.RoleRef{Kind: "Role", Name: "old-role"},
	}

	rbIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
		rbByOwnerIndex: func(obj interface{}) ([]string, error) {
			return rbByOwner(obj.(*rbacv1.RoleBinding))
		},
	})
	require.NoError(t, rbIndexer.Add(crtTokenReaderRB))
	require.NoError(t, rbIndexer.Add(staleRB))

	var deletedNames []string
	rbClientMock := &rbacFakes.RoleBindingInterfaceMock{
		DeleteNamespacedFunc: func(namespace, name string, options *v1.DeleteOptions) error {
			deletedNames = append(deletedNames, name)
			return nil
		},
	}
	nsListerMock := &normanFakes.NamespaceListerMock{
		GetFunc: func(namespace, name string) (*corev1.Namespace, error) {
			return &corev1.Namespace{Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}}, nil
		},
	}

	m := &manager{
		rtLister: &fakes.RoleTemplateListerMock{
			GetFunc: func(namespace, name string) (*v3.RoleTemplate, error) {
				return &v3.RoleTemplate{ObjectMeta: v1.ObjectMeta{Name: name}}, nil
			},
		},
		rbIndexer:  rbIndexer,
		rbClient:   rbClientMock,
		nsLister:   nsListerMock,
		controller: "test",
	}

	// No management-plane resources requested, so no new desired RoleBindings are computed here;
	// this isolates the currentRBs filtering/deletion behavior under test.
	err := m.grantManagementPlanePrivileges("test-rt", map[string]string{}, subject, binding)
	require.NoError(t, err)

	assert.NotContains(t, deletedNames, crtTokenReaderRB.Name, "crt-token-reader RoleBinding should not be deleted by grantManagementPlanePrivileges")
	assert.Contains(t, deletedNames, staleRB.Name, "stale RoleBinding not owned by crt-token-reader should still be deleted")
}

// TestGrantManagementPlanePrivilegesCustomRoleTemplateNamedCRTTokenReader verifies that a custom
// RoleTemplate literally named "crt-token-reader" is not mistaken for the real reserved
// crt-token-reader binding (whose RoleBinding uses a hashed name, not the normal
// "<binding>-<role>" name). Its RoleBinding must go through normal reconcile - deleted once
// undesired (not permanently orphaned), and left alone while still desired (not churned/recreated
// every sync).
func TestGrantManagementPlanePrivilegesCustomRoleTemplateNamedCRTTokenReader(t *testing.T) {
	tests := []struct {
		name  string
		rules []rbacv1.PolicyRule // rules on the colliding "crt-token-reader" RoleTemplate
	}{
		{
			name:  "RoleTemplate no longer grants access - RoleBinding is deleted, not orphaned",
			rules: nil,
		},
		{
			name: "RoleTemplate still grants access - RoleBinding is left alone, not churned",
			rules: []rbacv1.PolicyRule{
				{APIGroups: []string{"management.cattle.io"}, Resources: []string{"clusterregistrationtokens"}, Verbs: []string{"get"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const bindingUID = types.UID("test-uid")

			binding := &v3.ClusterRoleTemplateBinding{
				ObjectMeta: v1.ObjectMeta{
					Name:      "crtb1",
					Namespace: "ns1",
					UID:       bindingUID,
				},
				RoleTemplateName: "crt-token-reader",
			}
			ownerRef := v1.OwnerReference{UID: bindingUID}

			// Named via the normal "<binding>-<role>" convention, not the hashed name used by the
			// real reserved crt-token-reader binding.
			collidingRB := &rbacv1.RoleBinding{
				ObjectMeta: v1.ObjectMeta{
					Name:            "crtb1-crt-token-reader",
					Namespace:       "ns1",
					OwnerReferences: []v1.OwnerReference{ownerRef},
				},
				RoleRef: rbacv1.RoleRef{Kind: "Role", Name: "crt-token-reader"},
			}

			rbIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
				rbByOwnerIndex: func(obj interface{}) ([]string, error) {
					return rbByOwner(obj.(*rbacv1.RoleBinding))
				},
			})
			require.NoError(t, rbIndexer.Add(collidingRB))

			var deletedNames, createdNames []string
			rbClientMock := &rbacFakes.RoleBindingInterfaceMock{
				DeleteNamespacedFunc: func(namespace, name string, options *v1.DeleteOptions) error {
					deletedNames = append(deletedNames, name)
					return nil
				},
				CreateFunc: func(rb *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
					createdNames = append(createdNames, rb.Name)
					return rb, nil
				},
			}
			nsListerMock := &normanFakes.NamespaceListerMock{
				GetFunc: func(namespace, name string) (*corev1.Namespace, error) {
					return &corev1.Namespace{Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}}, nil
				},
			}

			m := &manager{
				rtLister: &fakes.RoleTemplateListerMock{
					GetFunc: func(namespace, name string) (*v3.RoleTemplate, error) {
						return &v3.RoleTemplate{ObjectMeta: v1.ObjectMeta{Name: name}, Rules: tt.rules}, nil
					},
				},
				rLister: &rbacFakes.RoleListerMock{
					GetFunc: func(namespace, name string) (*rbacv1.Role, error) {
						return nil, errors.NewNotFound(schema.GroupResource{}, name)
					},
				},
				rClient: &rbacFakes.RoleInterfaceMock{
					CreateFunc: func(r *rbacv1.Role) (*rbacv1.Role, error) { return r, nil },
				},
				rbIndexer:  rbIndexer,
				rbClient:   rbClientMock,
				nsLister:   nsListerMock,
				controller: "test",
			}

			err := m.grantManagementPlanePrivileges("crt-token-reader", map[string]string{"clusterregistrationtokens": "management.cattle.io"}, rbacv1.Subject{Name: "test-user"}, binding)
			require.NoError(t, err)

			if tt.rules == nil {
				assert.Contains(t, deletedNames, collidingRB.Name, "RoleBinding for a custom RoleTemplate named crt-token-reader should be deleted once its RoleTemplate no longer grants access, not permanently orphaned")
			} else {
				assert.NotContains(t, deletedNames, collidingRB.Name, "RoleBinding for a custom RoleTemplate named crt-token-reader should not be deleted while its RoleTemplate still grants access")
				assert.NotContains(t, createdNames, collidingRB.Name, "RoleBinding already exists and matches desired state, so it should not be recreated (churn)")
			}
		})
	}
}

// TestGrantManagementPlanePrivilegesPRTBWithProjectRoleTemplateNamedCRTTokenReader is the PRTB
// equivalent of the above: grantManagementPlanePrivileges is also called for PRTBs, using the
// project's backing namespace. A project RoleTemplate named "crt-token-reader" must get the same
// normal reconcile treatment there too - deleted once undesired, left alone (not churned) while
// still desired.
func TestGrantManagementPlanePrivilegesPRTBWithProjectRoleTemplateNamedCRTTokenReader(t *testing.T) {
	tests := []struct {
		name  string
		rules []rbacv1.PolicyRule // rules on the colliding "crt-token-reader" RoleTemplate
	}{
		{
			name:  "RoleTemplate no longer grants access - RoleBinding is deleted, not orphaned",
			rules: nil,
		},
		{
			name: "RoleTemplate still grants access - RoleBinding is left alone, not churned",
			rules: []rbacv1.PolicyRule{
				{APIGroups: []string{"management.cattle.io"}, Resources: []string{"clusterregistrationtokens"}, Verbs: []string{"get"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const bindingUID = types.UID("test-uid")
			const projectNamespace = "p-abc123"

			binding := &v3.ProjectRoleTemplateBinding{
				ObjectMeta: v1.ObjectMeta{
					Name:      "prtb1",
					Namespace: projectNamespace,
					UID:       bindingUID,
				},
				RoleTemplateName: "crt-token-reader",
			}
			ownerRef := v1.OwnerReference{UID: bindingUID}

			// Named via the normal "<binding>-<role>" convention, not the hashed name used by the
			// real reserved crt-token-reader binding (which is never created in project namespaces).
			collidingRB := &rbacv1.RoleBinding{
				ObjectMeta: v1.ObjectMeta{
					Name:            "prtb1-crt-token-reader",
					Namespace:       projectNamespace,
					OwnerReferences: []v1.OwnerReference{ownerRef},
				},
				RoleRef: rbacv1.RoleRef{Kind: "Role", Name: "crt-token-reader"},
			}

			rbIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
				rbByOwnerIndex: func(obj interface{}) ([]string, error) {
					return rbByOwner(obj.(*rbacv1.RoleBinding))
				},
			})
			require.NoError(t, rbIndexer.Add(collidingRB))

			var deletedNames, createdNames []string
			rbClientMock := &rbacFakes.RoleBindingInterfaceMock{
				DeleteNamespacedFunc: func(namespace, name string, options *v1.DeleteOptions) error {
					deletedNames = append(deletedNames, name)
					return nil
				},
				CreateFunc: func(rb *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
					createdNames = append(createdNames, rb.Name)
					return rb, nil
				},
			}
			nsListerMock := &normanFakes.NamespaceListerMock{
				GetFunc: func(namespace, name string) (*corev1.Namespace, error) {
					return &corev1.Namespace{Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}}, nil
				},
			}

			m := &manager{
				rtLister: &fakes.RoleTemplateListerMock{
					GetFunc: func(namespace, name string) (*v3.RoleTemplate, error) {
						return &v3.RoleTemplate{ObjectMeta: v1.ObjectMeta{Name: name}, Rules: tt.rules}, nil
					},
				},
				rLister: &rbacFakes.RoleListerMock{
					GetFunc: func(namespace, name string) (*rbacv1.Role, error) {
						return nil, errors.NewNotFound(schema.GroupResource{}, name)
					},
				},
				rClient: &rbacFakes.RoleInterfaceMock{
					CreateFunc: func(r *rbacv1.Role) (*rbacv1.Role, error) { return r, nil },
				},
				rbIndexer:  rbIndexer,
				rbClient:   rbClientMock,
				nsLister:   nsListerMock,
				controller: "test",
			}

			err := m.grantManagementPlanePrivileges("crt-token-reader", map[string]string{"clusterregistrationtokens": "management.cattle.io"}, rbacv1.Subject{Name: "test-user"}, binding)
			require.NoError(t, err)

			if tt.rules == nil {
				assert.Contains(t, deletedNames, collidingRB.Name, "RoleBinding for a project RoleTemplate named crt-token-reader should be deleted once its RoleTemplate no longer grants access, not permanently orphaned in the project's backing namespace")
			} else {
				assert.NotContains(t, deletedNames, collidingRB.Name, "RoleBinding for a project RoleTemplate named crt-token-reader should not be deleted while its RoleTemplate still grants access")
				assert.NotContains(t, createdNames, collidingRB.Name, "RoleBinding already exists and matches desired state, so it should not be recreated (churn)")
			}
		})
	}
}
