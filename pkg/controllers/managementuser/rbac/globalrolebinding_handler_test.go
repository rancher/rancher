package rbac

import (
	"testing"
	"time"

	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/status"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestSync(t *testing.T) {
	ctrl := gomock.NewController(t)
	err := errors.NewBadRequest("unexpected error")
	mockTime := func() time.Time {
		return time.Unix(0, 0)
	}
	grb := &v3.GlobalRoleBinding{
		GlobalRoleName: rbac.GlobalAdmin,
	}
	type grbTestStateChanges struct {
		t          *testing.T
		createdCRB *rbacv1.ClusterRoleBinding
	}
	type grbTestState struct {
		grListerMock  *fakes.GlobalRoleListerMock
		crbListerMock *fake.MockNonNamespacedCacheInterface[*rbacv1.ClusterRoleBinding]
		crbClientMock *fake.MockNonNamespacedClientInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList]
		grbListerMock *fake.MockNonNamespacedCacheInterface[*v3.GlobalRoleBinding]
		grbClientMock *fake.MockNonNamespacedControllerInterface[*v3.GlobalRoleBinding, *v3.GlobalRoleBindingList]
		stateChanges  *grbTestStateChanges
	}

	tests := map[string]struct {
		grb             *v3.GlobalRoleBinding
		stateSetup      func(*grbTestState)
		stateAssertions func(grbTestStateChanges)
		wantErr         error
	}{
		"admin role creation": {
			grb: grb,
			stateSetup: func(state *grbTestState) {
				state.crbClientMock.EXPECT().Create(gomock.Any()).DoAndReturn(func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
					state.stateChanges.createdCRB = crb
					return crb, nil
				})
				state.crbListerMock.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*rbacv1.ClusterRoleBinding, error) {
					return nil, errors.NewNotFound(schema.GroupResource{}, "")
				})
				state.grListerMock.GetFunc = func(namespace string, name string) (*v3.GlobalRole, error) {
					return &apisv3.GlobalRole{
						ObjectMeta: metav1.ObjectMeta{
							Name: rbac.GlobalAdmin,
						},
						Builtin: true,
					}, nil
				}
				state.grbListerMock.EXPECT().Get(grb.Name).Return(grb.DeepCopy(), nil)
				state.grbClientMock.EXPECT().UpdateStatus(&v3.GlobalRoleBinding{
					GlobalRoleName: rbac.GlobalAdmin,
					Status: v3.GlobalRoleBindingStatus{
						LastUpdateTime: mockTime().Format(time.RFC3339),
						SummaryRemote:  status.SummaryCompleted,
						RemoteConditions: []metav1.Condition{
							{
								Type:               clusterAdminRoleExists,
								Status:             metav1.ConditionTrue,
								LastTransitionTime: metav1.Time{Time: mockTime()},
								Reason:             clusterAdminRoleExists,
							},
						},
					},
				})
			},
			stateAssertions: func(changes grbTestStateChanges) {
				assert.Equal(changes.t, changes.createdCRB, &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: rbac.GrbCRBName(grb),
						Annotations: map[string]string{
							rbac.CrbGlobalRoleAnnotation:             grb.GlobalRoleName,
							rbac.CrbGlobalRoleBindingAnnotation:      grb.Name,
							rbac.CrbAdminGlobalRoleCheckedAnnotation: "true",
						},
					},
					Subjects: []rbacv1.Subject{rbac.GetGRBSubject(grb)},
					RoleRef: rbacv1.RoleRef{
						Name: rbac.ClusterAdminRoleName,
						Kind: "ClusterRole",
					},
				})
			},
		},
		"admin role already exists": {
			grb: grb,
			stateSetup: func(state *grbTestState) {
				state.crbListerMock.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*rbacv1.ClusterRoleBinding, error) {
					return &rbacv1.ClusterRoleBinding{}, nil
				})
				state.grListerMock.GetFunc = func(namespace string, name string) (*v3.GlobalRole, error) {
					return &apisv3.GlobalRole{
						ObjectMeta: metav1.ObjectMeta{
							Name: rbac.GlobalAdmin,
						},
						Builtin: true,
					}, nil
				}
				state.grbListerMock.EXPECT().Get(grb.Name).Return(grb.DeepCopy(), nil)
				state.grbClientMock.EXPECT().UpdateStatus(&v3.GlobalRoleBinding{
					GlobalRoleName: rbac.GlobalAdmin,
					Status: v3.GlobalRoleBindingStatus{
						LastUpdateTime: mockTime().Format(time.RFC3339),
						SummaryRemote:  status.SummaryCompleted,
						RemoteConditions: []metav1.Condition{
							{
								Type:               clusterAdminRoleExists,
								Status:             metav1.ConditionTrue,
								LastTransitionTime: metav1.Time{Time: mockTime()},
								Reason:             clusterAdminRoleExists,
							},
						},
					},
				})
			},
		},
		"no admin role": {
			grb: &v3.GlobalRoleBinding{
				GlobalRoleName: "gr",
			},
			stateSetup: func(state *grbTestState) {
				state.grListerMock.GetFunc = func(namespace string, name string) (*v3.GlobalRole, error) {
					return &apisv3.GlobalRole{
						ObjectMeta: metav1.ObjectMeta{
							Name: "gr",
						},
						Builtin: true,
					}, nil
				}
			},
		},
		"error getting GlobalRole": {
			grb: &v3.GlobalRoleBinding{
				GlobalRoleName: "gr",
			},
			stateSetup: func(state *grbTestState) {
				state.grListerMock.GetFunc = func(namespace string, name string) (*v3.GlobalRole, error) {
					return nil, err
				}
			},
			wantErr: err,
		},
		"error getting ClusterRoleBinding": {
			grb: grb,
			stateSetup: func(state *grbTestState) {
				state.crbListerMock.EXPECT().Get(gomock.Any()).Return(nil, err)
				state.grListerMock.GetFunc = func(namespace string, name string) (*v3.GlobalRole, error) {
					return &apisv3.GlobalRole{
						ObjectMeta: metav1.ObjectMeta{
							Name: rbac.GlobalAdmin,
						},
						Builtin: true,
					}, nil
				}
			},
			wantErr: err,
		},
		"error creating ClusterRole": {
			grb: grb,
			stateSetup: func(state *grbTestState) {
				state.crbClientMock.EXPECT().Create(gomock.Any()).Return(nil, err)
				state.crbListerMock.EXPECT().Get(gomock.Any()).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				state.grListerMock.GetFunc = func(namespace string, name string) (*v3.GlobalRole, error) {
					return &apisv3.GlobalRole{
						ObjectMeta: metav1.ObjectMeta{
							Name: rbac.GlobalAdmin,
						},
						Builtin: true,
					}, nil
				}
			},

			wantErr: err,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			grListerMock := &fakes.GlobalRoleListerMock{}
			crbListerMock := fake.NewMockNonNamespacedCacheInterface[*rbacv1.ClusterRoleBinding](ctrl)
			crbClientMock := fake.NewMockNonNamespacedClientInterface[*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBindingList](ctrl)
			grbListerMock := fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRoleBinding](ctrl)
			grbClientMock := fake.NewMockNonNamespacedControllerInterface[*v3.GlobalRoleBinding, *v3.GlobalRoleBindingList](ctrl)

			status := status.NewStatus()
			status.TimeNow = mockTime
			stateChanges := &grbTestStateChanges{
				t: t,
			}
			state := &grbTestState{
				grListerMock:  grListerMock,
				grbListerMock: grbListerMock,
				grbClientMock: grbClientMock,
				crbListerMock: crbListerMock,
				crbClientMock: crbClientMock,
				stateChanges:  stateChanges,
			}
			if test.stateSetup != nil {
				test.stateSetup(state)
			}
			h := grbHandler{
				crbClient: crbClientMock,
				crbLister: crbListerMock,
				grLister:  grListerMock,
				grbLister: grbListerMock,
				grbClient: grbClientMock,
				status:    status,
			}

			_, err := h.sync("", test.grb.DeepCopy())

			if test.stateAssertions != nil {
				test.stateAssertions(*state.stateChanges)
			}
			if err != nil {
				assert.ErrorContains(t, err, test.wantErr.Error())
			} else {
				assert.NoError(t, test.wantErr)
			}
		})
	}
}
