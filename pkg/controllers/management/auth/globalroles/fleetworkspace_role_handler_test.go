package globalroles

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers"
	genv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	wrangler "github.com/rancher/wrangler/v3/pkg/name"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	gr = &v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: grName,
			UID:  grUID,
		},
		InheritedFleetWorkspacePermissions: &v3.FleetWorkspacePermission{
			ResourceRules: []rbac.PolicyRule{
				{
					Verbs:     []string{"get", "list"},
					APIGroups: []string{"fleet.cattle.io"},
					Resources: []string{"gitrepos", "bundles"},
				},
			},
			WorkspaceVerbs: []string{"get", "list"},
		},
	}

	crResourceRulesMeta = metav1.ObjectMeta{
		Name: wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName),
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion: "management.cattle.io/v3",
				Kind:       "GlobalRole",
				Name:       grName,
				UID:        grUID,
			},
		},
		Labels: map[string]string{
			grOwnerLabel:                wrangler.SafeConcatName(grName),
			controllers.K8sManagedByKey: controllers.ManagerValue,
		},
	}

	crWorkspaceVerbsMeta = metav1.ObjectMeta{
		Name: wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName),
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion: "management.cattle.io/v3",
				Kind:       "GlobalRole",
				Name:       grName,
				UID:        grUID,
			},
		},
		Labels: map[string]string{
			grOwnerLabel:                wrangler.SafeConcatName(grName),
			controllers.K8sManagedByKey: controllers.ManagerValue,
		},
	}

	fleetWorkspaces = []*v3.FleetWorkspace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "fleet-local",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "fleet-default",
			},
		},
	}

	fleetWorkspaceNames = []string{"fleet-default"}
)

func TestReconcileFleetPermissions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	type testState struct {
		crClient *fake.MockNonNamespacedControllerInterface[*rbac.ClusterRole, *rbac.ClusterRoleList]
		crCache  *fake.MockNonNamespacedCacheInterface[*rbac.ClusterRole]
		fwCache  *fake.MockNonNamespacedCacheInterface[*v3.FleetWorkspace]
	}

	tests := map[string]struct {
		stateSetup func(state testState)
		gr         *v3.GlobalRole
	}{
		"backing ClusterRoles are created for a new GlobalRole": {
			stateSetup: func(state testState) {
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.crClient.EXPECT().Create(mockResourceRulesClusterRole(gr, wrangler.SafeConcatName(gr.Name, fleetWorkspaceClusterRulesName)))
				state.crClient.EXPECT().Create(mockWorkspaceVerbsClusterRole(gr, wrangler.SafeConcatName(gr.Name, fleetWorkspaceVerbsName), fleetWorkspaceNames))
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
			},
			gr: gr,
		},
		"no update if ClusterRoles are present, and haven't changed": {
			stateSetup: func(state testState) {
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName)).Return(mockWorkspaceVerbsClusterRole(gr, wrangler.SafeConcatName(gr.Name, fleetWorkspaceVerbsName), fleetWorkspaceNames), nil)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName)).Return(mockResourceRulesClusterRole(gr, wrangler.SafeConcatName(gr.Name, fleetWorkspaceClusterRulesName)), nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
			},
			gr: gr,
		},
		"backing Roles and ClusterRoles are updated with new content": {
			stateSetup: func(state testState) {
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName)).Return(mockWorkspaceVerbsClusterRole(gr, wrangler.SafeConcatName(gr.Name, fleetWorkspaceVerbsName), fleetWorkspaceNames), nil)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName)).Return(mockResourceRulesClusterRole(gr, wrangler.SafeConcatName(gr.Name, fleetWorkspaceClusterRulesName)), nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
				state.crClient.EXPECT().Update(&rbac.ClusterRole{
					ObjectMeta: crResourceRulesMeta,
					Rules: []rbac.PolicyRule{
						{
							Verbs:     []string{"*"},
							APIGroups: []string{"fleet.cattle.io"},
							Resources: []string{"gitrepos"},
						},
					},
				})
				state.crClient.EXPECT().Update(&rbac.ClusterRole{
					ObjectMeta: crWorkspaceVerbsMeta,
					Rules: []rbac.PolicyRule{
						{
							Verbs:         []string{"*"},
							APIGroups:     []string{"management.cattle.io"},
							Resources:     []string{"fleetworkspaces"},
							ResourceNames: []string{"fleet-default"},
						},
					},
				})
			},
			gr: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: grName,
					UID:  grUID,
				},
				InheritedFleetWorkspacePermissions: &v3.FleetWorkspacePermission{
					ResourceRules: []rbac.PolicyRule{
						{
							Verbs:     []string{"*"},
							APIGroups: []string{"fleet.cattle.io"},
							Resources: []string{"gitrepos"},
						},
					},
					WorkspaceVerbs: []string{"*"},
				},
			},
		},
		"backing ClusterRole for fleetworkspace cluster-wide resource is not created if there are no fleetworkspaces besides local": {
			stateSetup: func(state testState) {
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.crClient.EXPECT().Create(mockResourceRulesClusterRole(gr, wrangler.SafeConcatName(gr.Name, fleetWorkspaceClusterRulesName)))
				state.fwCache.EXPECT().List(labels.Everything()).Return([]*v3.FleetWorkspace{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "fleet-local",
						},
					},
				}, nil)
			},
			gr: gr,
		},
		"no backing ClusterRoles are created, updated or deleted if InheritedFleetWorkspacePermissions is not provided ": {
			stateSetup: func(state testState) {
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
			},
			gr: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: grName,
					UID:  grUID,
				},
			},
		},
		"existing backing ClusterRoles are deleted if InheritedFleetWorkspacePermissions is nil": {
			stateSetup: func(state testState) {
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName)).Return(&rbac.ClusterRole{}, nil)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRole{}, nil)
				state.crClient.EXPECT().Delete(wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName), &metav1.DeleteOptions{})
				state.crClient.EXPECT().Delete(wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName), &metav1.DeleteOptions{})
			},
			gr: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: grName,
					UID:  grUID,
				},
			},
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			state := testState{
				crClient: fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRole, *rbac.ClusterRoleList](ctrl),
				crCache:  fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRole](ctrl),
				fwCache:  fake.NewMockNonNamespacedCacheInterface[*v3.FleetWorkspace](ctrl),
			}
			if test.stateSetup != nil {
				test.stateSetup(state)
			}
			h := fleetWorkspaceRoleHandler{
				crClient: state.crClient,
				crCache:  state.crCache,
				fwCache:  state.fwCache,
			}

			err := h.reconcileFleetWorkspacePermissions(test.gr)

			assert.Equal(t, err, nil)
		})
	}
}

func TestReconcileFleetPermissions_errors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	type testState struct {
		crClient *fake.MockNonNamespacedControllerInterface[*rbac.ClusterRole, *rbac.ClusterRoleList]
		crCache  *fake.MockNonNamespacedCacheInterface[*rbac.ClusterRole]
		fwCache  *fake.MockNonNamespacedCacheInterface[*v3.FleetWorkspace]
	}
	unexpectedErr := errors.NewServiceUnavailable("unexpected error")

	tests := map[string]struct {
		stateSetup func(state testState)
		globalRole *v3.GlobalRole
		wantErrs   []error
	}{
		"Error retrieving resource rules ClusterRole": {
			stateSetup: func(state testState) {
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName)).Return(nil, unexpectedErr)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.crClient.EXPECT().Create(mockWorkspaceVerbsClusterRole(gr, wrangler.SafeConcatName(gr.Name, fleetWorkspaceVerbsName), fleetWorkspaceNames))
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
			},
			globalRole: gr,
			wantErrs:   []error{errReconcileResourceRules, unexpectedErr},
		},
		"Error creating resource rules ClusterRole": {
			stateSetup: func(state testState) {
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.crClient.EXPECT().Create(mockResourceRulesClusterRole(gr, wrangler.SafeConcatName(gr.Name, fleetWorkspaceClusterRulesName))).Return(nil, unexpectedErr)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.crClient.EXPECT().Create(mockWorkspaceVerbsClusterRole(gr, wrangler.SafeConcatName(gr.Name, fleetWorkspaceVerbsName), fleetWorkspaceNames))
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
			},
			globalRole: gr,
			wantErrs:   []error{errReconcileResourceRules, unexpectedErr},
		},
		"Error updating resource rules ClusterRole": {
			stateSetup: func(state testState) {
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName)).Return(&rbac.ClusterRole{}, nil)
				state.crClient.EXPECT().Update(&rbac.ClusterRole{
					Rules: gr.InheritedFleetWorkspacePermissions.ResourceRules,
				}).Return(nil, unexpectedErr)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.crClient.EXPECT().Create(mockWorkspaceVerbsClusterRole(gr, wrangler.SafeConcatName(gr.Name, fleetWorkspaceVerbsName), fleetWorkspaceNames))
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
			},
			globalRole: gr,
			wantErrs:   []error{errReconcileResourceRules, unexpectedErr},
		},
		"Error deleting resource rules ClusterRole": {
			stateSetup: func(state testState) {
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName)).Return(&rbac.ClusterRole{}, nil)
				state.crClient.EXPECT().Delete(wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName), &metav1.DeleteOptions{}).Return(unexpectedErr)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.crClient.EXPECT().Create(mockWorkspaceVerbsClusterRole(gr, wrangler.SafeConcatName(gr.Name, fleetWorkspaceVerbsName), fleetWorkspaceNames))
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
			},
			globalRole: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: grName,
					UID:  grUID,
				},
			},
			wantErrs: []error{errReconcileResourceRules, unexpectedErr},
		},
		"Error retrieving workspace verbs ClusterRole": {
			stateSetup: func(state testState) {
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName)).Return(nil, unexpectedErr)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.crClient.EXPECT().Create(mockResourceRulesClusterRole(gr, wrangler.SafeConcatName(gr.Name, fleetWorkspaceClusterRulesName)))
			},
			globalRole: gr,
			wantErrs:   []error{errReconcileWorkspaceVerbs, unexpectedErr},
		},
		"Error creating workspace verbs ClusterRole": {
			stateSetup: func(state testState) {
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.crClient.EXPECT().Create(mockResourceRulesClusterRole(gr, wrangler.SafeConcatName(gr.Name, fleetWorkspaceClusterRulesName)))
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
				state.crClient.EXPECT().Create(mockWorkspaceVerbsClusterRole(gr, wrangler.SafeConcatName(gr.Name, fleetWorkspaceVerbsName), []string{"fleet-default"})).Return(nil, unexpectedErr)
			},
			globalRole: gr,
			wantErrs:   []error{errReconcileWorkspaceVerbs, unexpectedErr},
		},
		"Error updating workspace verbs ClusterRole": {
			stateSetup: func(state testState) {
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.crClient.EXPECT().Create(mockResourceRulesClusterRole(gr, wrangler.SafeConcatName(gr.Name, fleetWorkspaceClusterRulesName)))
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRole{}, nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
				state.crClient.EXPECT().Update(&rbac.ClusterRole{
					Rules: mockWorkspaceVerbsClusterRole(gr, wrangler.SafeConcatName(gr.Name, fleetWorkspaceVerbsName), []string{"fleet-default"}).Rules,
				}).Return(nil, unexpectedErr)
			},
			globalRole: gr,
			wantErrs:   []error{errReconcileWorkspaceVerbs, unexpectedErr},
		},
		"Error deleting workspace verbs ClusterRole": {
			stateSetup: func(state testState) {
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.crClient.EXPECT().Create(mockResourceRulesClusterRole(gr, wrangler.SafeConcatName(gr.Name, fleetWorkspaceClusterRulesName)))
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRole{}, nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
				state.crClient.EXPECT().Delete(wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName), &metav1.DeleteOptions{}).Return(unexpectedErr)
			},
			globalRole: &v3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: grName,
					UID:  grUID,
				},
				InheritedFleetWorkspacePermissions: &v3.FleetWorkspacePermission{
					ResourceRules: gr.InheritedFleetWorkspacePermissions.ResourceRules,
				},
			},
			wantErrs: []error{errReconcileWorkspaceVerbs, unexpectedErr},
		},
		"Error getting fleet workspaces": {
			stateSetup: func(state testState) {
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceClusterRulesName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.crClient.EXPECT().Create(mockResourceRulesClusterRole(gr, wrangler.SafeConcatName(gr.Name, fleetWorkspaceClusterRulesName)))
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(grName, fleetWorkspaceVerbsName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, unexpectedErr)
			},
			globalRole: gr,
			wantErrs:   []error{errReconcileWorkspaceVerbs, unexpectedErr},
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			state := testState{
				crClient: fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRole, *rbac.ClusterRoleList](ctrl),
				crCache:  fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRole](ctrl),
				fwCache:  fake.NewMockNonNamespacedCacheInterface[*v3.FleetWorkspace](ctrl),
			}
			if test.stateSetup != nil {
				test.stateSetup(state)
			}
			h := fleetWorkspaceRoleHandler{
				crClient: state.crClient,
				crCache:  state.crCache,
				fwCache:  state.fwCache,
			}
			err := h.reconcileFleetWorkspacePermissions(test.globalRole)

			for _, wantErr := range test.wantErrs {
				assert.ErrorIs(t, err, wantErr)
			}
		})
	}
}

func mockResourceRulesClusterRole(gr *v3.GlobalRole, crName string) *rbac.ClusterRole {
	return &rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: crName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: genv3.GlobalRoleGroupVersionKind.GroupVersion().String(),
					Kind:       genv3.GlobalRoleGroupVersionKind.Kind,
					Name:       gr.Name,
					UID:        gr.UID,
				},
			},
			Labels: map[string]string{
				grOwnerLabel:                wrangler.SafeConcatName(gr.Name),
				controllers.K8sManagedByKey: controllers.ManagerValue,
			},
		},
		Rules: gr.InheritedFleetWorkspacePermissions.ResourceRules,
	}
}

func mockWorkspaceVerbsClusterRole(gr *v3.GlobalRole, crName string, workspaceNames []string) *rbac.ClusterRole {
	return &rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: crName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: genv3.GlobalRoleGroupVersionKind.GroupVersion().String(),
					Kind:       genv3.GlobalRoleGroupVersionKind.Kind,
					Name:       gr.Name,
					UID:        gr.UID,
				},
			},
			Labels: map[string]string{
				grOwnerLabel:                wrangler.SafeConcatName(gr.Name),
				controllers.K8sManagedByKey: controllers.ManagerValue,
			},
		},
		Rules: []rbac.PolicyRule{
			{
				Verbs:         gr.InheritedFleetWorkspacePermissions.WorkspaceVerbs,
				APIGroups:     []string{"management.cattle.io"},
				Resources:     []string{"fleetworkspaces"},
				ResourceNames: workspaceNames,
			},
		},
	}
}
