package rbac

import (
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	rbacFakes "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1/fakes"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"testing"

	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSync(t *testing.T) {
	err := errors.NewBadRequest("unexpected error")
	grb := &v3.GlobalRoleBinding{
		GlobalRoleName: rbac.GlobalAdmin,
	}
	type grbTestStateChanges struct {
		t          *testing.T
		createdCRB *rbacv1.ClusterRoleBinding
	}
	type grbTestState struct {
		grListerMock  *fakes.GlobalRoleListerMock
		crbListerMock *rbacFakes.ClusterRoleBindingListerMock
		crbClientMock *rbacFakes.ClusterRoleBindingInterfaceMock
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
				state.crbClientMock.CreateFunc = func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
					state.stateChanges.createdCRB = crb
					return crb, nil
				}
				state.crbListerMock.GetFunc = func(namespace, name string) (*rbacv1.ClusterRoleBinding, error) {
					return nil, errors.NewNotFound(schema.GroupResource{}, "")
				}
				state.grListerMock.GetFunc = func(namespace string, name string) (*v3.GlobalRole, error) {
					return &apisv3.GlobalRole{
						ObjectMeta: metav1.ObjectMeta{
							Name: rbac.GlobalAdmin,
						},
						Builtin: true,
					}, nil
				}
			},
			stateAssertions: func(changes grbTestStateChanges) {
				assert.Equal(changes.t, changes.createdCRB, &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: rbac.GrbCRBName(grb),
					},
					Subjects: []rbacv1.Subject{rbac.GetGRBSubject(grb)},
					RoleRef: rbacv1.RoleRef{
						Name: "cluster-admin",
						Kind: "ClusterRole",
					},
				})
			},
		},
		"admin role already exists": {
			grb: grb,
			stateSetup: func(state *grbTestState) {
				state.crbListerMock.GetFunc = func(namespace, name string) (*rbacv1.ClusterRoleBinding, error) {
					return &rbacv1.ClusterRoleBinding{}, nil
				}
				state.grListerMock.GetFunc = func(namespace string, name string) (*v3.GlobalRole, error) {
					return &apisv3.GlobalRole{
						ObjectMeta: metav1.ObjectMeta{
							Name: rbac.GlobalAdmin,
						},
						Builtin: true,
					}, nil
				}
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
				state.crbListerMock.GetFunc = func(namespace, name string) (*rbacv1.ClusterRoleBinding, error) {
					return nil, err
				}
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
				state.crbClientMock.CreateFunc = func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
					return nil, err
				}
				state.crbListerMock.GetFunc = func(namespace, name string) (*rbacv1.ClusterRoleBinding, error) {
					return nil, errors.NewNotFound(schema.GroupResource{}, "")
				}
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
			crbListerMock := &rbacFakes.ClusterRoleBindingListerMock{}
			crbClientMock := &rbacFakes.ClusterRoleBindingInterfaceMock{}
			stateChanges := &grbTestStateChanges{
				t: t,
			}
			state := &grbTestState{
				grListerMock:  grListerMock,
				crbListerMock: crbListerMock,
				crbClientMock: crbClientMock,
				stateChanges:  stateChanges,
			}
			if test.stateSetup != nil {
				test.stateSetup(state)
			}
			h := grbHandler{
				clusterRoleBindings: crbClientMock,
				crbLister:           crbListerMock,
				grLister:            grListerMock,
			}

			_, err := h.sync("", test.grb)

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
