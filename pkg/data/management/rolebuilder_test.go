package management

import (
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var errExpected = fmt.Errorf("expected error")

type testMocks struct {
	t            *testing.T
	grClientMock *fake.MockNonNamespacedClientInterface[*v3.GlobalRole, *v3.GlobalRoleList]
	rtClientMock *fake.MockNonNamespacedClientInterface[*v3.RoleTemplate, *v3.RoleTemplateList]
}

var (
	ruleReadPods = rbacv1.PolicyRule{
		Verbs:     []string{"GET", "WATCH"},
		APIGroups: []string{"v1"},
		Resources: []string{"pods"},
	}
	ruleReadServices = rbacv1.PolicyRule{
		Verbs:     []string{"GET", "WATCH"},
		APIGroups: []string{"v1"},
		Resources: []string{"services"},
	}
	ruleWriteNodes = rbacv1.PolicyRule{
		Verbs:     []string{"PUT", "CREATE", "UPDATE"},
		APIGroups: []string{"v1"},
		Resources: []string{"nodes"},
	}
	ruleAdmin = rbacv1.PolicyRule{
		Verbs:     []string{"*"},
		APIGroups: []string{"*"},
		Resources: []string{"*"},
	}
)

func Test_reconcileGlobalRoles(t *testing.T) {
	readGR := &v3.GlobalRole{
		ObjectMeta: v1.ObjectMeta{
			Name: "read-gr",
		},
		DisplayName: "Read GR",
		Rules:       []rbacv1.PolicyRule{ruleReadPods, ruleReadServices},
		Builtin:     true,
	}
	adminGR := &v3.GlobalRole{
		ObjectMeta: v1.ObjectMeta{
			Name: "admin-gr",
		},
		DisplayName: "Admin GR",
		Rules:       []rbacv1.PolicyRule{ruleAdmin},
		Builtin:     true,
	}
	basicGR := &v3.GlobalRole{
		ObjectMeta: v1.ObjectMeta{
			Name: "basic-gr",
		},
		DisplayName: "Basic GR",
		Rules:       []rbacv1.PolicyRule{ruleReadPods},
		Builtin:     true,
	}
	namespacedGR := &v3.GlobalRole{
		ObjectMeta: v1.ObjectMeta{
			Name: "namespaced-gr",
		},
		DisplayName: "Namespaced GR",
		NamespacedRules: map[string][]rbacv1.PolicyRule{
			"namespace1": {ruleReadPods},
		},
		Builtin: true,
	}
	tests := []struct {
		name        string
		grsToCreate []*v3.GlobalRole
		setup       func(mocks testMocks)
		wantErr     bool
	}{
		{
			name:        "Create new GR with no preexisting",
			grsToCreate: []*v3.GlobalRole{basicGR},
			setup: func(mocks testMocks) {

				// return empty list
				mocks.grClientMock.EXPECT().List(gomock.Any()).Return(&v3.GlobalRoleList{}, nil)
				mocks.grClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.grClientMock, nil)
				mocks.grClientMock.EXPECT().Create(ObjectMatcher(basicGR)).DoAndReturn(
					func(toCreate *v3.GlobalRole) (*v3.GlobalRole, error) {
						require.EqualValues(mocks.t, basicGR.Rules, toCreate.Rules, "roleBuilder did not attempt to create the correct role")
						require.EqualValues(mocks.t, basicGR.Name, toCreate.Name, "roleBuilder did not attempt to create the correct role")
						require.EqualValues(mocks.t, basicGR.DisplayName, toCreate.DisplayName, "roleBuilder did not attempt to create the correct role")
						return toCreate, nil
					})
			},
		},
		{
			name:        "Create new GR and append to existing",
			grsToCreate: []*v3.GlobalRole{basicGR, adminGR},
			setup: func(mocks testMocks) {
				curr := &v3.GlobalRoleList{Items: []v3.GlobalRole{*adminGR}}
				mocks.grClientMock.EXPECT().List(gomock.Any()).Return(curr, nil)
				mocks.grClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.grClientMock, nil)
				mocks.grClientMock.EXPECT().Create(ObjectMatcher(basicGR)).DoAndReturn(
					func(toCreate *v3.GlobalRole) (*v3.GlobalRole, error) {
						require.EqualValues(mocks.t, basicGR.Rules, toCreate.Rules, "roleBuilder did not attempt to create the correct role")
						require.EqualValues(mocks.t, basicGR.Name, toCreate.Name, "roleBuilder did not attempt to create the correct role")
						require.EqualValues(mocks.t, basicGR.DisplayName, toCreate.DisplayName, "roleBuilder did not attempt to create the correct role")
						return toCreate, nil
					})

			},
		},
		{
			name:        "Create multiple new GRs",
			grsToCreate: []*v3.GlobalRole{basicGR, readGR, adminGR},
			setup: func(mocks testMocks) {
				curr := &v3.GlobalRoleList{Items: []v3.GlobalRole{*adminGR}}
				mocks.grClientMock.EXPECT().List(gomock.Any()).Return(curr, nil)
				mocks.grClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.grClientMock, nil)
				mocks.grClientMock.EXPECT().Create(ObjectMatcher(basicGR)).DoAndReturn(
					func(toCreate *v3.GlobalRole) (*v3.GlobalRole, error) {
						require.EqualValues(mocks.t, basicGR.Rules, toCreate.Rules, "roleBuilder did not attempt to create the correct role")
						require.EqualValues(mocks.t, basicGR.Name, toCreate.Name, "roleBuilder did not attempt to create the correct role")
						require.EqualValues(mocks.t, basicGR.DisplayName, toCreate.DisplayName, "roleBuilder did not attempt to create the correct role")
						return toCreate, nil
					},
				)
				mocks.grClientMock.EXPECT().Create(ObjectMatcher(readGR)).DoAndReturn(
					func(toCreate *v3.GlobalRole) (*v3.GlobalRole, error) {
						require.EqualValues(mocks.t, readGR.Rules, toCreate.Rules, "roleBuilder did not attempt to create the correct role")
						require.EqualValues(mocks.t, readGR.Name, toCreate.Name, "roleBuilder did not attempt to create the correct role")
						require.EqualValues(mocks.t, readGR.DisplayName, toCreate.DisplayName, "roleBuilder did not attempt to create the correct role")
						return toCreate, nil
					},
				)
			},
		},
		{
			name:        "Create GR with NamespacedRules",
			grsToCreate: []*v3.GlobalRole{namespacedGR},
			setup: func(mocks testMocks) {
				mocks.grClientMock.EXPECT().List(gomock.Any()).Return(&v3.GlobalRoleList{}, nil)
				mocks.grClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.grClientMock, nil)
				mocks.grClientMock.EXPECT().Create(ObjectMatcher(namespacedGR)).DoAndReturn(
					func(toCreate *v3.GlobalRole) (*v3.GlobalRole, error) {
						require.EqualValues(mocks.t, namespacedGR.Rules, toCreate.Rules, "roleBuilder did not attempt to create the correct role")
						require.EqualValues(mocks.t, namespacedGR.Name, toCreate.Name, "roleBuilder did not attempt to create the correct role")
						require.EqualValues(mocks.t, namespacedGR.DisplayName, toCreate.DisplayName, "roleBuilder did not attempt to create the correct role")
						require.EqualValues(mocks.t, namespacedGR.NamespacedRules, toCreate.NamespacedRules, "roleBuilder did not attempt to create the correct role")
						return toCreate, nil
					})
			},
		},
		{
			name:        "Update existing GR DisplayName",
			grsToCreate: []*v3.GlobalRole{adminGR},
			setup: func(mocks testMocks) {
				oldAdmin := adminGR.DeepCopy()
				oldAdmin.DisplayName = "Old Display Name"
				curr := &v3.GlobalRoleList{Items: []v3.GlobalRole{*oldAdmin}}
				mocks.grClientMock.EXPECT().List(gomock.Any()).Return(curr, nil)
				mocks.grClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.grClientMock, nil)
				mocks.grClientMock.EXPECT().Update(ObjectMatcher(adminGR)).DoAndReturn(
					func(toUpdate *v3.GlobalRole) (*v3.GlobalRole, error) {
						require.EqualValues(mocks.t, adminGR.Rules, toUpdate.Rules, "roleBuilder attempted to update rules that were not changed")
						require.EqualValues(mocks.t, adminGR.DisplayName, toUpdate.DisplayName, "roleBuilder did not attempt to update the correct DisplayName")
						return toUpdate, nil
					})
			},
		},
		{
			name:        "Update existing GR Rules",
			grsToCreate: []*v3.GlobalRole{adminGR},
			setup: func(mocks testMocks) {
				oldAdmin := adminGR.DeepCopy()
				oldAdmin.Rules = append(oldAdmin.Rules, ruleWriteNodes)
				curr := &v3.GlobalRoleList{Items: []v3.GlobalRole{*oldAdmin}}
				mocks.grClientMock.EXPECT().List(gomock.Any()).Return(curr, nil)
				mocks.grClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.grClientMock, nil)
				mocks.grClientMock.EXPECT().Update(ObjectMatcher(adminGR)).DoAndReturn(
					func(toUpdate *v3.GlobalRole) (*v3.GlobalRole, error) {
						require.EqualValues(mocks.t, adminGR.Rules, toUpdate.Rules, "roleBuilder did not attempt to update the correct rules")
						require.EqualValues(mocks.t, adminGR.DisplayName, toUpdate.DisplayName, "roleBuilder attempted to update the unchanged display name")
						return toUpdate, nil
					})
			},
		},
		{
			name:        "Update existing GR Namespaced Rules",
			grsToCreate: []*v3.GlobalRole{namespacedGR},
			setup: func(mocks testMocks) {
				oldGR := namespacedGR.DeepCopy()
				oldGR.NamespacedRules["namespace2"] = []rbacv1.PolicyRule{ruleReadServices}
				curr := &v3.GlobalRoleList{Items: []v3.GlobalRole{*oldGR}}
				mocks.grClientMock.EXPECT().List(gomock.Any()).Return(curr, nil)
				mocks.grClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.grClientMock, nil)
				mocks.grClientMock.EXPECT().Update(ObjectMatcher(namespacedGR)).DoAndReturn(
					func(toUpdate *v3.GlobalRole) (*v3.GlobalRole, error) {
						require.EqualValues(mocks.t, namespacedGR.Rules, toUpdate.Rules, "roleBuilder did not attempt to update the correct rules")
						require.EqualValues(mocks.t, namespacedGR.NamespacedRules, toUpdate.NamespacedRules, "roleBuilder did not attempt to update the correct rules")
						require.EqualValues(mocks.t, namespacedGR.DisplayName, toUpdate.DisplayName, "roleBuilder attempted to update the unchanged display name")
						return toUpdate, nil
					})
			},
		},
		{
			name:        "Delete existing GR Rules",
			grsToCreate: nil,
			setup: func(mocks testMocks) {
				curr := &v3.GlobalRoleList{Items: []v3.GlobalRole{*adminGR}}
				mocks.grClientMock.EXPECT().List(gomock.Any()).Return(curr, nil)
				mocks.grClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.grClientMock, nil)
				mocks.grClientMock.EXPECT().Delete(adminGR.Name, nil).Return(nil)
			},
		},
		{
			name:    "Fail to delete existing GR Rules",
			wantErr: true,
			setup: func(mocks testMocks) {
				curr := &v3.GlobalRoleList{Items: []v3.GlobalRole{*adminGR}}
				mocks.grClientMock.EXPECT().List(gomock.Any()).Return(curr, nil)
				mocks.grClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.grClientMock, nil)
				mocks.grClientMock.EXPECT().Delete(adminGR.Name, nil).Return(errExpected)
			},
		},
		{
			name:    "Fail to list existing GR Rules",
			wantErr: true,
			setup: func(mocks testMocks) {
				mocks.grClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.grClientMock, nil)
				mocks.grClientMock.EXPECT().List(gomock.Any()).Return(nil, errExpected)
			},
		},
		{
			name:        "Fail to create new GR",
			grsToCreate: []*v3.GlobalRole{basicGR},
			wantErr:     true,
			setup: func(mocks testMocks) {
				mocks.grClientMock.EXPECT().List(gomock.Any()).Return(&v3.GlobalRoleList{}, nil)
				mocks.grClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.grClientMock, nil)
				mocks.grClientMock.EXPECT().Create(ObjectMatcher(basicGR)).Return(nil, errExpected)
			},
		},
		{
			name:        "Fail to update existing GR",
			grsToCreate: []*v3.GlobalRole{adminGR},
			wantErr:     true,
			setup: func(mocks testMocks) {
				oldAdmin := adminGR.DeepCopy()
				oldAdmin.Rules = append(oldAdmin.Rules, ruleWriteNodes)
				curr := &v3.GlobalRoleList{Items: []v3.GlobalRole{*oldAdmin}}
				mocks.grClientMock.EXPECT().List(gomock.Any()).Return(curr, nil)
				mocks.grClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.grClientMock, nil)
				mocks.grClientMock.EXPECT().Update(ObjectMatcher(adminGR)).Return(nil, errExpected)
			},
		},
	}
	for _, tt := range tests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			rb := newRoleBuilder()
			addGlobalRole(rb, test.grsToCreate...)
			ctrl := gomock.NewController(t)
			mocks := testMocks{
				t:            t,
				grClientMock: fake.NewMockNonNamespacedClientInterface[*v3.GlobalRole, *v3.GlobalRoleList](ctrl),
			}
			if test.setup != nil {
				test.setup(mocks)
			}
			err := rb.reconcileGlobalRoles(mocks.grClientMock)
			if test.wantErr {
				require.Error(t, err, "Expected an error while reconciling roles.")
			} else {
				require.NoError(t, err, "Unexpected error while reconciling roles.")
			}
		})
	}
}

func Test_reconcileRoleTemplate(t *testing.T) {
	readRT := &v3.RoleTemplate{
		ObjectMeta: v1.ObjectMeta{
			Name: "read-rt",
		},
		DisplayName: "Read RT",
		Rules:       []rbacv1.PolicyRule{ruleReadPods, ruleReadServices},
		Context:     "cluster",
		Builtin:     true,
	}
	adminRT := &v3.RoleTemplate{
		ObjectMeta: v1.ObjectMeta{
			Name: "admin-rt",
		},
		DisplayName: "Admin RT",
		Rules:       []rbacv1.PolicyRule{ruleAdmin},
		Context:     "cluster",
		Builtin:     true,
	}
	basicRT := &v3.RoleTemplate{
		ObjectMeta: v1.ObjectMeta{
			Name: "basic-rt",
		},
		DisplayName: "Basic RT",
		Rules:       []rbacv1.PolicyRule{ruleReadPods},
		Context:     "project",
		Builtin:     true,
	}
	invalidExternalRT := &v3.RoleTemplate{
		ObjectMeta: v1.ObjectMeta{
			Name: "invalid-external-rt",
		},
		DisplayName:   "External RT",
		Rules:         []rbacv1.PolicyRule{ruleAdmin},
		Context:       "cluster",
		Builtin:       true,
		ExternalRules: []rbacv1.PolicyRule{ruleReadPods},
	}
	tests := []struct {
		name        string
		rtsToCreate []*v3.RoleTemplate
		setup       func(mocks testMocks)
		wantErr     bool
	}{
		{
			name:        "Create new RT with no preexisting roles",
			rtsToCreate: []*v3.RoleTemplate{basicRT},
			setup: func(mocks testMocks) {
				// return empty list
				mocks.rtClientMock.EXPECT().List(gomock.Any()).Return(&v3.RoleTemplateList{}, nil)
				mocks.rtClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.rtClientMock, nil)
				mocks.rtClientMock.EXPECT().Create(ObjectMatcher(basicRT)).DoAndReturn(
					func(toCreate *v3.RoleTemplate) (*v3.RoleTemplate, error) {
						require.EqualValues(mocks.t, basicRT.Rules, toCreate.Rules, "roleBuilder did not attempt to create the correct role")
						require.EqualValues(mocks.t, basicRT.Name, toCreate.Name, "roleBuilder did not attempt to create the correct role")
						require.EqualValues(mocks.t, basicRT.DisplayName, toCreate.DisplayName, "roleBuilder did not attempt to create the correct role")
						return toCreate, nil
					})
			},
		},
		{
			name:        "Create new RT and append to existing",
			rtsToCreate: []*v3.RoleTemplate{basicRT, adminRT},
			setup: func(mocks testMocks) {
				curr := &v3.RoleTemplateList{Items: []v3.RoleTemplate{*adminRT}}
				mocks.rtClientMock.EXPECT().List(gomock.Any()).Return(curr, nil)
				mocks.rtClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.rtClientMock, nil)
				mocks.rtClientMock.EXPECT().Create(ObjectMatcher(basicRT)).DoAndReturn(
					func(toCreate *v3.RoleTemplate) (*v3.RoleTemplate, error) {
						require.EqualValues(mocks.t, basicRT.Rules, toCreate.Rules, "roleBuilder did not attempt to create the correct role")
						require.EqualValues(mocks.t, basicRT.Name, toCreate.Name, "roleBuilder did not attempt to create the correct role")
						require.EqualValues(mocks.t, basicRT.DisplayName, toCreate.DisplayName, "roleBuilder did not attempt to create the correct role")
						return toCreate, nil
					})
			},
		},
		{
			name:        "Create multiple new RTs",
			rtsToCreate: []*v3.RoleTemplate{basicRT, adminRT, readRT},
			setup: func(mocks testMocks) {
				curr := &v3.RoleTemplateList{Items: []v3.RoleTemplate{*adminRT}}
				mocks.rtClientMock.EXPECT().List(gomock.Any()).Return(curr, nil)
				mocks.rtClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.rtClientMock, nil)
				mocks.rtClientMock.EXPECT().Create(ObjectMatcher(basicRT)).DoAndReturn(
					func(toCreate *v3.RoleTemplate) (*v3.RoleTemplate, error) {
						require.EqualValues(mocks.t, basicRT.Rules, toCreate.Rules, "roleBuilder did not attempt to create the correct role")
						require.EqualValues(mocks.t, basicRT.Name, toCreate.Name, "roleBuilder did not attempt to create the correct role")
						require.EqualValues(mocks.t, basicRT.DisplayName, toCreate.DisplayName, "roleBuilder did not attempt to create the correct role")
						return toCreate, nil
					},
				)
				mocks.rtClientMock.EXPECT().Create(ObjectMatcher(readRT)).DoAndReturn(
					func(toCreate *v3.RoleTemplate) (*v3.RoleTemplate, error) {
						require.EqualValues(mocks.t, readRT.Rules, toCreate.Rules, "roleBuilder did not attempt to create the correct role")
						require.EqualValues(mocks.t, readRT.Name, toCreate.Name, "roleBuilder did not attempt to create the correct role")
						require.EqualValues(mocks.t, readRT.DisplayName, toCreate.DisplayName, "roleBuilder did not attempt to create the correct role")
						return toCreate, nil
					},
				)
			},
		},

		{
			name:        "Update existing RT DisplayName",
			rtsToCreate: []*v3.RoleTemplate{adminRT},
			setup: func(mocks testMocks) {
				oldAdmin := adminRT.DeepCopy()
				oldAdmin.DisplayName = "Update Display Name"
				curr := &v3.RoleTemplateList{Items: []v3.RoleTemplate{*oldAdmin}}
				mocks.rtClientMock.EXPECT().List(gomock.Any()).Return(curr, nil)
				mocks.rtClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.rtClientMock, nil)
				mocks.rtClientMock.EXPECT().Update(ObjectMatcher(adminRT)).DoAndReturn(
					func(toUpdate *v3.RoleTemplate) (*v3.RoleTemplate, error) {
						require.EqualValues(mocks.t, adminRT.Rules, toUpdate.Rules, "roleBuilder attempted to update rules that were not changed")
						require.EqualValues(mocks.t, adminRT.DisplayName, toUpdate.DisplayName, "roleBuilder did not attempt to update the correct DisplayName")
						return toUpdate, nil
					})
			},
		},
		{
			name:        "Update existing RT Rules",
			rtsToCreate: []*v3.RoleTemplate{adminRT},
			setup: func(mocks testMocks) {
				oldAdmin := adminRT.DeepCopy()
				oldAdmin.Rules = append(oldAdmin.Rules, ruleWriteNodes)
				curr := &v3.RoleTemplateList{Items: []v3.RoleTemplate{*oldAdmin}}
				mocks.rtClientMock.EXPECT().List(gomock.Any()).Return(curr, nil)
				mocks.rtClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.rtClientMock, nil)
				mocks.rtClientMock.EXPECT().Update(ObjectMatcher(adminRT)).DoAndReturn(
					func(toUpdate *v3.RoleTemplate) (*v3.RoleTemplate, error) {
						require.EqualValues(mocks.t, adminRT.Rules, toUpdate.Rules, "roleBuilder did not attempt to update the correct rules")
						require.EqualValues(mocks.t, adminRT.DisplayName, toUpdate.DisplayName, "roleBuilder attempted to update the unchanged display name")
						return toUpdate, nil
					})
			},
		},
		{
			name:        "Update existing RT builtin bool",
			rtsToCreate: []*v3.RoleTemplate{adminRT},
			setup: func(mocks testMocks) {
				oldAdmin := adminRT.DeepCopy()
				oldAdmin.Builtin = false
				curr := &v3.RoleTemplateList{Items: []v3.RoleTemplate{*oldAdmin}}
				mocks.rtClientMock.EXPECT().List(gomock.Any()).Return(curr, nil)
				mocks.rtClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.rtClientMock, nil)
				mocks.rtClientMock.EXPECT().Update(ObjectMatcher(adminRT)).DoAndReturn(
					func(toUpdate *v3.RoleTemplate) (*v3.RoleTemplate, error) {
						require.EqualValues(mocks.t, adminRT.Builtin, toUpdate.Builtin, "roleBuilder did not make the correct updates")
						require.EqualValues(mocks.t, adminRT.DisplayName, toUpdate.DisplayName, "roleBuilder attempted to update the unchanged display name")
						return toUpdate, nil
					})
			},
		},
		{
			name:        "Update existing RT External bool",
			rtsToCreate: []*v3.RoleTemplate{adminRT},
			setup: func(mocks testMocks) {
				oldAdmin := adminRT.DeepCopy()
				oldAdmin.External = true
				curr := &v3.RoleTemplateList{Items: []v3.RoleTemplate{*oldAdmin}}
				mocks.rtClientMock.EXPECT().List(gomock.Any()).Return(curr, nil)
				mocks.rtClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.rtClientMock, nil)
				mocks.rtClientMock.EXPECT().Update(ObjectMatcher(adminRT)).DoAndReturn(
					func(toUpdate *v3.RoleTemplate) (*v3.RoleTemplate, error) {
						require.EqualValues(mocks.t, adminRT.External, toUpdate.External, "roleBuilder did not make the correct updates")
						require.EqualValues(mocks.t, adminRT.DisplayName, toUpdate.DisplayName, "roleBuilder attempted to update the unchanged display name")
						return toUpdate, nil
					})
			},
		},
		{
			name:        "Update existing RT external rules",
			rtsToCreate: []*v3.RoleTemplate{adminRT},
			setup: func(mocks testMocks) {
				oldAdmin := adminRT.DeepCopy()
				oldAdmin.External = true
				oldAdmin.ExternalRules = []rbacv1.PolicyRule{ruleAdmin}
				curr := &v3.RoleTemplateList{Items: []v3.RoleTemplate{*oldAdmin}}
				mocks.rtClientMock.EXPECT().List(gomock.Any()).Return(curr, nil)
				mocks.rtClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.rtClientMock, nil)
				mocks.rtClientMock.EXPECT().Update(ObjectMatcher(adminRT)).DoAndReturn(
					func(toUpdate *v3.RoleTemplate) (*v3.RoleTemplate, error) {
						require.EqualValues(mocks.t, adminRT.External, toUpdate.External, "roleBuilder did not make the correct updates")
						require.EqualValues(mocks.t, adminRT.ExternalRules, toUpdate.ExternalRules, "roleBuilder did not make the correct external rules updates")
						return toUpdate, nil
					})
			},
		},
		{
			name:        "Update existing RT Hidden bool",
			rtsToCreate: []*v3.RoleTemplate{adminRT},
			setup: func(mocks testMocks) {
				oldAdmin := adminRT.DeepCopy()
				oldAdmin.Hidden = true
				curr := &v3.RoleTemplateList{Items: []v3.RoleTemplate{*oldAdmin}}
				mocks.rtClientMock.EXPECT().List(gomock.Any()).Return(curr, nil)
				mocks.rtClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.rtClientMock, nil)
				mocks.rtClientMock.EXPECT().Update(ObjectMatcher(adminRT)).DoAndReturn(
					func(toUpdate *v3.RoleTemplate) (*v3.RoleTemplate, error) {
						require.EqualValues(mocks.t, adminRT.Hidden, toUpdate.Hidden, "roleBuilder did not make the correct updates")
						require.EqualValues(mocks.t, adminRT.DisplayName, toUpdate.DisplayName, "roleBuilder attempted to update the unchanged display name")
						return toUpdate, nil
					})
			},
		},
		{
			name:        "Update existing RT Context",
			rtsToCreate: []*v3.RoleTemplate{adminRT},
			setup: func(mocks testMocks) {
				oldAdmin := adminRT.DeepCopy()
				oldAdmin.Context = "project"
				curr := &v3.RoleTemplateList{Items: []v3.RoleTemplate{*oldAdmin}}
				mocks.rtClientMock.EXPECT().List(gomock.Any()).Return(curr, nil)
				mocks.rtClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.rtClientMock, nil)
				mocks.rtClientMock.EXPECT().Update(ObjectMatcher(adminRT)).DoAndReturn(
					func(toUpdate *v3.RoleTemplate) (*v3.RoleTemplate, error) {
						require.EqualValues(mocks.t, adminRT.Context, toUpdate.Context, "roleBuilder did not make the correct updates")
						require.EqualValues(mocks.t, adminRT.DisplayName, toUpdate.DisplayName, "roleBuilder attempted to update the unchanged display name")
						return toUpdate, nil
					})
			},
		},
		{
			name:        "Update existing RT inherited RoleTemplates",
			rtsToCreate: []*v3.RoleTemplate{adminRT},
			setup: func(mocks testMocks) {
				oldAdmin := adminRT.DeepCopy()
				oldAdmin.RoleTemplateNames = []string{readRT.Name}
				curr := &v3.RoleTemplateList{Items: []v3.RoleTemplate{*oldAdmin}}
				mocks.rtClientMock.EXPECT().List(gomock.Any()).Return(curr, nil)
				mocks.rtClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.rtClientMock, nil)
				mocks.rtClientMock.EXPECT().Update(ObjectMatcher(adminRT)).DoAndReturn(
					func(toUpdate *v3.RoleTemplate) (*v3.RoleTemplate, error) {
						require.EqualValues(mocks.t, adminRT.RoleTemplateNames, toUpdate.RoleTemplateNames, "roleBuilder did not make the correct updates")
						require.EqualValues(mocks.t, adminRT.DisplayName, toUpdate.DisplayName, "roleBuilder attempted to update the unchanged display name")
						return toUpdate, nil
					})
			},
		},
		{
			name:        "Update existing RT Administrative bool",
			rtsToCreate: []*v3.RoleTemplate{adminRT},
			setup: func(mocks testMocks) {
				oldAdmin := adminRT.DeepCopy()
				oldAdmin.Administrative = true
				curr := &v3.RoleTemplateList{Items: []v3.RoleTemplate{*oldAdmin}}
				mocks.rtClientMock.EXPECT().List(gomock.Any()).Return(curr, nil)
				mocks.rtClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.rtClientMock, nil)
				mocks.rtClientMock.EXPECT().Update(ObjectMatcher(adminRT)).DoAndReturn(
					func(toUpdate *v3.RoleTemplate) (*v3.RoleTemplate, error) {
						require.EqualValues(mocks.t, adminRT.Administrative, toUpdate.Administrative, "roleBuilder did not make the correct updates")
						require.EqualValues(mocks.t, adminRT.DisplayName, toUpdate.DisplayName, "roleBuilder attempted to update the unchanged display name")
						return toUpdate, nil
					})
			},
		},
		{
			name:        "Delete existing RT Rules",
			rtsToCreate: nil,
			setup: func(mocks testMocks) {
				curr := &v3.RoleTemplateList{Items: []v3.RoleTemplate{*adminRT}}
				mocks.rtClientMock.EXPECT().List(gomock.Any()).Return(curr, nil)
				mocks.rtClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.rtClientMock, nil)
				mocks.rtClientMock.EXPECT().Delete(adminRT.Name, nil).Return(nil)
			},
		},
		{
			name:        "Fail to delete existing RT Rules",
			rtsToCreate: nil,
			setup: func(mocks testMocks) {
				curr := &v3.RoleTemplateList{Items: []v3.RoleTemplate{*adminRT}}
				mocks.rtClientMock.EXPECT().List(gomock.Any()).Return(curr, nil)
				mocks.rtClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.rtClientMock, nil)
				mocks.rtClientMock.EXPECT().Delete(adminRT.Name, nil).Return(errExpected)
			},
			wantErr: true,
		},
		{
			name:    "Fail to list existing GRB Rules",
			wantErr: true,
			setup: func(mocks testMocks) {
				mocks.rtClientMock.EXPECT().List(gomock.Any()).Return(nil, errExpected)
				mocks.rtClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.rtClientMock, nil)
			},
		},
		{
			name:        "Fail to create new GRB",
			rtsToCreate: []*v3.RoleTemplate{basicRT},
			wantErr:     true,
			setup: func(mocks testMocks) {
				// return empty list
				mocks.rtClientMock.EXPECT().List(gomock.Any()).Return(&v3.RoleTemplateList{}, nil)
				mocks.rtClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.rtClientMock, nil)
				mocks.rtClientMock.EXPECT().Create(ObjectMatcher(basicRT)).Return(nil, errExpected)
			},
		},
		{
			name:        "Fail to update existing GRB",
			rtsToCreate: []*v3.RoleTemplate{adminRT},
			wantErr:     true,
			setup: func(mocks testMocks) {
				oldAdmin := adminRT.DeepCopy()
				oldAdmin.Rules = append(oldAdmin.Rules, ruleWriteNodes)
				curr := &v3.RoleTemplateList{Items: []v3.RoleTemplate{*oldAdmin}}
				mocks.rtClientMock.EXPECT().List(gomock.Any()).Return(curr, nil)
				mocks.rtClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.rtClientMock, nil)
				mocks.rtClientMock.EXPECT().Update(ObjectMatcher(adminRT)).Return(nil, errExpected)
			},
		},
		{
			name:        "Fail to create invalid external RoleTemplate",
			rtsToCreate: []*v3.RoleTemplate{invalidExternalRT},
			wantErr:     true,
			setup: func(mocks testMocks) {
				mocks.rtClientMock.EXPECT().WithImpersonation(controllers.WebhookImpersonation()).Return(mocks.rtClientMock, nil)
			},
		},
	}
	for _, tt := range tests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			rb := newRoleBuilder()
			addRoleTemplates(rb, test.rtsToCreate...)
			ctrl := gomock.NewController(t)
			mocks := testMocks{
				t:            t,
				rtClientMock: fake.NewMockNonNamespacedClientInterface[*v3.RoleTemplate, *v3.RoleTemplateList](ctrl),
			}
			if test.setup != nil {
				test.setup(mocks)
			}
			err := rb.reconcileRoleTemplates(mocks.rtClientMock)
			if test.wantErr {
				require.Error(t, err, "Expected an error while reconciling roles.")
			} else {
				require.NoError(t, err, "Unexpected error while reconciling roles.")
			}
		})
	}
}

func addGlobalRole(builder *roleBuilder, roles ...*v3.GlobalRole) {
	for _, gr := range roles {
		r := builder.addRole(gr.DisplayName, gr.Name)
		addRules(r, gr.Rules...)
		addNamespacedRules(builder, gr.NamespacedRules)
	}
}

func addRoleTemplates(builder *roleBuilder, templates ...*v3.RoleTemplate) {
	for _, rt := range templates {
		builder = builder.addRoleTemplate(
			rt.DisplayName, rt.Name, rt.Context, rt.External, rt.Hidden, rt.Administrative,
		)
		addRules(builder, rt.Rules...)
		addExternalRules(builder, rt.ExternalRules...)
	}
}

func addRules(builder *roleBuilder, rules ...rbacv1.PolicyRule) {
	for _, rule := range rules {
		rb := builder.addRule()
		rb.verbs(rule.Verbs...)
		rb.apiGroups(rule.APIGroups...)
		rb.resources(rule.Resources...)
		rb.resourceNames(rule.ResourceNames...)
		rb.nonResourceURLs(rule.NonResourceURLs...)
	}
}
func addNamespacedRules(builder *roleBuilder, nsRules map[string][]rbacv1.PolicyRule) {
	for ns, rules := range nsRules {
		nsrb := builder.addNamespacedRule(ns)
		for _, r := range rules {
			rb := nsrb.addRule()
			rb.verbs(r.Verbs...)
			rb.apiGroups(r.APIGroups...)
			rb.resources(r.Resources...)
			rb.resourceNames(r.ResourceNames...)
			rb.nonResourceURLs(r.NonResourceURLs...)
		}
	}
}

func addExternalRules(builder *roleBuilder, externalRules ...rbacv1.PolicyRule) {
	for _, rule := range externalRules {
		rb := builder.addExternalRule()
		rb.verbs(rule.Verbs...)
		rb.apiGroups(rule.APIGroups...)
		rb.resources(rule.Resources...)
		rb.resourceNames(rule.ResourceNames...)
		rb.nonResourceURLs(rule.NonResourceURLs...)
	}
}

type objectMatcher struct {
	name      string
	namespace string
}

func (o objectMatcher) Matches(x any) bool {
	obj, ok := x.(v1.Object)
	if !ok {
		return false
	}
	return obj.GetName() == o.name && obj.GetNamespace() == o.namespace
}
func (o objectMatcher) String() string {
	return fmt.Sprintf("is equal to metav1.Object with name: %q and namespace: %q", o.name, o.namespace)
}

func ObjectMatcher(obj v1.Object) gomock.Matcher {
	return &objectMatcher{obj.GetName(), obj.GetNamespace()}
}
