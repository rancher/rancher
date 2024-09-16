package globalroles

import (
	"fmt"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"

	mgmt "github.com/rancher/rancher/pkg/apis/management.cattle.io"
	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	normanv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	rbacFakes "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1/fakes"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// using a subset of condition, because we don't need to check LastTransitionTime or Message
type reducedCondition struct {
	reason string
	status metav1.ConditionStatus
}

const generation int64 = 1

var (
	readPodPolicyRule = rbacv1.PolicyRule{
		Verbs:     []string{"get", "list", "watch"},
		APIGroups: []string{""},
		Resources: []string{"pods"},
	}

	readConfigPolicyRule = rbacv1.PolicyRule{
		Verbs:     []string{"get", "list", "watch"},
		APIGroups: []string{""},
		Resources: []string{"configmaps"},
	}
	adminPodPolicyRule = rbacv1.PolicyRule{
		Verbs:     []string{"*"},
		APIGroups: []string{""},
		Resources: []string{"pod"},
	}
	templatePolicyRule = rbacv1.PolicyRule{
		Verbs:     []string{"*"},
		APIGroups: []string{"*"},
		Resources: []string{"*"},
	}
	// templatePolicyRule gets transformed into catalogTemplatePolicyRule via reconcileCatalogRole
	catalogTemplatePolicyRule = rbacv1.PolicyRule{
		Verbs:     []string{"*"},
		APIGroups: []string{mgmt.GroupName},
		Resources: []string{
			catalogTemplateResourceRule,
			catalogTemplateVersionResourceRule,
		},
		NonResourceURLs: []string{},
	}

	defaultGR = v3.GlobalRole{
		Rules: []rbacv1.PolicyRule{
			readPodPolicyRule,
		},
	}
	readConfigCR = rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "clusterRole",
		},
		Rules: []rbacv1.PolicyRule{
			readConfigPolicyRule,
		},
	}
	readPodCR = rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "clusterRole",
		},
		Rules: []rbacv1.PolicyRule{
			readPodPolicyRule,
		},
	}

	catalogGR = v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "catalogRole",
		},
		Rules: []rbacv1.PolicyRule{
			templatePolicyRule,
		},
	}

	namespacedRulesGR = v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "namespacedRulesGR",
			UID:  "00000000",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "fake-kind",
			APIVersion: "fake-version",
		},
		NamespacedRules: map[string][]rbacv1.PolicyRule{
			"namespace1": {
				readPodPolicyRule,
				readConfigPolicyRule,
			},
			"namespace2": {
				adminPodPolicyRule,
				readPodPolicyRule,
			},
		},
	}
	namespacedRulesOwnerRef = metav1.OwnerReference{
		APIVersion: namespacedRulesGR.APIVersion,
		Kind:       namespacedRulesGR.Kind,
		Name:       namespacedRulesGR.Name,
		UID:        namespacedRulesGR.UID,
	}

	updatedNamespacedRulesGR = v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "namespacedRulesGR",
		},
		NamespacedRules: map[string][]rbacv1.PolicyRule{
			"namespace1": {
				readPodPolicyRule,
			},
			"namespace2": {
				adminPodPolicyRule,
				readPodPolicyRule,
				readConfigPolicyRule,
			},
		},
	}
)

type grTestStateChanges struct {
	t                   *testing.T
	createdRoles        map[string]*rbacv1.Role
	deletedRoles        map[string]struct{}
	createdClusterRoles map[string]*rbacv1.ClusterRole
}
type grTestState struct {
	nsCacheMock  *fake.MockNonNamespacedCacheInterface[*corev1.Namespace]
	rListerMock  *rbacFakes.RoleListerMock
	rClientMock  *rbacFakes.RoleInterfaceMock
	crListerMock *rbacFakes.ClusterRoleListerMock
	crClientMock *rbacFakes.ClusterRoleInterfaceMock
	counter      int
	stateChanges *grTestStateChanges
}

func TestReconcileGlobalRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		stateSetup      func(grTestState)
		stateAssertions func(grTestStateChanges)
		globalRole      *v3.GlobalRole
		wantError       bool
		condition       reducedCondition
		annotation      string
	}{
		{
			name: "no changes to clusterRole",
			stateSetup: func(state grTestState) {
				state.crListerMock.GetFunc = func(_, _ string) (*normanv1.ClusterRole, error) {
					return readPodCR.DeepCopy(), nil
				}
			},
			globalRole: defaultGR.DeepCopy(),
			wantError:  false,
			condition: reducedCondition{
				reason: ClusterRoleExists,
				status: metav1.ConditionTrue,
			},
		},
		{
			name: "clusterRole is updated",
			stateSetup: func(state grTestState) {
				state.crListerMock.GetFunc = func(_, _ string) (*normanv1.ClusterRole, error) {
					return readConfigCR.DeepCopy(), nil
				}
				state.crClientMock.UpdateFunc = func(cr *normanv1.ClusterRole) (*normanv1.ClusterRole, error) {
					state.stateChanges.createdClusterRoles[cr.Name] = cr
					return nil, nil
				}
			},
			stateAssertions: func(gtsc grTestStateChanges) {
				require.Len(gtsc.t, gtsc.createdClusterRoles, 1)
				cr, ok := gtsc.createdClusterRoles["clusterRole"]
				require.True(gtsc.t, ok)
				require.Equal(gtsc.t, &readPodCR, cr)
			},
			globalRole: defaultGR.DeepCopy(),
			wantError:  false,
			condition: reducedCondition{
				reason: ClusterRoleExists,
				status: metav1.ConditionTrue,
			},
		},
		{
			name: "update clusterRole fails",
			stateSetup: func(state grTestState) {
				state.crListerMock.GetFunc = func(_, _ string) (*normanv1.ClusterRole, error) {
					return readConfigCR.DeepCopy(), nil
				}
				state.crClientMock.UpdateFunc = func(cr *normanv1.ClusterRole) (*normanv1.ClusterRole, error) {
					return nil, fmt.Errorf("error")
				}
			},
			globalRole: defaultGR.DeepCopy(),
			wantError:  true,
			condition: reducedCondition{
				reason: FailedToUpdateClusterRole,
				status: metav1.ConditionFalse,
			},
		},
		{
			name: "create clusterRole fails",
			stateSetup: func(state grTestState) {
				state.crListerMock.GetFunc = func(_, _ string) (*normanv1.ClusterRole, error) {
					return nil, nil
				}
				state.crClientMock.CreateFunc = func(cr *normanv1.ClusterRole) (*normanv1.ClusterRole, error) {
					return nil, fmt.Errorf("error")
				}
			},
			globalRole: defaultGR.DeepCopy(),
			wantError:  true,
			condition: reducedCondition{
				reason: FailedToCreateClusterRole,
				status: metav1.ConditionFalse,
			},
		},
		{
			name: "clusterRole is created",
			stateSetup: func(state grTestState) {
				state.crListerMock.GetFunc = func(_, _ string) (*normanv1.ClusterRole, error) {
					return nil, nil
				}
				state.crClientMock.CreateFunc = func(cr *normanv1.ClusterRole) (*normanv1.ClusterRole, error) {
					state.stateChanges.createdClusterRoles[cr.Name] = cr
					return nil, nil
				}
			},
			stateAssertions: func(gtsc grTestStateChanges) {
				require.Len(gtsc.t, gtsc.createdClusterRoles, 1)
				cr, ok := gtsc.createdClusterRoles["cattle-globalrole-"]
				require.True(gtsc.t, ok)
				require.Equal(gtsc.t, readPodCR.Rules, cr.Rules)
			},
			globalRole: defaultGR.DeepCopy(),
			wantError:  false,
			condition: reducedCondition{
				reason: ClusterRoleExists,
				status: metav1.ConditionTrue,
			},
			annotation: "cattle-globalrole-",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			grLifecycle := globalRoleLifecycle{}
			state := setupTest(t)
			if test.stateSetup != nil {
				test.stateSetup(state)
			}
			grLifecycle.crLister = state.crListerMock
			grLifecycle.crClient = state.crClientMock

			err := grLifecycle.reconcileGlobalRole(test.globalRole)

			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if test.stateAssertions != nil {
				test.stateAssertions(*state.stateChanges)
			}
			if test.annotation != "" {
				require.Equal(t, test.annotation, test.globalRole.Annotations[crNameAnnotation])
			}
			// only 1 ClusterRole is created, so there should only ever be 1 condition
			require.Len(t, test.globalRole.Status.Conditions, 1)
			c := test.globalRole.Status.Conditions[0]
			rc := reducedCondition{
				reason: c.Reason,
				status: c.Status,
			}
			require.Equal(t, test.condition, rc)
			require.Equal(t, ClusterRoleExists, c.Type)
		})
	}
}

func TestReconcileCatalogRole(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		stateSetup      func(grTestState)
		stateAssertions func(grTestStateChanges)
		globalRole      *v3.GlobalRole
		wantError       bool
		condition       *reducedCondition
	}{
		{
			name: "no catalog role",
			globalRole: &v3.GlobalRole{
				Rules: []rbacv1.PolicyRule{
					catalogTemplatePolicyRule,
				},
			},
			wantError: false,
		},
		{
			name: "get role failed",
			stateSetup: func(state grTestState) {
				state.rListerMock.GetFunc = func(namespace, name string) (*rbacv1.Role, error) {
					return nil, fmt.Errorf("error")
				}
			},
			globalRole: catalogGR.DeepCopy(),
			wantError:  true,
			condition: &reducedCondition{
				reason: FailedToGetRole,
				status: metav1.ConditionFalse,
			},
		},
		{
			name: "create role failed",
			stateSetup: func(state grTestState) {
				state.rListerMock.GetFunc = func(namespace string, name string) (*rbacv1.Role, error) {
					return nil, apierrors.NewNotFound(schema.GroupResource{
						Group:    normanv1.RoleGroupVersionKind.Group,
						Resource: normanv1.RoleGroupVersionResource.Resource,
					}, "")
				}
				state.rClientMock.CreateFunc = func(in1 *rbacv1.Role) (*rbacv1.Role, error) {
					return nil, fmt.Errorf("error")
				}
			},
			globalRole: catalogGR.DeepCopy(),
			wantError:  true,
			condition: &reducedCondition{
				reason: FailedToCreateRole,
				status: metav1.ConditionFalse,
			},
		},
		{
			name: "create role succeeds",
			stateSetup: func(state grTestState) {
				state.rListerMock.GetFunc = func(namespace string, name string) (*rbacv1.Role, error) {
					return nil, apierrors.NewNotFound(schema.GroupResource{
						Group:    normanv1.RoleGroupVersionKind.Group,
						Resource: normanv1.RoleGroupVersionResource.Resource,
					}, "")
				}
				state.rClientMock.CreateFunc = func(role *rbacv1.Role) (*rbacv1.Role, error) {
					state.stateChanges.createdRoles[role.Name] = role
					return nil, nil
				}
			},
			stateAssertions: func(gtsc grTestStateChanges) {
				require.Len(gtsc.t, gtsc.createdRoles, 1)
				r, ok := gtsc.createdRoles["catalogRole-global-catalog"]
				require.True(gtsc.t, ok)
				require.Equal(gtsc.t, catalogTemplatePolicyRule, r.Rules[0])
			},
			globalRole: catalogGR.DeepCopy(),
			wantError:  false,
			condition: &reducedCondition{
				reason: CatalogRoleExists,
				status: metav1.ConditionTrue,
			},
		},
		{
			name: "update role failed",
			stateSetup: func(state grTestState) {
				state.rListerMock.GetFunc = func(namespace string, name string) (*rbacv1.Role, error) {
					return &rbacv1.Role{
						Rules: []rbacv1.PolicyRule{readPodPolicyRule},
					}, nil
				}
				state.rClientMock.UpdateFunc = func(_ *rbacv1.Role) (*rbacv1.Role, error) {
					return nil, fmt.Errorf("error")
				}
			},
			globalRole: catalogGR.DeepCopy(),
			wantError:  true,
			condition: &reducedCondition{
				reason: FailedToUpdateRole,
				status: metav1.ConditionFalse,
			},
		},
		{
			name: "update role succeeds",
			stateSetup: func(state grTestState) {
				state.rListerMock.GetFunc = func(namespace string, name string) (*rbacv1.Role, error) {
					return &rbacv1.Role{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-role",
						},
						Rules: []rbacv1.PolicyRule{readPodPolicyRule},
					}, nil
				}
				state.rClientMock.UpdateFunc = func(role *rbacv1.Role) (*rbacv1.Role, error) {
					state.stateChanges.createdRoles[role.Name] = role
					return nil, nil
				}
			},
			stateAssertions: func(gtsc grTestStateChanges) {
				require.Len(gtsc.t, gtsc.createdRoles, 1)
				r, ok := gtsc.createdRoles["test-role"]
				require.True(gtsc.t, ok)
				require.Equal(gtsc.t, catalogTemplatePolicyRule, r.Rules[0])
			},
			globalRole: catalogGR.DeepCopy(),
			wantError:  false,
			condition: &reducedCondition{
				reason: CatalogRoleExists,
				status: metav1.ConditionTrue,
			},
		},
		{
			name: "update role no changes",
			stateSetup: func(state grTestState) {
				state.rListerMock.GetFunc = func(namespace string, name string) (*rbacv1.Role, error) {
					return &rbacv1.Role{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-role",
						},
						Rules: []rbacv1.PolicyRule{readConfigPolicyRule},
					}, nil
				}
				state.rClientMock.UpdateFunc = func(role *rbacv1.Role) (*rbacv1.Role, error) {
					state.stateChanges.createdRoles[role.Name] = role
					return nil, nil
				}
			},
			stateAssertions: func(gtsc grTestStateChanges) {
				require.Len(gtsc.t, gtsc.createdRoles, 0)
			},
			// templatePolicyRule is a catalog rule, but it is covered by catalogTemplatePolicyRule
			// so the update does not need to happen since the user has all needed rules
			globalRole: &v3.GlobalRole{
				Rules: []rbacv1.PolicyRule{
					templatePolicyRule,
					catalogTemplatePolicyRule,
				},
			},
			wantError: false,
			condition: &reducedCondition{
				reason: CatalogRoleExists,
				status: metav1.ConditionTrue,
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			grLifecycle := globalRoleLifecycle{}
			state := setupTest(t)
			if test.stateSetup != nil {
				test.stateSetup(state)
			}
			grLifecycle.rLister = state.rListerMock
			grLifecycle.rClient = state.rClientMock

			err := grLifecycle.reconcileCatalogRole(test.globalRole)

			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if test.stateAssertions != nil {
				test.stateAssertions(*state.stateChanges)
			}
			if test.condition != nil {
				// only 1 ClusterRole is created, so there should only ever be 1 condition
				require.Len(t, test.globalRole.Status.Conditions, 1)
				c := test.globalRole.Status.Conditions[0]
				require.Equal(t, test.condition.reason, c.Reason)
				require.Equal(t, test.condition.status, c.Status)
				require.Equal(t, CatalogRoleExists, c.Type)
			}
		})
	}
}

func TestReconcileNamespacedRoles(t *testing.T) {
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

	tests := []struct {
		name            string
		stateSetup      func(grTestState)
		stateAssertions func(grTestStateChanges)
		globalRole      *v3.GlobalRole
		wantError       bool
		conditions      []reducedCondition
	}{
		{
			name: "getting namespace fails",
			stateSetup: func(state grTestState) {
				state.rListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.Role, error) {
					return nil, nil
				}

				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, fmt.Errorf("error"))
			},
			globalRole: namespacedRulesGR.DeepCopy(),
			wantError:  true,
			conditions: []reducedCondition{
				{
					reason: FailedToGetNamespace,
					status: metav1.ConditionFalse,
				},
			},
		},
		{
			name: "namespace is not found",
			stateSetup: func(state grTestState) {
				state.rListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.Role, error) {
					return nil, nil
				}

				nsNotFound := apierrors.NewNotFound(schema.GroupResource{
					Group:    normanv1.RoleGroupVersionKind.Group,
					Resource: normanv1.RoleGroupVersionResource.Resource,
				}, "")

				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nsNotFound)
			},
			globalRole: namespacedRulesGR.DeepCopy(),
			wantError:  false,
			conditions: []reducedCondition{
				{
					reason: NamespaceNotFound,
					status: metav1.ConditionFalse,
				},
			},
		},
		{
			name: "namespace is nil",
			stateSetup: func(state grTestState) {
				state.rListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.Role, error) {
					return nil, nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(nil, nil)
			},
			globalRole: namespacedRulesGR.DeepCopy(),
			wantError:  false,
			conditions: []reducedCondition{
				{
					reason: NamespaceNotFound,
					status: metav1.ConditionFalse,
				},
			},
		},
		{
			name: "getting role fails",
			stateSetup: func(state grTestState) {
				state.rListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.Role, error) {
					return nil, nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)
				state.rListerMock.GetFunc = func(_, _ string) (*rbacv1.Role, error) {
					return nil, fmt.Errorf("error")
				}
			},
			globalRole: namespacedRulesGR.DeepCopy(),
			wantError:  true,
			conditions: []reducedCondition{
				{
					reason: FailedToGetRole,
					status: metav1.ConditionFalse,
				},
			},
		},
		{
			name: "creating role fails",
			stateSetup: func(state grTestState) {
				state.rListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.Role, error) {
					return nil, nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)
				state.rListerMock.GetFunc = func(_, _ string) (*rbacv1.Role, error) {
					return nil, apierrors.NewNotFound(schema.GroupResource{
						Group:    normanv1.RoleGroupVersionKind.Group,
						Resource: normanv1.RoleGroupVersionResource.Resource,
					}, "")
				}
				state.rClientMock.CreateFunc = func(role *rbacv1.Role) (*rbacv1.Role, error) {
					return nil, fmt.Errorf("error")
				}
			},
			globalRole: namespacedRulesGR.DeepCopy(),
			wantError:  true,
			conditions: []reducedCondition{
				{
					reason: FailedToCreateRole,
					status: metav1.ConditionFalse,
				},
			},
		},
		{
			name: "created role already exists but get continuously fails",
			stateSetup: func(state grTestState) {
				state.rListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.Role, error) {
					return nil, nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)
				state.rListerMock.GetFunc = func(_, _ string) (*rbacv1.Role, error) {
					return nil, apierrors.NewNotFound(schema.GroupResource{
						Group:    normanv1.RoleGroupVersionKind.Group,
						Resource: normanv1.RoleGroupVersionResource.Resource,
					}, "")
				}
				state.rClientMock.CreateFunc = func(role *rbacv1.Role) (*rbacv1.Role, error) {
					return nil, apierrors.NewAlreadyExists(schema.GroupResource{
						Group:    normanv1.RoleGroupVersionKind.Group,
						Resource: normanv1.RoleGroupVersionResource.Resource,
					}, "")
				}
				state.rClientMock.UpdateFunc = func(role *rbacv1.Role) (*rbacv1.Role, error) {
					state.stateChanges.createdRoles[role.Name] = role
					role.UID = ""
					return role, nil
				}
			},
			globalRole: namespacedRulesGR.DeepCopy(),
			wantError:  true,
			conditions: []reducedCondition{
				{
					reason: FailedToGetRole,
					status: metav1.ConditionFalse,
				},
			},
		},
		{
			// It's possible that a user can create the invalid role in the middle of the reconcile
			// In that case, the first attempt to get the role fails. Then the reconcile function attempts to
			// create the role and finds that it already exists. It gets the new role and checks that it is valid
			name: "role gets created incorrectly mid reconcile",
			stateSetup: func(state grTestState) {
				state.rListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.Role, error) {
					return nil, nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)
				state.rListerMock.GetFunc = func(namespace, _ string) (*rbacv1.Role, error) {
					// counter == 0 means that the create hasn't happened and should fail
					// counter == 1 means that the create has occurred and should return an incorrect role
					if state.counter == 0 {
						return nil, apierrors.NewNotFound(schema.GroupResource{
							Group:    normanv1.RoleGroupVersionKind.Group,
							Resource: normanv1.RoleGroupVersionResource.Resource,
						}, "")
					} else {
						state.counter = 0
						role := &rbacv1.Role{}
						if namespace == "namespace1" {
							role.ObjectMeta = metav1.ObjectMeta{
								Name:      "namespacedRulesGR-namespace1",
								Namespace: "namespace1",
							}
							role.Rules = []rbacv1.PolicyRule{
								readPodPolicyRule,
								readConfigPolicyRule,
							}
						} else if namespace == "namespace2" {
							role.ObjectMeta = metav1.ObjectMeta{
								Name:      "namespacedRulesGR-namespace2",
								Namespace: "namespace2",
								Labels:    map[string]string{grOwnerLabel: "badGR"},
							}
							role.Rules = []rbacv1.PolicyRule{
								adminPodPolicyRule,
								readPodPolicyRule,
							}
						}
						return role, nil
					}
				}
				state.rClientMock.CreateFunc = func(role *rbacv1.Role) (*rbacv1.Role, error) {
					state.counter = 1
					return nil, apierrors.NewAlreadyExists(schema.GroupResource{
						Group:    normanv1.RoleGroupVersionKind.Group,
						Resource: normanv1.RoleGroupVersionResource.Resource,
					}, "")
				}
				state.rClientMock.UpdateFunc = func(role *rbacv1.Role) (*rbacv1.Role, error) {
					state.stateChanges.createdRoles[role.Name] = role
					role.UID = ""
					return role, nil
				}
			},
			stateAssertions: func(gtsc grTestStateChanges) {
				require.Len(gtsc.t, gtsc.createdRoles, 2)

				role, ok := gtsc.createdRoles["namespacedRulesGR-namespace1"]
				require.True(gtsc.t, ok)
				require.Equal(gtsc.t, "namespace1", role.Namespace)
				require.Equal(gtsc.t, "namespacedRulesGR", role.Labels[grOwnerLabel])
				require.Len(gtsc.t, role.Rules, 2)
				require.Equal(gtsc.t, readPodPolicyRule, role.Rules[0])
				require.Equal(gtsc.t, readConfigPolicyRule, role.Rules[1])

				role, ok = gtsc.createdRoles["namespacedRulesGR-namespace2"]
				require.True(gtsc.t, ok)
				require.Equal(gtsc.t, "namespace2", role.Namespace)
				require.Equal(gtsc.t, "namespacedRulesGR", role.Labels[grOwnerLabel])
				require.Len(gtsc.t, role.Rules, 2)
				require.Equal(gtsc.t, adminPodPolicyRule, role.Rules[0])
				require.Equal(gtsc.t, readPodPolicyRule, role.Rules[1])
			},
			globalRole: namespacedRulesGR.DeepCopy(),
			wantError:  false,
			conditions: []reducedCondition{
				{
					reason: NamespacedRuleRoleExists,
					status: metav1.ConditionTrue,
				},
			},
		},
		{
			name: "create roles successfully",
			stateSetup: func(state grTestState) {
				state.rListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.Role, error) {
					return nil, nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)
				state.rListerMock.GetFunc = func(namespace string, name string) (*rbacv1.Role, error) {
					return nil, apierrors.NewNotFound(schema.GroupResource{
						Group:    normanv1.RoleGroupVersionKind.Group,
						Resource: normanv1.RoleGroupVersionResource.Resource,
					}, "")
				}
				state.rClientMock.CreateFunc = func(role *rbacv1.Role) (*rbacv1.Role, error) {
					state.stateChanges.createdRoles[role.Name] = role
					role.UID = ""
					return role, nil
				}
			},
			stateAssertions: func(gtsc grTestStateChanges) {
				require.Len(gtsc.t, gtsc.createdRoles, 2)

				role, ok := gtsc.createdRoles["namespacedRulesGR-namespace1"]
				require.True(gtsc.t, ok)
				require.Equal(gtsc.t, "namespace1", role.Namespace)
				require.Equal(gtsc.t, "namespacedRulesGR", role.Labels[grOwnerLabel])
				require.Len(gtsc.t, role.Rules, 2)
				require.Equal(gtsc.t, readPodPolicyRule, role.Rules[0])
				require.Equal(gtsc.t, readConfigPolicyRule, role.Rules[1])
				require.Equal(gtsc.t, namespacedRulesOwnerRef, role.OwnerReferences[0])

				role, ok = gtsc.createdRoles["namespacedRulesGR-namespace2"]
				require.True(gtsc.t, ok)
				require.Equal(gtsc.t, "namespace2", role.Namespace)
				require.Equal(gtsc.t, "namespacedRulesGR", role.Labels[grOwnerLabel])
				require.Len(gtsc.t, role.Rules, 2)
				require.Equal(gtsc.t, adminPodPolicyRule, role.Rules[0])
				require.Equal(gtsc.t, readPodPolicyRule, role.Rules[1])
				require.Equal(gtsc.t, namespacedRulesOwnerRef, role.OwnerReferences[0])
			},
			globalRole: namespacedRulesGR.DeepCopy(),
			wantError:  false,
			conditions: []reducedCondition{
				{
					reason: NamespacedRuleRoleExists,
					status: metav1.ConditionTrue,
				},
			},
		},
		{
			name: "create role in terminating namespace",
			stateSetup: func(state grTestState) {
				state.rListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.Role, error) {
					return nil, nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(terminatingNamespace, nil)

				state.rListerMock.GetFunc = func(namespace string, name string) (*rbacv1.Role, error) {
					return nil, apierrors.NewNotFound(schema.GroupResource{
						Group:    normanv1.RoleGroupVersionKind.Group,
						Resource: normanv1.RoleGroupVersionResource.Resource,
					}, "")
				}
				state.rClientMock.CreateFunc = func(role *rbacv1.Role) (*rbacv1.Role, error) {
					state.stateChanges.createdRoles[role.Name] = role
					role.UID = ""
					return role, nil
				}
			},
			stateAssertions: func(gtsc grTestStateChanges) {
				require.Len(gtsc.t, gtsc.createdRoles, 0)
			},
			globalRole: namespacedRulesGR.DeepCopy(),
			wantError:  false,
			conditions: []reducedCondition{
				{
					reason: NamespaceTerminating,
					status: metav1.ConditionFalse,
				},
			},
		},
		{
			name: "some roles have errors but rest get created",
			stateSetup: func(state grTestState) {
				state.rListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.Role, error) {
					return nil, nil
				}
				first := state.nsCacheMock.EXPECT().Get(gomock.Any()).Return(activeNamespace, fmt.Errorf("error"))
				second := state.nsCacheMock.EXPECT().Get(gomock.Any()).Return(activeNamespace, nil)
				gomock.InOrder(first, second)

				state.rListerMock.GetFunc = func(_, _ string) (*rbacv1.Role, error) {
					return nil, apierrors.NewNotFound(schema.GroupResource{
						Group:    normanv1.RoleGroupVersionKind.Group,
						Resource: normanv1.RoleGroupVersionResource.Resource,
					}, "")
				}
				state.rClientMock.CreateFunc = func(role *rbacv1.Role) (*rbacv1.Role, error) {
					state.stateChanges.createdRoles[role.Name] = role
					role.UID = ""
					return role, nil
				}
			},
			stateAssertions: func(gtsc grTestStateChanges) {
				// The second role should be created despite the first getting an error
				// Because the order is not guaranteed, we can't assert any info on the
				// created role, just that it exists
				require.Len(gtsc.t, gtsc.createdRoles, 1)
			},
			globalRole: namespacedRulesGR.DeepCopy(),
			wantError:  true,
			conditions: []reducedCondition{
				{
					reason: NamespacedRuleRoleExists,
					status: metav1.ConditionTrue,
				},
				{
					reason: FailedToGetNamespace,
					status: metav1.ConditionFalse,
				},
			},
		},
		{
			name: "update an existing role with rule and label changes",
			stateSetup: func(state grTestState) {
				state.rListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.Role, error) {
					return nil, nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)

				state.rListerMock.GetFunc = func(namespace string, _ string) (*rbacv1.Role, error) {
					role := &rbacv1.Role{}
					if namespace == "namespace1" {
						role.ObjectMeta = metav1.ObjectMeta{
							Name:      "namespacedRulesGR-namespace1",
							Namespace: "namespace1",
						}
						role.Rules = []rbacv1.PolicyRule{
							readPodPolicyRule,
							readConfigPolicyRule,
						}
					} else if namespace == "namespace2" {
						role.ObjectMeta = metav1.ObjectMeta{
							Name:      "namespacedRulesGR-namespace2",
							Namespace: "namespace2",
							Labels:    map[string]string{grOwnerLabel: "badGR"},
						}
						role.Rules = []rbacv1.PolicyRule{
							adminPodPolicyRule,
							readPodPolicyRule,
						}
					}

					return role, nil
				}
				state.rClientMock.UpdateFunc = func(role *rbacv1.Role) (*rbacv1.Role, error) {
					state.stateChanges.createdRoles[role.Name] = role
					role.UID = ""
					return role, nil
				}
			},
			stateAssertions: func(gtsc grTestStateChanges) {
				require.Len(gtsc.t, gtsc.createdRoles, 2)

				role, ok := gtsc.createdRoles["namespacedRulesGR-namespace1"]
				require.True(gtsc.t, ok)
				require.Equal(gtsc.t, "namespace1", role.Namespace)
				require.Len(gtsc.t, role.Rules, 1)
				require.Equal(gtsc.t, readPodPolicyRule, role.Rules[0])
				require.Equal(gtsc.t, "namespacedRulesGR", role.Labels[grOwnerLabel])

				role, ok = gtsc.createdRoles["namespacedRulesGR-namespace2"]
				require.True(gtsc.t, ok)
				require.Equal(gtsc.t, "namespace2", role.Namespace)
				require.Len(gtsc.t, role.Rules, 3)
				require.Equal(gtsc.t, adminPodPolicyRule, role.Rules[0])
				require.Equal(gtsc.t, readPodPolicyRule, role.Rules[1])
				require.Equal(gtsc.t, readConfigPolicyRule, role.Rules[2])
				require.Equal(gtsc.t, "namespacedRulesGR", role.Labels[grOwnerLabel])
			},
			globalRole: updatedNamespacedRulesGR.DeepCopy(),
			wantError:  false,
			conditions: []reducedCondition{
				{
					reason: NamespacedRuleRoleExists,
					status: metav1.ConditionTrue,
				},
			},
		},
		{
			name: "update an existing role no changes",
			stateSetup: func(state grTestState) {
				state.rListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.Role, error) {
					return nil, nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)

				state.rListerMock.GetFunc = func(namespace string, _ string) (*rbacv1.Role, error) {
					role := &rbacv1.Role{}
					if namespace == "namespace1" {
						role.ObjectMeta = metav1.ObjectMeta{
							Name:      "namespacedRulesGR-namespace1",
							Namespace: "namespace1",
							Labels: map[string]string{
								grOwnerLabel: "namespacedRulesGR",
							},
						}
						role.Rules = []rbacv1.PolicyRule{
							readPodPolicyRule,
							readConfigPolicyRule,
						}
					} else if namespace == "namespace2" {
						role.ObjectMeta = metav1.ObjectMeta{
							Name:      "namespacedRulesGR-namespace2",
							Namespace: "namespace2",
							Labels: map[string]string{
								grOwnerLabel: "namespacedRulesGR",
							},
						}
						role.Rules = []rbacv1.PolicyRule{
							adminPodPolicyRule,
							readPodPolicyRule,
						}
					}

					return role, nil
				}
				state.rClientMock.UpdateFunc = func(role *rbacv1.Role) (*rbacv1.Role, error) {
					state.stateChanges.createdRoles[role.Name] = role
					return nil, nil
				}
			},
			stateAssertions: func(gtsc grTestStateChanges) {
				require.Len(gtsc.t, gtsc.createdRoles, 0)
			},
			globalRole: namespacedRulesGR.DeepCopy(),
			wantError:  false,
			conditions: []reducedCondition{
				{
					reason: NamespacedRuleRoleExists,
					status: metav1.ConditionTrue,
				},
			},
		},
		{
			name: "update role fails",
			stateSetup: func(state grTestState) {
				state.rListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.Role, error) {
					return nil, nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)

				state.rListerMock.GetFunc = func(namespace string, _ string) (*rbacv1.Role, error) {
					role := &rbacv1.Role{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "namespacedRulesGR-namespace1",
							Namespace: "namespace1",
						},
						Rules: []rbacv1.PolicyRule{},
					}

					return role, nil
				}
				state.rClientMock.UpdateFunc = func(role *rbacv1.Role) (*rbacv1.Role, error) {
					return nil, fmt.Errorf("error")
				}
			},
			globalRole: namespacedRulesGR.DeepCopy(),
			wantError:  true,
			conditions: []reducedCondition{
				{
					reason: FailedToUpdateRole,
					status: metav1.ConditionFalse,
				},
			},
		},
		{
			name: "remove roles that falsely claim to be owned by GR after create",
			stateSetup: func(state grTestState) {
				state.rListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.Role, error) {
					roles := []*rbacv1.Role{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "deleted-role-1",
								UID:  "2222",
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "kept-role-1",
								UID:  "1111",
							},
						},
					}

					return roles, nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)

				state.rListerMock.GetFunc = func(namespace string, _ string) (*rbacv1.Role, error) {
					return nil, apierrors.NewNotFound(schema.GroupResource{
						Group:    normanv1.RoleGroupVersionKind.Group,
						Resource: normanv1.RoleGroupVersionResource.Resource,
					}, "")
				}
				state.rClientMock.CreateFunc = func(role *rbacv1.Role) (*rbacv1.Role, error) {
					state.stateChanges.createdRoles[role.Name] = role
					role.UID = "1111"
					return role, nil
				}
				state.rClientMock.DeleteNamespacedFunc = func(_, name string, _ *metav1.DeleteOptions) error {
					state.stateChanges.deletedRoles[name] = struct{}{}
					return nil
				}
			},
			stateAssertions: func(gtsc grTestStateChanges) {
				require.Len(gtsc.t, gtsc.createdRoles, 2)

				role, ok := gtsc.createdRoles["namespacedRulesGR-namespace1"]
				require.True(gtsc.t, ok)
				require.Equal(gtsc.t, "namespace1", role.Namespace)
				require.Equal(gtsc.t, "namespacedRulesGR", role.Labels[grOwnerLabel])
				require.Len(gtsc.t, role.Rules, 2)
				require.Equal(gtsc.t, readPodPolicyRule, role.Rules[0])
				require.Equal(gtsc.t, readConfigPolicyRule, role.Rules[1])
				require.Equal(gtsc.t, namespacedRulesOwnerRef, role.OwnerReferences[0])

				role, ok = gtsc.createdRoles["namespacedRulesGR-namespace2"]
				require.True(gtsc.t, ok)
				require.Equal(gtsc.t, "namespace2", role.Namespace)
				require.Equal(gtsc.t, "namespacedRulesGR", role.Labels[grOwnerLabel])
				require.Len(gtsc.t, role.Rules, 2)
				require.Equal(gtsc.t, adminPodPolicyRule, role.Rules[0])
				require.Equal(gtsc.t, readPodPolicyRule, role.Rules[1])
				require.Equal(gtsc.t, namespacedRulesOwnerRef, role.OwnerReferences[0])

				require.Contains(gtsc.t, gtsc.deletedRoles, "deleted-role-1")
			},
			globalRole: namespacedRulesGR.DeepCopy(),
			wantError:  false,
			conditions: []reducedCondition{
				{
					reason: NamespacedRuleRoleExists,
					status: metav1.ConditionTrue,
				},
			},
		},
		{
			name: "remove invalid role despite terminating namespace",
			stateSetup: func(state grTestState) {
				state.rListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.Role, error) {
					roles := []*rbacv1.Role{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "deleted-role-1",
								UID:  "1111",
							},
						},
					}

					return roles, nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(terminatingNamespace, nil)

				state.rListerMock.GetFunc = func(namespace string, _ string) (*rbacv1.Role, error) {
					return nil, apierrors.NewNotFound(schema.GroupResource{
						Group:    normanv1.RoleGroupVersionKind.Group,
						Resource: normanv1.RoleGroupVersionResource.Resource,
					}, "")
				}
				state.rClientMock.DeleteNamespacedFunc = func(_, name string, _ *metav1.DeleteOptions) error {
					state.stateChanges.deletedRoles[name] = struct{}{}
					return nil
				}
			},
			stateAssertions: func(gtsc grTestStateChanges) {
				require.Len(gtsc.t, gtsc.deletedRoles, 1)
				require.Contains(gtsc.t, gtsc.deletedRoles, "deleted-role-1")
			},
			globalRole: namespacedRulesGR.DeepCopy(),
			wantError:  false,
			conditions: []reducedCondition{
				{
					reason: NamespaceTerminating,
					status: metav1.ConditionFalse,
				},
			},
		},
		{
			name: "delete Role fails",
			stateSetup: func(state grTestState) {
				state.rListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.Role, error) {
					roles := []*rbacv1.Role{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "deleted-role-1",
								UID:  "2222",
							},
						},
					}

					return roles, nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)

				state.rListerMock.GetFunc = func(namespace string, _ string) (*rbacv1.Role, error) {
					return nil, nil
				}
				state.rClientMock.DeleteNamespacedFunc = func(_, name string, _ *metav1.DeleteOptions) error {
					return fmt.Errorf("error")
				}
			},
			globalRole: namespacedRulesGR.DeepCopy(),
			wantError:  true,
		},
		{
			name: "remove roles that falsely claim to be owned by GR after update",
			stateSetup: func(state grTestState) {
				state.rListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.Role, error) {
					roles := []*rbacv1.Role{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "kept-role-1",
								UID:  "1111",
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "kept-role-2",
								UID:  "2222",
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "deleted-role-1",
								UID:  "3333",
							},
						},
					}

					return roles, nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)

				state.rListerMock.GetFunc = func(namespace string, _ string) (*rbacv1.Role, error) {
					role := &rbacv1.Role{}
					if namespace == "namespace1" {
						role.ObjectMeta = metav1.ObjectMeta{
							Name:      "namespacedRulesGR-namespace1",
							Namespace: "namespace1",
						}
						role.Rules = []rbacv1.PolicyRule{
							readPodPolicyRule,
							readConfigPolicyRule,
						}
						role.UID = "1111"
					} else if namespace == "namespace2" {
						role.ObjectMeta = metav1.ObjectMeta{
							Name:      "namespacedRulesGR-namespace2",
							Namespace: "namespace2",
							Labels:    map[string]string{grOwnerLabel: "badGR"},
						}
						role.Rules = []rbacv1.PolicyRule{
							adminPodPolicyRule,
							readPodPolicyRule,
						}
						role.UID = "2222"
					}

					return role, nil
				}
				state.rClientMock.UpdateFunc = func(role *rbacv1.Role) (*rbacv1.Role, error) {
					state.stateChanges.createdRoles[role.Name] = role
					return role, nil
				}
				state.rClientMock.DeleteNamespacedFunc = func(_, name string, _ *metav1.DeleteOptions) error {
					state.stateChanges.deletedRoles[name] = struct{}{}
					return nil
				}
			},
			stateAssertions: func(gtsc grTestStateChanges) {
				require.Len(gtsc.t, gtsc.createdRoles, 2)

				role, ok := gtsc.createdRoles["namespacedRulesGR-namespace1"]
				require.True(gtsc.t, ok)
				require.Equal(gtsc.t, "namespace1", role.Namespace)
				require.Len(gtsc.t, role.Rules, 1)
				require.Equal(gtsc.t, readPodPolicyRule, role.Rules[0])
				require.Equal(gtsc.t, "namespacedRulesGR", role.Labels[grOwnerLabel])

				role, ok = gtsc.createdRoles["namespacedRulesGR-namespace2"]
				require.True(gtsc.t, ok)
				require.Equal(gtsc.t, "namespace2", role.Namespace)
				require.Len(gtsc.t, role.Rules, 3)
				require.Equal(gtsc.t, adminPodPolicyRule, role.Rules[0])
				require.Equal(gtsc.t, readPodPolicyRule, role.Rules[1])
				require.Equal(gtsc.t, readConfigPolicyRule, role.Rules[2])
				require.Equal(gtsc.t, "namespacedRulesGR", role.Labels[grOwnerLabel])

				require.Contains(gtsc.t, gtsc.deletedRoles, "deleted-role-1")
			},
			globalRole: updatedNamespacedRulesGR.DeepCopy(),
			wantError:  false,
			conditions: []reducedCondition{
				{
					reason: NamespacedRuleRoleExists,
					status: metav1.ConditionTrue,
				},
			},
		},
		{
			name: "listing existing roles fails",
			stateSetup: func(state grTestState) {
				state.rListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.Role, error) {
					return nil, fmt.Errorf("error")
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)

				state.rListerMock.GetFunc = func(_, _ string) (*rbacv1.Role, error) {
					return nil, nil
				}
			},
			globalRole: namespacedRulesGR.DeepCopy(),
			wantError:  true,
		},
		{
			name: "roles that should exist with no changes don't get deleted",
			stateSetup: func(state grTestState) {
				state.rListerMock.ListFunc = func(_ string, _ labels.Selector) ([]*rbacv1.Role, error) {
					roles := []*rbacv1.Role{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "kept-role-1",
								UID:  "1111",
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "kept-role-2",
								UID:  "2222",
							},
						},
					}

					return roles, nil
				}
				state.nsCacheMock.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)

				state.rListerMock.GetFunc = func(namespace string, _ string) (*rbacv1.Role, error) {
					role := &rbacv1.Role{}
					if namespace == "namespace1" {
						role.ObjectMeta = metav1.ObjectMeta{
							Name:      "namespacedRulesGR-namespace1",
							Namespace: "namespace1",
							Labels:    map[string]string{grOwnerLabel: "namespacedRulesGR"},
						}
						role.Rules = []rbacv1.PolicyRule{
							readPodPolicyRule,
							readConfigPolicyRule,
						}
						role.UID = "1111"
					} else if namespace == "namespace2" {
						role.ObjectMeta = metav1.ObjectMeta{
							Name:      "namespacedRulesGR-namespace2",
							Namespace: "namespace2",
							Labels:    map[string]string{grOwnerLabel: "namespacedRulesGR"},
						}
						role.Rules = []rbacv1.PolicyRule{
							adminPodPolicyRule,
							readPodPolicyRule,
						}
						role.UID = "2222"
					}

					return role, nil
				}
				state.rClientMock.DeleteNamespacedFunc = func(_, name string, _ *metav1.DeleteOptions) error {
					state.stateChanges.deletedRoles[name] = struct{}{}
					return nil
				}
			},
			stateAssertions: func(gtsc grTestStateChanges) {
				require.Len(gtsc.t, gtsc.deletedRoles, 0)
			},
			globalRole: namespacedRulesGR.DeepCopy(),
			wantError:  false,
			conditions: []reducedCondition{
				{
					reason: NamespacedRuleRoleExists,
					status: metav1.ConditionTrue,
				},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			grLifecycle := globalRoleLifecycle{}
			state := setupTest(t)
			if test.stateSetup != nil {
				test.stateSetup(state)
			}
			grLifecycle.nsCache = state.nsCacheMock
			grLifecycle.rLister = state.rListerMock
			grLifecycle.rClient = state.rClientMock

			err := grLifecycle.reconcileNamespacedRoles(test.globalRole)

			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if test.stateAssertions != nil {
				test.stateAssertions(*state.stateChanges)
			}
			if test.conditions != nil {
				// All tests are done with 2 NamespacedRules
				require.Len(t, test.globalRole.Status.Conditions, 2)
				for _, c := range test.globalRole.Status.Conditions {
					rc := reducedCondition{
						reason: c.Reason,
						status: c.Status,
					}
					require.Contains(t, test.conditions, rc)
					require.Equal(t, NamespacedRuleRoleExists, c.Type)
				}
			}
		})
	}
}

func setupTest(t *testing.T) grTestState {
	ctrl := gomock.NewController(t)
	nsCacheMock := fake.NewMockNonNamespacedCacheInterface[*corev1.Namespace](ctrl)
	rListerMock := rbacFakes.RoleListerMock{}
	rClientMock := rbacFakes.RoleInterfaceMock{}
	crListerMock := rbacFakes.ClusterRoleListerMock{}
	crClientMock := rbacFakes.ClusterRoleInterfaceMock{}

	stateChanges := grTestStateChanges{
		t:                   t,
		createdRoles:        map[string]*rbacv1.Role{},
		deletedRoles:        map[string]struct{}{},
		createdClusterRoles: map[string]*rbacv1.ClusterRole{},
	}
	state := grTestState{
		nsCacheMock:  nsCacheMock,
		rListerMock:  &rListerMock,
		rClientMock:  &rClientMock,
		crListerMock: &crListerMock,
		crClientMock: &crClientMock,
		stateChanges: &stateChanges,
	}
	return state
}

func TestSetGRAsInProgress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		oldGR        *v3.GlobalRole
		updateReturn error
		wantError    bool
	}{
		{
			name: "update gr status to InProgress",
			oldGR: &v3.GlobalRole{
				Status: mgmtv3.GlobalRoleStatus{
					Summary: SummaryCompleted,
					Conditions: []metav1.Condition{
						{
							Type:   "test",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			updateReturn: nil,
			wantError:    false,
		},
		{
			name: "update gr with empty status to InProgress",
			oldGR: &v3.GlobalRole{
				Status: mgmtv3.GlobalRoleStatus{},
			},
			updateReturn: nil,
			wantError:    false,
		},
		{
			name:         "update gr with nil status to InProgress",
			oldGR:        &v3.GlobalRole{},
			updateReturn: nil,
			wantError:    false,
		},
		{
			name:         "update gr fails",
			oldGR:        &v3.GlobalRole{},
			updateReturn: fmt.Errorf("error"),
			wantError:    true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			grLifecycle := globalRoleLifecycle{}
			ctrl := gomock.NewController(t)

			grClientMock := fake.NewMockNonNamespacedControllerInterface[*v3.GlobalRole, *v3.GlobalRoleList](ctrl)
			var updatedGR *v3.GlobalRole
			grClientMock.EXPECT().UpdateStatus(gomock.Any()).AnyTimes().DoAndReturn(
				func(gr *v3.GlobalRole) (*v3.GlobalRole, error) {
					updatedGR = gr
					return updatedGR, test.updateReturn
				},
			)
			grLifecycle.grClient = grClientMock

			err := grLifecycle.setGRAsInProgress(test.oldGR)
			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Empty(t, updatedGR.Status.Conditions)
			require.Equal(t, SummaryInProgress, updatedGR.Status.Summary)
		})
	}
}

func TestSetGRAsCompleted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		gr           *v3.GlobalRole
		summary      string
		updateReturn error
		wantError    bool
	}{
		{
			name: "gr with a met condition is Completed",
			gr: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Generation: generation,
				},
				Status: mgmtv3.GlobalRoleStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "test1",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			summary:      SummaryCompleted,
			updateReturn: nil,
			wantError:    false,
		},
		{
			name: "gr with multiple met conditions is Completed",
			gr: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Generation: generation,
				},
				Status: mgmtv3.GlobalRoleStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "test1",
							Status: metav1.ConditionTrue,
						},
						{
							Type:   "test2",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			summary:      SummaryCompleted,
			updateReturn: nil,
			wantError:    false,
		},
		{
			name: "gr with no conditions is Completed",
			gr: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Generation: generation,
				},
				Status: mgmtv3.GlobalRoleStatus{
					Conditions: []metav1.Condition{},
				},
			},
			summary:      SummaryCompleted,
			updateReturn: nil,
			wantError:    false,
		},
		{
			name: "gr with nil status is Completed",
			gr: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Generation: generation,
				},
			},
			summary:      SummaryCompleted,
			updateReturn: nil,
			wantError:    false,
		},
		{
			name: "gr with one unmet and one met condition is Error",
			gr: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Generation: generation,
				},
				Status: mgmtv3.GlobalRoleStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "test1",
							Status: metav1.ConditionTrue,
						},
						{
							Type:   "test2",
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			summary:      SummaryError,
			updateReturn: nil,
			wantError:    false,
		},
		{
			name: "gr with multiple unmet conditions is Error",
			gr: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Generation: generation,
				},
				Status: mgmtv3.GlobalRoleStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "test1",
							Status: metav1.ConditionFalse,
						},
						{
							Type:   "test2",
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			summary:      SummaryError,
			updateReturn: nil,
			wantError:    false,
		},
		{
			name: "gr with unknown conditions is Error",
			gr: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Generation: generation,
				},
				Status: mgmtv3.GlobalRoleStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "test1",
							Status: metav1.ConditionUnknown,
						},
						{
							Type:   "test2",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			summary:      SummaryError,
			updateReturn: nil,
			wantError:    false,
		},
		{
			name: "update gr fails",
			gr: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Generation: generation,
				},
			},
			updateReturn: fmt.Errorf("error"),
			wantError:    true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			grLifecycle := globalRoleLifecycle{}
			ctrl := gomock.NewController(t)
			grClientMock := fake.NewMockNonNamespacedControllerInterface[*v3.GlobalRole, *v3.GlobalRoleList](ctrl)
			var updatedGR *v3.GlobalRole
			grClientMock.EXPECT().UpdateStatus(gomock.Any()).AnyTimes().DoAndReturn(
				func(gr *v3.GlobalRole) (*v3.GlobalRole, error) {
					updatedGR = gr
					return nil, test.updateReturn
				},
			)

			grLifecycle.grClient = grClientMock
			err := grLifecycle.setGRAsCompleted(test.gr)
			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if test.summary != "" {
				require.Equal(t, test.summary, updatedGR.Status.Summary)
			}
			require.Equal(t, generation, updatedGR.Status.ObservedGeneration)
		})
	}
}

func TestSetGRAsTerminating(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		oldGR        *v3.GlobalRole
		updateReturn error
		wantError    bool
	}{
		{
			name: "update gr status to Terminating",
			oldGR: &v3.GlobalRole{
				Status: mgmtv3.GlobalRoleStatus{
					Summary: SummaryCompleted,
					Conditions: []metav1.Condition{
						{
							Type:   "test",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			updateReturn: nil,
			wantError:    false,
		},
		{
			name: "update gr with empty status to Terminating",
			oldGR: &v3.GlobalRole{
				Status: mgmtv3.GlobalRoleStatus{},
			},
			updateReturn: nil,
			wantError:    false,
		},
		{
			name:         "update gr with nil status to Terminating",
			oldGR:        &v3.GlobalRole{},
			updateReturn: nil,
			wantError:    false,
		},
		{
			name:         "update gr fails",
			oldGR:        &v3.GlobalRole{},
			updateReturn: fmt.Errorf("error"),
			wantError:    true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			grLifecycle := globalRoleLifecycle{}
			ctrl := gomock.NewController(t)

			grClientMock := fake.NewMockNonNamespacedControllerInterface[*v3.GlobalRole, *v3.GlobalRoleList](ctrl)
			var updatedGR *v3.GlobalRole
			grClientMock.EXPECT().UpdateStatus(gomock.Any()).AnyTimes().DoAndReturn(
				func(gr *v3.GlobalRole) (*v3.GlobalRole, error) {
					updatedGR = gr
					return updatedGR, test.updateReturn
				},
			)
			grLifecycle.grClient = grClientMock

			err := grLifecycle.setGRAsTerminating(test.oldGR)
			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Empty(t, updatedGR.Status.Conditions)
			require.Equal(t, SummaryTerminating, updatedGR.Status.Summary)
		})
	}
}
