package globalroles

import (
	"testing"

	wrangler "github.com/rancher/wrangler/v2/pkg/name"

	"github.com/golang/mock/gomock"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v2/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	grName  = "gr"
	grUID   = "9cf141b8-54ab-4711-8e43-eb1fc0a189a8"
	grbName = "grb"
	grbUID  = "3267582b-96eb-4752-81de-cb33e7d8f3e7"
	user    = "user"
)

var (
	grVerbs = &v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: grName,
			UID:  grUID,
		},
		InheritedFleetWorkspacePermissions: v3.FleetWorkspacePermission{
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
	grb = &v3.GlobalRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: grbName,
			UID:  grbUID,
		},
		UserName:       user,
		GlobalRoleName: grName,
	}
)

func TestReconcileFleetWorkspacePermissionsBindings(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	type testState struct {
		crbClient *fake.MockNonNamespacedControllerInterface[*rbac.ClusterRoleBinding, *rbac.ClusterRoleBindingList]
		crbCache  *fake.MockNonNamespacedCacheInterface[*rbac.ClusterRoleBinding]
		grCache   *fake.MockNonNamespacedCacheInterface[*v3.GlobalRole]
		rbClient  *fake.MockControllerInterface[*rbac.RoleBinding, *rbac.RoleBindingList]
		rbCache   *fake.MockCacheInterface[*rbac.RoleBinding]
		fwCache   *fake.MockNonNamespacedCacheInterface[*v3.FleetWorkspace]
	}

	tests := map[string]struct {
		stateSetup func(state testState)
		grb        *v3.GlobalRoleBinding
	}{
		"backing RoleBindings and ClusterRoleBindings are created for a new GlobalRoleBinding": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(grVerbs, nil)
				state.rbCache.EXPECT().Get("fleet-default", grbName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.crbClient.EXPECT().Create(backingClusterRoleBinding(grb, grVerbs, wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName)))
				state.rbClient.EXPECT().Create(backingRoleBinding(grb, grVerbs, "fleet-default"))
				state.crbCache.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
			},

			grb: grb,
		},
		"backing RoleBindings and ClusterRoleBindings are updated with new content": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(grVerbs, nil)
				rb := backingRoleBinding(grb, grVerbs, "fleet-default")
				rb.Subjects = []rbac.Subject{
					{
						Name: "modified",
					},
				}
				state.rbCache.EXPECT().Get("fleet-default", grbName).Return(backingRoleBinding(grb, grVerbs, "fleet-default"), nil)
				state.crbClient.EXPECT().Delete(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName), &metav1.DeleteOptions{})
				state.crbClient.EXPECT().Create(backingClusterRoleBinding(grb, grVerbs, wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName)))
				state.rbClient.EXPECT().Delete("fleet-default", grbName, &metav1.DeleteOptions{})
				state.rbClient.EXPECT().Create(backingRoleBinding(grb, grVerbs, "fleet-default"))
				state.crbCache.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(backingClusterRoleBinding(grb, grVerbs, wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName)), nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
			},
			grb: grb,
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			state := testState{
				crbClient: fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRoleBinding, *rbac.ClusterRoleBindingList](ctrl),
				crbCache:  fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRoleBinding](ctrl),
				grCache:   fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](ctrl),
				rbClient:  fake.NewMockControllerInterface[*rbac.RoleBinding, *rbac.RoleBindingList](ctrl),
				rbCache:   fake.NewMockCacheInterface[*rbac.RoleBinding](ctrl),
				fwCache:   fake.NewMockNonNamespacedCacheInterface[*v3.FleetWorkspace](ctrl),
			}
			if test.stateSetup != nil {
				test.stateSetup(state)
			}
			h := fleetWorkspaceBindingHandler{
				crbClient: state.crbClient,
				crbCache:  state.crbCache,
				grCache:   state.grCache,
				rbClient:  state.rbClient,
				rbCache:   state.rbCache,
				fwCache:   state.fwCache,
			}

			err := h.reconcileFleetWorkspacePermissionsBindings(test.grb)

			assert.Equal(t, err, nil)
		})
	}
}

func TestReconcileFleetWorkspacePermissionsBindings_errors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	type testState struct {
		crbClient *fake.MockNonNamespacedControllerInterface[*rbac.ClusterRoleBinding, *rbac.ClusterRoleBindingList]
		crbCache  *fake.MockNonNamespacedCacheInterface[*rbac.ClusterRoleBinding]
		grCache   *fake.MockNonNamespacedCacheInterface[*v3.GlobalRole]
		rbClient  *fake.MockControllerInterface[*rbac.RoleBinding, *rbac.RoleBindingList]
		rbCache   *fake.MockCacheInterface[*rbac.RoleBinding]
		fwCache   *fake.MockNonNamespacedCacheInterface[*v3.FleetWorkspace]
	}
	tests := map[string]struct {
		stateSetup     func(state testState)
		wantErrMessage string
	}{
		"GlobalRole not found": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(nil, errors.NewNotFound(schema.GroupResource{
					Group:    "management.cattle.io",
					Resource: "GlobalRole",
				}, grName))
			},
			wantErrMessage: "unable to get globalRole: GlobalRole.management.cattle.io \"gr\" not found",
		},
		"Error retrieving fleetworkspaces": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(grVerbs, nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(nil, errors.NewServiceUnavailable("unexpected error"))
			},
			wantErrMessage: "unable to list fleetWorkspaces when reconciling globalRoleBinding grb: unexpected error",
		},
		"Error creating backing RoleBindings for permission rules": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(grVerbs, nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
				state.rbCache.EXPECT().Get("fleet-default", grbName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.rbClient.EXPECT().Create(backingRoleBinding(grb, grVerbs, "fleet-default")).Return(nil, errors.NewServiceUnavailable("unexpected error"))
			},
			wantErrMessage: "error reconciling fleet permissions rules: 1 error occurred:\n\t* unexpected error\n\n",
		},
		"Error deleting backing RoleBindings for permission rules when it has changed": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(grVerbs, nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
				state.rbCache.EXPECT().Get("fleet-default", grbName).Return(&rbac.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "fleet-default",
						Name:      grbName,
					},
				}, nil)
				state.rbClient.EXPECT().Delete("fleet-default", grbName, &metav1.DeleteOptions{}).Return(errors.NewServiceUnavailable("unexpected error"))
				state.rbClient.EXPECT().Create(backingRoleBinding(grb, grVerbs, "fleet-default")).Return(nil, errors.NewServiceUnavailable("unexpected error"))
			},
			wantErrMessage: "error reconciling fleet permissions rules",
		},
		"Error creating backing ClusterRoleBinding for workspace verbs": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(grVerbs, nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
				state.rbCache.EXPECT().Get("fleet-default", grbName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.rbClient.EXPECT().Create(backingRoleBinding(grb, grVerbs, "fleet-default"))
				state.crbCache.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.crbClient.EXPECT().Create(backingClusterRoleBinding(grb, grVerbs, wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName))).Return(nil, errors.NewServiceUnavailable("unexpected error"))
			},
			wantErrMessage: "error reconciling fleet workspace verbs: unexpected error",
		},
		"Error deleting backing ClusterRoleBinding for workspace verbs when it has changed": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(grVerbs, nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
				state.rbCache.EXPECT().Get("fleet-default", grbName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.rbClient.EXPECT().Create(backingRoleBinding(grb, grVerbs, "fleet-default"))
				state.crbCache.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName)},
				}, nil)
				state.crbClient.EXPECT().Delete(wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName), &metav1.DeleteOptions{}).Return(errors.NewServiceUnavailable("unexpected error"))
				state.rbClient.EXPECT().Create(backingClusterRoleBinding(grb, grVerbs, wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName))).Return(nil, errors.NewServiceUnavailable("unexpected error"))
			},
			wantErrMessage: "error reconciling fleet workspace verbs",
		},
		"Error getting RoleBinding": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(grVerbs, nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
				state.rbCache.EXPECT().Get("fleet-default", grbName).Return(nil, errors.NewServiceUnavailable("unexpected error"))
			},
			wantErrMessage: "error reconciling fleet permissions rules",
		},
		"Error getting ClusterRoleBinding": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(grVerbs, nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
				state.rbCache.EXPECT().Get("fleet-default", grbName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.rbClient.EXPECT().Create(backingRoleBinding(grb, grVerbs, "fleet-default"))
				state.crbCache.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(nil, errors.NewServiceUnavailable("unexpected error"))
			},
			wantErrMessage: "error reconciling fleet workspace verbs",
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			state := testState{
				crbClient: fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRoleBinding, *rbac.ClusterRoleBindingList](ctrl),
				crbCache:  fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRoleBinding](ctrl),
				grCache:   fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRole](ctrl),
				rbClient:  fake.NewMockControllerInterface[*rbac.RoleBinding, *rbac.RoleBindingList](ctrl),
				rbCache:   fake.NewMockCacheInterface[*rbac.RoleBinding](ctrl),
				fwCache:   fake.NewMockNonNamespacedCacheInterface[*v3.FleetWorkspace](ctrl),
			}
			if test.stateSetup != nil {
				test.stateSetup(state)
			}
			h := fleetWorkspaceBindingHandler{
				crbClient: state.crbClient,
				crbCache:  state.crbCache,
				grCache:   state.grCache,
				rbClient:  state.rbClient,
				rbCache:   state.rbCache,
				fwCache:   state.fwCache,
			}

			err := h.reconcileFleetWorkspacePermissionsBindings(&v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: grbName,
					UID:  grbUID,
				},
				UserName:       user,
				GlobalRoleName: grName,
			})

			assert.ErrorContains(t, err, test.wantErrMessage)
		})
	}
}
