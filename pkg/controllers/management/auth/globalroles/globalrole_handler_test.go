package globalroles

import (
	"fmt"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/status"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

	defaultGR = v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default-globalRole",
		},
		Rules: []rbacv1.PolicyRule{
			readPodPolicyRule,
		},
	}
	readConfigCR = rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "clusterRole",
			Labels: map[string]string{
				"authz.management.cattle.io/globalrole": "true",
				"authz.management.cattle.io/gr-owner":   defaultGR.Name,
			},
		},
		Rules: []rbacv1.PolicyRule{
			readConfigPolicyRule,
		},
	}
	readPodCR = rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "clusterRole",
			Labels: map[string]string{
				"authz.management.cattle.io/globalrole": "true",
				"authz.management.cattle.io/gr-owner":   defaultGR.Name,
			},
		},
		Rules: []rbacv1.PolicyRule{
			readPodPolicyRule,
		},
	}
	missingLabelCR = rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "clusterRole",
			Labels: map[string]string{
				"authz.management.cattle.io/globalrole": "true",
			},
		},
		Rules: []rbacv1.PolicyRule{
			readPodPolicyRule,
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
)

func TestReconcileGlobalRole(t *testing.T) {
	t.Parallel()

	type controllers struct {
		crController *fake.MockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList]
		crCache      *fake.MockNonNamespacedCacheInterface[*rbacv1.ClusterRole]
	}
	tests := []struct {
		name             string
		setupControllers func(controllers)
		globalRole       *v3.GlobalRole
		wantError        bool
		condition        reducedCondition
		annotation       string
	}{
		{
			name: "no changes to clusterRole",
			setupControllers: func(c controllers) {
				c.crCache.EXPECT().Get(gomock.Any()).Return(readPodCR.DeepCopy(), nil)
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
			setupControllers: func(c controllers) {
				c.crCache.EXPECT().Get(gomock.Any()).Return(readConfigCR.DeepCopy(), nil)
				c.crController.EXPECT().Update(gomock.Any()).Return(readConfigCR.DeepCopy(), nil)
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
			setupControllers: func(c controllers) {
				c.crCache.EXPECT().Get(gomock.Any()).Return(readConfigCR.DeepCopy(), nil)
				c.crController.EXPECT().Update(gomock.Any()).Return(nil, fmt.Errorf("error"))
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
			setupControllers: func(c controllers) {
				c.crCache.EXPECT().Get(gomock.Any()).Return(nil, nil)
				c.crController.EXPECT().Create(gomock.Any()).Return(nil, fmt.Errorf("error"))
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
			setupControllers: func(c controllers) {
				c.crCache.EXPECT().Get(gomock.Any()).Return(nil, nil)
				c.crController.EXPECT().Create(gomock.Any()).Return(readPodCR.DeepCopy(), nil)
			},
			globalRole: defaultGR.DeepCopy(),
			wantError:  false,
			condition: reducedCondition{
				reason: ClusterRoleExists,
				status: metav1.ConditionTrue,
			},
			annotation: getCRName(&defaultGR),
		},
		{
			name: "missing grOwnerLabel in clusterRole triggers update",
			setupControllers: func(c controllers) {
				c.crCache.EXPECT().Get(gomock.Any()).Return(missingLabelCR.DeepCopy(), nil)
				c.crController.EXPECT().Update(gomock.Any()).Return(readPodCR.DeepCopy(), nil)
			},
			globalRole: defaultGR.DeepCopy(),
			wantError:  false,
			condition: reducedCondition{
				reason: ClusterRoleExists,
				status: metav1.ConditionTrue,
			},
		},
	}

	ctrl := gomock.NewController(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			controllers := controllers{
				crController: fake.NewMockNonNamespacedControllerInterface[*rbacv1.ClusterRole, *rbacv1.ClusterRoleList](ctrl),
				crCache:      fake.NewMockNonNamespacedCacheInterface[*rbacv1.ClusterRole](ctrl),
			}
			if test.setupControllers != nil {
				test.setupControllers(controllers)
			}

			grLifecycle := globalRoleLifecycle{
				crClient: controllers.crController,
				crLister: controllers.crCache,
			}
			err := grLifecycle.reconcileGlobalRole(test.globalRole)

			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
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
	errRoleNotFound := apierrors.NewNotFound(schema.GroupResource{
		Group:    "rbac.authorization.k8s.io",
		Resource: "Role",
	}, "")
	errRoleAlreadyExists := apierrors.NewAlreadyExists(schema.GroupResource{
		Group:    "rbac.authorization.k8s.io",
		Resource: "Role",
	}, "")
	role1 := rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "namespacedRulesGR-namespace1",
			Namespace: "namespace1",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: namespacedRulesGR.APIVersion,
					Kind:       namespacedRulesGR.Kind,
					Name:       namespacedRulesGR.Name,
					UID:        namespacedRulesGR.UID,
				},
			},
			Labels: map[string]string{grOwnerLabel: "namespacedRulesGR"},
		},
		Rules: []rbacv1.PolicyRule{
			readPodPolicyRule,
			readConfigPolicyRule,
		},
	}
	role2 := rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "namespacedRulesGR-namespace2",
			Namespace: "namespace2",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: namespacedRulesGR.APIVersion,
					Kind:       namespacedRulesGR.Kind,
					Name:       namespacedRulesGR.Name,
					UID:        namespacedRulesGR.UID,
				},
			},
			Labels: map[string]string{grOwnerLabel: "namespacedRulesGR"},
		},
		Rules: []rbacv1.PolicyRule{
			adminPodPolicyRule,
			readPodPolicyRule,
		},
	}

	type controllers struct {
		nsCache     *fake.MockNonNamespacedCacheInterface[*corev1.Namespace]
		rCache      *fake.MockCacheInterface[*rbacv1.Role]
		rController *fake.MockControllerInterface[*rbacv1.Role, *rbacv1.RoleList]
	}

	tests := []struct {
		name             string
		setupControllers func(controllers)
		globalRole       *v3.GlobalRole
		wantError        bool
		conditions       []reducedCondition
	}{
		{
			name: "getting namespace fails",
			setupControllers: func(c controllers) {
				c.rCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, nil)
				c.nsCache.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, fmt.Errorf("error"))
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
			setupControllers: func(c controllers) {
				c.rCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, nil)

				nsNotFound := apierrors.NewNotFound(schema.GroupResource{
					Group:    rbacv1.GroupKind,
					Resource: activeNamespace.Name,
				}, "")

				c.nsCache.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nsNotFound)
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
			setupControllers: func(c controllers) {
				c.rCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, nil)
				c.nsCache.EXPECT().Get(gomock.Any()).AnyTimes().Return(nil, nil)
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
			setupControllers: func(c controllers) {
				c.rCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, nil)
				c.nsCache.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)
				c.rCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error")).Times(2)
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
			setupControllers: func(c controllers) {
				c.rCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, nil)
				c.nsCache.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)
				c.rCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, errRoleNotFound).Times(2)
				c.rController.EXPECT().Create(gomock.Any()).Return(nil, fmt.Errorf("error")).Times(2)
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
			setupControllers: func(c controllers) {
				c.rCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, nil)
				c.nsCache.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)
				c.rCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, errRoleNotFound).Times(4)
				c.rController.EXPECT().Create(gomock.Any()).Return(nil, errRoleAlreadyExists).Times(2)
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
			// It's possible that a user can create the role in the middle of the reconcile
			// In that case, the first attempt to get the role fails. Then the reconcile function attempts to
			// create the role and finds that it already exists. It gets the new role and checks that it is valid
			name: "role gets created mid reconcile",
			setupControllers: func(c controllers) {
				c.rCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, nil)
				c.nsCache.EXPECT().Get("namespace1").Return(activeNamespace, nil)
				c.nsCache.EXPECT().Get("namespace2").Return(activeNamespace, nil)
				gomock.InOrder(
					c.rCache.EXPECT().Get("namespace1", gomock.Any()).Return(nil, errRoleNotFound),
					c.rCache.EXPECT().Get("namespace1", gomock.Any()).Return(role1.DeepCopy(), nil),
				)
				c.rCache.EXPECT().Get("namespace2", gomock.Any()).Return(&rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "namespacedRulesGR-namespace2",
						Namespace: "namespace2",
						Labels:    map[string]string{grOwnerLabel: "badGR"},
					},
					Rules: []rbacv1.PolicyRule{
						adminPodPolicyRule,
						readPodPolicyRule,
					},
				}, nil)
				c.rController.EXPECT().Create(role1.DeepCopy()).Return(nil, errRoleAlreadyExists)
				c.rController.EXPECT().Update(gomock.Any()).Return(nil, nil)
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
			setupControllers: func(c controllers) {
				c.rCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, nil)
				c.nsCache.EXPECT().Get(gomock.Any()).AnyTimes().Return(activeNamespace, nil)
				c.rCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, errRoleNotFound).Times(2)
				c.rController.EXPECT().Create(role1.DeepCopy()).Return(role1.DeepCopy(), nil)
				c.rController.EXPECT().Create(role2.DeepCopy()).Return(role2.DeepCopy(), nil)
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
			setupControllers: func(c controllers) {
				c.rCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, nil)
				c.nsCache.EXPECT().Get(gomock.Any()).Return(terminatingNamespace, nil).Times(2)
				c.rCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, errRoleNotFound).Times(2)
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
			setupControllers: func(c controllers) {
				c.rCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, nil)
				c.nsCache.EXPECT().Get("namespace1").Return(activeNamespace, nil)
				c.nsCache.EXPECT().Get("namespace2").Return(activeNamespace, fmt.Errorf("error"))
				c.rCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, errRoleNotFound)
				c.rController.EXPECT().Create(role1.DeepCopy()).Return(role1.DeepCopy(), nil)
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
			setupControllers: func(c controllers) {
				c.rCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, nil)
				c.nsCache.EXPECT().Get("namespace1").Return(activeNamespace, nil)
				c.nsCache.EXPECT().Get("namespace2").Return(activeNamespace, nil)
				c.rCache.EXPECT().Get("namespace1", "namespacedRulesGR-namespace1").DoAndReturn(func(_ string, _ string) (*rbacv1.Role, error) {
					role := role1.DeepCopy()
					role.Labels = map[string]string{grOwnerLabel: "badGR"}
					role.Rules = append(role.Rules, adminPodPolicyRule)
					return role, nil
				})
				c.rController.EXPECT().Update(role1.DeepCopy()).Return(nil, nil)
				c.rCache.EXPECT().Get("namespace2", "namespacedRulesGR-namespace2").Return(nil, errRoleNotFound)
				c.rController.EXPECT().Create(role2.DeepCopy()).Return(role2.DeepCopy(), nil)
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
			name: "update an existing role no changes",
			setupControllers: func(c controllers) {
				c.rCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, nil)
				c.nsCache.EXPECT().Get("namespace1").Return(activeNamespace, nil)
				c.nsCache.EXPECT().Get("namespace2").Return(activeNamespace, nil)
				c.rCache.EXPECT().Get("namespace1", "namespacedRulesGR-namespace1").Return(role1.DeepCopy(), nil)
				c.rCache.EXPECT().Get("namespace2", "namespacedRulesGR-namespace2").Return(role2.DeepCopy(), nil)
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
			setupControllers: func(c controllers) {
				c.rCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, nil)
				c.nsCache.EXPECT().Get("namespace1").Return(activeNamespace, nil)
				c.nsCache.EXPECT().Get("namespace2").Return(activeNamespace, nil)
				c.rCache.EXPECT().Get("namespace1", "namespacedRulesGR-namespace1").DoAndReturn(func(_ string, _ string) (*rbacv1.Role, error) {
					role := role1.DeepCopy()
					role.Labels = map[string]string{grOwnerLabel: "badGR"}
					role.Rules = append(role.Rules, adminPodPolicyRule)
					return role, nil
				})
				c.rController.EXPECT().Update(role1.DeepCopy()).Return(nil, fmt.Errorf("error"))
				c.rCache.EXPECT().Get("namespace2", "namespacedRulesGR-namespace2").Return(role2.DeepCopy(), nil)
			},
			globalRole: namespacedRulesGR.DeepCopy(),
			wantError:  true,
			conditions: []reducedCondition{
				{
					reason: FailedToUpdateRole,
					status: metav1.ConditionFalse,
				},
				{
					reason: NamespacedRuleRoleExists,
					status: metav1.ConditionTrue,
				},
			},
		},
		{
			name: "listing existing roles fails",
			setupControllers: func(c controllers) {
				c.rCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error"))
				c.nsCache.EXPECT().Get("namespace1").Return(activeNamespace, nil)
				c.nsCache.EXPECT().Get("namespace2").Return(activeNamespace, nil)
				c.rCache.EXPECT().Get("namespace1", "namespacedRulesGR-namespace1").Return(role1.DeepCopy(), nil)
				c.rCache.EXPECT().Get("namespace2", "namespacedRulesGR-namespace2").Return(role2.DeepCopy(), nil)
			},
			globalRole: namespacedRulesGR.DeepCopy(),
			wantError:  true,
		},
	}
	ctrl := gomock.NewController(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			controllers := controllers{
				nsCache:     fake.NewMockNonNamespacedCacheInterface[*corev1.Namespace](ctrl),
				rCache:      fake.NewMockCacheInterface[*rbacv1.Role](ctrl),
				rController: fake.NewMockControllerInterface[*rbacv1.Role, *rbacv1.RoleList](ctrl),
			}

			if test.setupControllers != nil {
				test.setupControllers(controllers)
			}

			grLifecycle := globalRoleLifecycle{
				rClient: controllers.rController,
				rLister: controllers.rCache,
				nsCache: controllers.nsCache,
			}

			err := grLifecycle.reconcileNamespacedRoles(test.globalRole)

			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
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
				Status: v3.GlobalRoleStatus{
					Summary: status.SummaryCompleted,
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
				Status: v3.GlobalRoleStatus{},
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
			// ensure the lastUpdateTime is of format RFC3339
			if _, err := time.Parse(time.RFC3339, updatedGR.Status.LastUpdate); err != nil {
				t.Errorf("failed to parse lastUpdate as RFC3339: %v", err)
			}
			require.Empty(t, updatedGR.Status.Conditions)
			require.Equal(t, status.SummaryInProgress, updatedGR.Status.Summary)
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
				Status: v3.GlobalRoleStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "test1",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			summary:      status.SummaryCompleted,
			updateReturn: nil,
			wantError:    false,
		},
		{
			name: "gr with multiple met conditions is Completed",
			gr: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Generation: generation,
				},
				Status: v3.GlobalRoleStatus{
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
			summary:      status.SummaryCompleted,
			updateReturn: nil,
			wantError:    false,
		},
		{
			name: "gr with no conditions is Completed",
			gr: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Generation: generation,
				},
				Status: v3.GlobalRoleStatus{
					Conditions: []metav1.Condition{},
				},
			},
			summary:      status.SummaryCompleted,
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
			summary:      status.SummaryCompleted,
			updateReturn: nil,
			wantError:    false,
		},
		{
			name: "gr with one unmet and one met condition is Error",
			gr: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Generation: generation,
				},
				Status: v3.GlobalRoleStatus{
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
			summary:      status.SummaryError,
			updateReturn: nil,
			wantError:    false,
		},
		{
			name: "gr with multiple unmet conditions is Error",
			gr: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Generation: generation,
				},
				Status: v3.GlobalRoleStatus{
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
			summary:      status.SummaryError,
			updateReturn: nil,
			wantError:    false,
		},
		{
			name: "gr with unknown conditions is Error",
			gr: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Generation: generation,
				},
				Status: v3.GlobalRoleStatus{
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
			summary:      status.SummaryError,
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
			// ensure the lastUpdateTime follows format RFC3339
			if _, err := time.Parse(time.RFC3339, updatedGR.Status.LastUpdate); err != nil {
				t.Errorf("failed to parse lastUpdate as RFC3339: %v", err)
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
				Status: v3.GlobalRoleStatus{
					Summary: status.SummaryCompleted,
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
				Status: v3.GlobalRoleStatus{},
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
			// ensure the lastUpdateTime is of format RFC3339
			if _, err := time.Parse(time.RFC3339, updatedGR.Status.LastUpdate); err != nil {
				t.Errorf("failed to parse lastUpdate as RFC3339: %v", err)
			}
			require.Empty(t, updatedGR.Status.Conditions)
			require.Equal(t, status.SummaryTerminating, updatedGR.Status.Summary)
		})
	}
}
