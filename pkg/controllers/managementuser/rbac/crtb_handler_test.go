package rbac

import (
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var (
	e           = fmt.Errorf("error")
	defaultCRTB = v3.ClusterRoleTemplateBinding{
		UserName:         "crtb-name",
		RoleTemplateName: "rt-name",
	}
	noRoleTemplateCRTB = v3.ClusterRoleTemplateBinding{
		UserName:         "crtb-name",
		RoleTemplateName: "",
	}
	noSubjectCRTB = v3.ClusterRoleTemplateBinding{
		UserName:           "",
		GroupName:          "",
		GroupPrincipalName: "",
		RoleTemplateName:   "rt-name",
	}
)

type crtbTestState struct {
	managerMock  *MockmanagerInterface
	rtListerMock *fakes.RoleTemplateListerMock
}

func TestSyncCRTB(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		stateSetup func(crtbTestState)
		crtb       *v3.ClusterRoleTemplateBinding
		wantError  bool
	}{
		{
			name: "crtb with no role template",
			crtb: noRoleTemplateCRTB.DeepCopy(),
		},
		{
			name: "crtb with no subject",
			crtb: noSubjectCRTB.DeepCopy(),
		},
		{
			name: "error getting roletemplate",
			stateSetup: func(cts crtbTestState) {
				cts.rtListerMock.GetFunc = func(namespace, name string) (*v3.RoleTemplate, error) {
					return nil, e
				}
			},
			crtb:      defaultCRTB.DeepCopy(),
			wantError: true,
		},
		{
			name: "error gathering roles",
			stateSetup: func(cts crtbTestState) {
				cts.rtListerMock.GetFunc = func(namespace, name string) (*v3.RoleTemplate, error) {
					return nil, nil
				}
				cts.managerMock.EXPECT().gatherRoles(gomock.Any(), gomock.Any(), gomock.Any()).Return(e)
			},
			crtb:      defaultCRTB.DeepCopy(),
			wantError: true,
		},
		{
			name: "error ensuring roles",
			stateSetup: func(cts crtbTestState) {
				cts.rtListerMock.GetFunc = func(namespace, name string) (*v3.RoleTemplate, error) {
					return nil, nil
				}
				cts.managerMock.EXPECT().gatherRoles(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				cts.managerMock.EXPECT().ensureRoles(gomock.Any()).Return(e)
			},
			crtb:      defaultCRTB.DeepCopy(),
			wantError: true,
		},
		{
			name: "error ensuring cluster bindings",
			stateSetup: func(cts crtbTestState) {
				cts.rtListerMock.GetFunc = func(namespace, name string) (*v3.RoleTemplate, error) {
					return nil, nil
				}
				cts.managerMock.EXPECT().gatherRoles(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				cts.managerMock.EXPECT().ensureRoles(gomock.Any()).Return(nil)
				cts.managerMock.EXPECT().ensureClusterBindings(gomock.Any(), gomock.Any()).Return(e)
			},
			crtb:      defaultCRTB.DeepCopy(),
			wantError: true,
		},
		{
			name: "error ensuring service account impersonator",
			stateSetup: func(cts crtbTestState) {
				cts.rtListerMock.GetFunc = func(namespace, name string) (*v3.RoleTemplate, error) {
					return nil, nil
				}
				cts.managerMock.EXPECT().gatherRoles(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				cts.managerMock.EXPECT().ensureRoles(gomock.Any()).Return(nil)
				cts.managerMock.EXPECT().ensureClusterBindings(gomock.Any(), gomock.Any()).Return(nil)
				cts.managerMock.EXPECT().ensureServiceAccountImpersonator(gomock.Any()).Return(e)
			},
			crtb:      defaultCRTB.DeepCopy(),
			wantError: true,
		},
		{
			name: "success",
			stateSetup: func(cts crtbTestState) {
				cts.rtListerMock.GetFunc = func(namespace, name string) (*v3.RoleTemplate, error) {
					return nil, nil
				}
				cts.managerMock.EXPECT().gatherRoles(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				cts.managerMock.EXPECT().ensureRoles(gomock.Any()).Return(nil)
				cts.managerMock.EXPECT().ensureClusterBindings(gomock.Any(), gomock.Any()).Return(nil)
				cts.managerMock.EXPECT().ensureServiceAccountImpersonator(gomock.Any()).Return(nil)
			},
			crtb: defaultCRTB.DeepCopy(),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			crtbLifecycle := crtbLifecycle{}
			state := setupCRTBTest(t)
			if test.stateSetup != nil {
				test.stateSetup(state)
			}
			crtbLifecycle.rtLister = state.rtListerMock
			crtbLifecycle.m = state.managerMock

			err := crtbLifecycle.syncCRTB(test.crtb)

			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func setupCRTBTest(t *testing.T) crtbTestState {
	ctrl := gomock.NewController(t)
	fakeManager := NewMockmanagerInterface(ctrl)
	rtListerMock := fakes.RoleTemplateListerMock{}
	state := crtbTestState{
		managerMock:  fakeManager,
		rtListerMock: &rtListerMock,
	}
	return state
}
