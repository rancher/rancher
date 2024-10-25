package globalroles

import (
	"testing"

	wrangler "github.com/rancher/wrangler/v3/pkg/name"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers"
	genv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	rancherbac "github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
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
	grVerbsNoFleetPermissions = &v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: grName,
			UID:  grUID,
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
		crCache   *fake.MockNonNamespacedCacheInterface[*rbac.ClusterRole]
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
				state.crbClient.EXPECT().Create(mockClusterRoleBinding(grb, grVerbs, wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName)))
				state.rbClient.EXPECT().Create(mockRoleBinding(grb, grVerbs, "fleet-default"))
				state.crbCache.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRole{}, nil)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceClusterRulesName)).Return(&rbac.ClusterRole{}, nil)
			},

			grb: grb,
		},
		"backing RoleBindings and ClusterRoleBindings are updated with new content": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(grVerbs, nil)
				rb := mockRoleBinding(grb, grVerbs, "fleet-default")
				rb.Subjects = []rbac.Subject{
					{
						Name: "modified",
					},
				}
				state.rbCache.EXPECT().Get("fleet-default", grbName).Return(mockRoleBinding(grb, grVerbs, "fleet-default"), nil)
				state.crbClient.EXPECT().Delete(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName), &metav1.DeleteOptions{})
				state.crbClient.EXPECT().Create(mockClusterRoleBinding(grb, grVerbs, wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName)))
				state.rbClient.EXPECT().Delete("fleet-default", grbName, &metav1.DeleteOptions{})
				state.rbClient.EXPECT().Create(mockRoleBinding(grb, grVerbs, "fleet-default"))
				state.crbCache.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(mockClusterRoleBinding(grb, grVerbs, wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName)), nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRole{}, nil)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceClusterRulesName)).Return(&rbac.ClusterRole{}, nil)
			},
			grb: grb,
		},
		"no RoleBindings and ClusterRoleBindings are created or updated if inheritedFleetWorkspaceRoles not provided": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(grVerbsNoFleetPermissions, nil)
				state.rbCache.EXPECT().Get("fleet-default", grbName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.crbCache.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
			},

			grb: grb,
		},
		"RoleBindings and ClusterRoleBinding are deleted if inheritedFleetWorkspaceRoles is set to nil": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(grVerbsNoFleetPermissions, nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
				state.rbCache.EXPECT().Get("fleet-default", grbName).Return(&rbac.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      grbName,
						Namespace: "fleet-default",
					},
				}, nil)
				state.crbCache.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName),
					},
				}, nil)
				state.rbClient.EXPECT().Delete("fleet-default", grbName, &metav1.DeleteOptions{})
				state.crbClient.EXPECT().Delete(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName), &metav1.DeleteOptions{})
			},

			grb: grb,
		},
		"RoleBindings and ClusterRoleBinding when inheritedFleetWorkspaceRoles is set to nil, and roles were previously deleted": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(grVerbsNoFleetPermissions, nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
				state.rbCache.EXPECT().Get("fleet-default", grbName).Return(&rbac.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      grbName,
						Namespace: "fleet-default",
					},
				}, nil)
				state.crbCache.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName),
					},
				}, nil)
				state.rbClient.EXPECT().Delete("fleet-default", grbName, &metav1.DeleteOptions{}).Return(errors.NewNotFound(schema.GroupResource{}, ""))
				state.crbClient.EXPECT().Delete(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName), &metav1.DeleteOptions{}).Return(errors.NewNotFound(schema.GroupResource{}, ""))
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
				crCache:   fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRole](ctrl),
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
				crCache:   state.crCache,
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
	grNotFoundErr := errors.NewNotFound(schema.GroupResource{
		Group:    "management.cattle.io",
		Resource: "GlobalRole",
	}, grName)
	unexpectedErr := errors.NewServiceUnavailable("unexpected error")
	type testState struct {
		crbClient *fake.MockNonNamespacedControllerInterface[*rbac.ClusterRoleBinding, *rbac.ClusterRoleBindingList]
		crbCache  *fake.MockNonNamespacedCacheInterface[*rbac.ClusterRoleBinding]
		crCache   *fake.MockNonNamespacedCacheInterface[*rbac.ClusterRole]
		grCache   *fake.MockNonNamespacedCacheInterface[*v3.GlobalRole]
		rbClient  *fake.MockControllerInterface[*rbac.RoleBinding, *rbac.RoleBindingList]
		rbCache   *fake.MockCacheInterface[*rbac.RoleBinding]
		fwCache   *fake.MockNonNamespacedCacheInterface[*v3.FleetWorkspace]
	}
	tests := map[string]struct {
		stateSetup func(state testState)
		wantErrs   []error
	}{
		"GlobalRole not found": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(nil, grNotFoundErr)
			},
			wantErrs: []error{grNotFoundErr},
		},
		"Error retrieving fleetworkspaces": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(grVerbs, nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(nil, unexpectedErr)
				state.crbClient.EXPECT().Create(mockClusterRoleBinding(grb, grVerbs, wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName)))
				state.crbCache.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRole{}, nil)
			},
			wantErrs: []error{errReconcileResourceRulesBinding, unexpectedErr},
		},
		"Error creating backing RoleBindings for permission rules": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(grVerbs, nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
				state.rbCache.EXPECT().Get("fleet-default", grbName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.rbClient.EXPECT().Create(mockRoleBinding(grb, grVerbs, "fleet-default")).Return(nil, unexpectedErr)
				state.crbClient.EXPECT().Create(mockClusterRoleBinding(grb, grVerbs, wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName)))
				state.crbCache.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRole{}, nil)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceClusterRulesName)).Return(&rbac.ClusterRole{}, nil)
			},
			wantErrs: []error{errReconcileResourceRulesBinding, unexpectedErr},
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
				state.rbClient.EXPECT().Delete("fleet-default", grbName, &metav1.DeleteOptions{}).Return(unexpectedErr)
				state.crbClient.EXPECT().Create(mockClusterRoleBinding(grb, grVerbs, wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName)))
				state.crbCache.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRole{}, nil)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceClusterRulesName)).Return(&rbac.ClusterRole{}, nil)
			},
			wantErrs: []error{errReconcileResourceRulesBinding, unexpectedErr},
		},
		"Error re-creating backing RoleBindings for permission rules when it has changed": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(grVerbs, nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
				state.rbCache.EXPECT().Get("fleet-default", grbName).Return(&rbac.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "fleet-default",
						Name:      grbName,
					},
				}, nil)
				state.rbClient.EXPECT().Delete("fleet-default", grbName, &metav1.DeleteOptions{})
				state.rbClient.EXPECT().Create(mockRoleBinding(grb, grVerbs, "fleet-default")).Return(nil, unexpectedErr)
				state.crbClient.EXPECT().Create(mockClusterRoleBinding(grb, grVerbs, wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName)))
				state.crbCache.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRole{}, nil)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceClusterRulesName)).Return(&rbac.ClusterRole{}, nil)
			},
			wantErrs: []error{errReconcileResourceRulesBinding, unexpectedErr},
		},
		"Error creating backing ClusterRoleBinding for workspace verbs": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(grVerbs, nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
				state.rbCache.EXPECT().Get("fleet-default", grbName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.rbClient.EXPECT().Create(mockRoleBinding(grb, grVerbs, "fleet-default"))
				state.crbCache.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.crbClient.EXPECT().Create(mockClusterRoleBinding(grb, grVerbs, wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName))).Return(nil, unexpectedErr)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRole{}, nil)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceClusterRulesName)).Return(&rbac.ClusterRole{}, nil)
			},
			wantErrs: []error{errReconcileWorkspaceVerbsBinding, unexpectedErr},
		},
		"Error deleting backing ClusterRoleBinding for workspace verbs when it has changed": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(grVerbs, nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
				state.rbCache.EXPECT().Get("fleet-default", grbName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.rbClient.EXPECT().Create(mockRoleBinding(grb, grVerbs, "fleet-default"))
				state.crbCache.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName)},
				}, nil)
				state.crbClient.EXPECT().Delete(wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName), &metav1.DeleteOptions{}).Return(unexpectedErr)
				state.rbClient.EXPECT().Create(mockClusterRoleBinding(grb, grVerbs, wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName))).Return(nil, nil)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRole{}, nil)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceClusterRulesName)).Return(&rbac.ClusterRole{}, nil)
			},
			wantErrs: []error{errReconcileWorkspaceVerbsBinding, unexpectedErr},
		},
		"Error re-creating backing ClusterRoleBinding for workspace verbs when it has changed": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(grVerbs, nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
				state.rbCache.EXPECT().Get("fleet-default", grbName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.rbClient.EXPECT().Create(mockRoleBinding(grb, grVerbs, "fleet-default"))
				state.crbCache.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName)},
				}, nil)
				state.crbClient.EXPECT().Delete(wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName), &metav1.DeleteOptions{})
				state.crbClient.EXPECT().Create(mockClusterRoleBinding(grb, grVerbs, wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName))).Return(nil, unexpectedErr)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRole{}, nil)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceClusterRulesName)).Return(&rbac.ClusterRole{}, nil)
			},
			wantErrs: []error{errReconcileWorkspaceVerbsBinding, unexpectedErr},
		},
		"Error creating backing ClusterRoleBinding for workspace verbs when it has changed": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(grVerbs, nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
				state.rbCache.EXPECT().Get("fleet-default", grbName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.rbClient.EXPECT().Create(mockClusterRoleBinding(grb, grVerbs, wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName))).Return(nil, nil)
				state.rbClient.EXPECT().Create(mockRoleBinding(grb, grVerbs, "fleet-default"))
				state.crbCache.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName)},
				}, nil)
				state.crbClient.EXPECT().Delete(wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName), &metav1.DeleteOptions{}).Return(nil)
				state.crbClient.EXPECT().Create(mockClusterRoleBinding(grb, grVerbs, wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName))).Return(nil, unexpectedErr)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRole{}, nil)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceClusterRulesName)).Return(&rbac.ClusterRole{}, nil)
			},
			wantErrs: []error{errReconcileWorkspaceVerbsBinding, unexpectedErr},
		},
		"Error getting RoleBinding": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(grVerbs, nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
				state.rbCache.EXPECT().Get("fleet-default", grbName).Return(nil, unexpectedErr)
				state.crbCache.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName)},
				}, nil)
				state.crbClient.EXPECT().Delete(wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName), &metav1.DeleteOptions{}).Return(unexpectedErr)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRole{}, nil)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceClusterRulesName)).Return(&rbac.ClusterRole{}, nil)
			},
			wantErrs: []error{errReconcileWorkspaceVerbsBinding, unexpectedErr},
		},
		"Error getting ClusterRoleBinding": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(grVerbs, nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
				state.rbCache.EXPECT().Get("fleet-default", grbName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.rbClient.EXPECT().Create(mockRoleBinding(grb, grVerbs, "fleet-default"))
				state.crbCache.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(nil, unexpectedErr)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRole{}, nil)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceClusterRulesName)).Return(&rbac.ClusterRole{}, nil)
			},
			wantErrs: []error{errReconcileWorkspaceVerbsBinding, unexpectedErr},
		},
		"Error getting ClusterRole for resource rules": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(grVerbs, nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
				state.rbCache.EXPECT().Get("fleet-default", grbName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.crbClient.EXPECT().Create(mockClusterRoleBinding(grb, grVerbs, wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName))).Return(nil, unexpectedErr)
				state.crbCache.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceClusterRulesName)).Return(nil, unexpectedErr)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRole{}, nil)
			},
			wantErrs: []error{errReconcileWorkspaceVerbsBinding, unexpectedErr},
		},
		"Error getting ClusterRole for workspace verbs": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(grVerbs, nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
				state.rbCache.EXPECT().Get("fleet-default", grbName).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.rbClient.EXPECT().Create(mockRoleBinding(grb, grVerbs, "fleet-default"))
				state.crbCache.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceVerbsName)).Return(nil, unexpectedErr)
				state.crCache.EXPECT().Get(wrangler.SafeConcatName(gr.Name, fleetWorkspaceClusterRulesName)).Return(&rbac.ClusterRole{}, nil)
			},
			wantErrs: []error{errReconcileWorkspaceVerbsBinding, unexpectedErr},
		},
		"Error deleting RoleBindings and ClusterRoleBinding inheritedFleetWorkspaceRoles is set to nil": {
			stateSetup: func(state testState) {
				state.grCache.EXPECT().Get(grName).Return(grVerbsNoFleetPermissions, nil)
				state.fwCache.EXPECT().List(labels.Everything()).Return(fleetWorkspaces, nil)
				state.rbCache.EXPECT().Get("fleet-default", grbName).Return(&rbac.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      grbName,
						Namespace: "fleet-default",
					},
				}, nil)
				state.crbCache.EXPECT().Get(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName)).Return(&rbac.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName),
					},
				}, nil)
				state.rbClient.EXPECT().Delete("fleet-default", grbName, &metav1.DeleteOptions{}).Return(unexpectedErr)
				state.crbClient.EXPECT().Delete(wrangler.SafeConcatName(grbName, fleetWorkspaceVerbsName), &metav1.DeleteOptions{}).Return(unexpectedErr)
			},
			wantErrs: []error{errReconcileWorkspaceVerbsBinding, errReconcileResourceRulesBinding, unexpectedErr},
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			state := testState{
				crbClient: fake.NewMockNonNamespacedControllerInterface[*rbac.ClusterRoleBinding, *rbac.ClusterRoleBindingList](ctrl),
				crbCache:  fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRoleBinding](ctrl),
				crCache:   fake.NewMockNonNamespacedCacheInterface[*rbac.ClusterRole](ctrl),
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
				crCache:   state.crCache,
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

			for _, wantErr := range test.wantErrs {
				assert.ErrorIs(t, err, wantErr)
			}
		})
	}
}

func mockClusterRoleBinding(grb *v3.GlobalRoleBinding, gr *v3.GlobalRole, crbName string) *rbac.ClusterRoleBinding {
	return &rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: crbName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: genv3.GlobalRoleBindingGroupVersionKind.GroupVersion().String(),
					Kind:       genv3.GlobalRoleBindingGroupVersionKind.Kind,
					Name:       grb.Name,
					UID:        grb.UID,
				},
			},
			Labels: map[string]string{
				grbOwnerLabel:               wrangler.SafeConcatName(grb.Name),
				controllers.K8sManagedByKey: controllers.ManagerValue,
			},
		},
		RoleRef: rbac.RoleRef{
			APIGroup: rbac.GroupName,
			Kind:     "ClusterRole",
			Name:     wrangler.SafeConcatName(gr.Name, fleetWorkspaceVerbsName),
		},
		Subjects: []rbac.Subject{rancherbac.GetGRBSubject(grb)},
	}
}

func mockRoleBinding(grb *v3.GlobalRoleBinding, gb *v3.GlobalRole, fwName string) *rbac.RoleBinding {
	return &rbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      wrangler.SafeConcatName(grb.Name),
			Namespace: fwName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: genv3.GlobalRoleBindingGroupVersionKind.GroupVersion().String(),
					Kind:       genv3.GlobalRoleBindingGroupVersionKind.Kind,
					Name:       grb.Name,
					UID:        grb.UID,
				},
			},
			Labels: map[string]string{
				grbOwnerLabel:                 wrangler.SafeConcatName(grb.Name),
				fleetWorkspacePermissionLabel: "true",
				controllers.K8sManagedByKey:   controllers.ManagerValue,
			},
		},
		RoleRef: rbac.RoleRef{
			APIGroup: rbac.GroupName,
			Kind:     "ClusterRole",
			Name:     wrangler.SafeConcatName(gb.Name, fleetWorkspaceClusterRulesName),
		},
		Subjects: []rbac.Subject{
			rancherbac.GetGRBSubject(grb),
		},
	}
}
