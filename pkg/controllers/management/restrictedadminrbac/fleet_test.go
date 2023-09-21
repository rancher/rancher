package restrictedadminrbac

import (
	"strings"
	"testing"

	"github.com/pkg/errors"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1/fakes"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/stretchr/testify/assert"
	k8srbac "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_rbaccontroller_ensureRolebinding(t *testing.T) {
	namespace := "fleet-default"
	subject := k8srbac.Subject{
		Kind:      "User",
		Name:      "TestUser",
		Namespace: "",
	}
	grb := &v3.GlobalRoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "GlobalRoleBinding",
			APIVersion: "management.cattle.io/v3",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testGrb",
			Namespace: "testNamespace",
			UID:       "1234",
		},
		UserName:           subject.Name,
		GroupPrincipalName: "",
		GlobalRoleName:     "",
	}
	name := grb.Name + "-fleetworkspace-" + rbac.RestrictedAdminClusterRoleBinding
	ownerRefs := []metav1.OwnerReference{
		{
			APIVersion: grb.TypeMeta.APIVersion,
			Kind:       grb.TypeMeta.Kind,
			UID:        grb.UID,
			Name:       grb.Name,
		},
	}
	roleRef := k8srbac.RoleRef{
		Name: "fleetworkspace-admin",
		Kind: "ClusterRole",
	}
	subjects := []k8srbac.Subject{
		subject,
	}
	expected := &k8srbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			Labels:          map[string]string{rbac.RestrictedAdminClusterRoleBinding: "true"},
			OwnerReferences: ownerRefs,
		},
		RoleRef:  roleRef,
		Subjects: subjects,
	}

	tests := []struct {
		name        string
		setup       func(*mockController)
		wantErr     bool
		expectedErr string
	}{
		{
			name: "no previously existing rolebinding",
			setup: func(c *mockController) {
				c.mockRBLister = &fakes.RoleBindingListerMock{
					GetFunc: func(namespace string, name string) (*k8srbac.RoleBinding, error) {
						return nil, &k8serrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
					},
				}

				c.mockRBInterface = &fakes.RoleBindingInterfaceMock{
					CreateFunc: func(rb *k8srbac.RoleBinding) (*k8srbac.RoleBinding, error) {
						assert.Equal(c.t, rb, expected)
						return expected, nil
					},
				}
			},
			wantErr: false,
		},
		{
			name: "one previously existing incorrect rolebinding (wrong Labels)",
			setup: func(c *mockController) {
				c.mockRBLister = &fakes.RoleBindingListerMock{
					GetFunc: func(namespace string, name string) (*k8srbac.RoleBinding, error) {
						return &k8srbac.RoleBinding{
							ObjectMeta: metav1.ObjectMeta{
								Name:            name,
								Namespace:       namespace,
								Labels:          map[string]string{},
								OwnerReferences: ownerRefs,
							},
							RoleRef:  roleRef,
							Subjects: subjects,
						}, nil
					},
				}

				c.mockRBInterface = &fakes.RoleBindingInterfaceMock{
					UpdateFunc: func(rb *k8srbac.RoleBinding) (*k8srbac.RoleBinding, error) {
						assert.Equal(c.t, rb, expected)
						return expected, nil
					},
				}
			},
			wantErr: false,
		},
		{
			name: "one previously existing incorrect rolebinding (wrong OwnerReferences)",
			setup: func(c *mockController) {
				c.mockRBLister = &fakes.RoleBindingListerMock{
					GetFunc: func(namespace string, name string) (*k8srbac.RoleBinding, error) {
						return &k8srbac.RoleBinding{
							ObjectMeta: metav1.ObjectMeta{
								Name:            name,
								Namespace:       namespace,
								Labels:          map[string]string{rbac.RestrictedAdminClusterRoleBinding: "true"},
								OwnerReferences: []metav1.OwnerReference{},
							},
							RoleRef:  roleRef,
							Subjects: subjects,
						}, nil
					},
				}

				c.mockRBInterface = &fakes.RoleBindingInterfaceMock{
					UpdateFunc: func(rb *k8srbac.RoleBinding) (*k8srbac.RoleBinding, error) {
						assert.Equal(c.t, rb, expected)
						return expected, nil
					},
				}
			},
			wantErr: false,
		},
		{
			name: "one previously existing incorrect rolebinding (wrong RoleRef)",
			setup: func(c *mockController) {
				c.mockRBLister = &fakes.RoleBindingListerMock{
					GetFunc: func(namespace string, name string) (*k8srbac.RoleBinding, error) {
						return &k8srbac.RoleBinding{
							ObjectMeta: metav1.ObjectMeta{
								Name:            name,
								Namespace:       namespace,
								Labels:          map[string]string{rbac.RestrictedAdminClusterRoleBinding: "true"},
								OwnerReferences: ownerRefs,
							},
							RoleRef:  k8srbac.RoleRef{},
							Subjects: subjects,
						}, nil
					},
				}

				c.mockRBInterface = &fakes.RoleBindingInterfaceMock{
					UpdateFunc: func(rb *k8srbac.RoleBinding) (*k8srbac.RoleBinding, error) {
						assert.Equal(c.t, rb, expected)
						return expected, nil
					},
				}
			},
			wantErr: false,
		},
		{
			name: "one previously existing incorrect rolebinding (wrong Subjects)",
			setup: func(c *mockController) {
				c.mockRBLister = &fakes.RoleBindingListerMock{
					GetFunc: func(namespace string, name string) (*k8srbac.RoleBinding, error) {
						return &k8srbac.RoleBinding{
							ObjectMeta: metav1.ObjectMeta{
								Name:            name,
								Namespace:       namespace,
								Labels:          map[string]string{rbac.RestrictedAdminClusterRoleBinding: "true"},
								OwnerReferences: ownerRefs,
							},
							RoleRef:  roleRef,
							Subjects: []k8srbac.Subject{},
						}, nil
					},
				}

				c.mockRBInterface = &fakes.RoleBindingInterfaceMock{
					UpdateFunc: func(rb *k8srbac.RoleBinding) (*k8srbac.RoleBinding, error) {
						assert.Equal(c.t, rb, expected)
						return expected, nil
					},
				}
			},
			wantErr: false,
		},
		{
			name: "one previously existing correct rolebinding",
			setup: func(c *mockController) {
				c.mockRBLister = &fakes.RoleBindingListerMock{
					GetFunc: func(namespace string, name string) (*k8srbac.RoleBinding, error) {
						return expected, nil
					},
				}
			},
			wantErr: false,
		},
		{
			name: "unexpected error in Get call",
			setup: func(c *mockController) {
				c.mockRBLister = &fakes.RoleBindingListerMock{
					GetFunc: func(namespace string, name string) (*k8srbac.RoleBinding, error) {
						return nil, errors.New("Unexpected error ABC")
					},
				}
			},
			wantErr:     true,
			expectedErr: "Unexpected error ABC",
		},
		{
			name: "unexpected error in Create call",
			setup: func(c *mockController) {
				c.mockRBLister = &fakes.RoleBindingListerMock{
					GetFunc: func(namespace string, name string) (*k8srbac.RoleBinding, error) {
						return nil, &k8serrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
					},
				}

				c.mockRBInterface = &fakes.RoleBindingInterfaceMock{
					CreateFunc: func(rb *k8srbac.RoleBinding) (*k8srbac.RoleBinding, error) {
						return nil, errors.New("Unexpected error ABC")
					},
				}
			},
			wantErr:     true,
			expectedErr: "Unexpected error ABC",
		},
		{
			name: "unexpected error in Update call",
			setup: func(c *mockController) {
				c.mockRBLister = &fakes.RoleBindingListerMock{
					GetFunc: func(namespace string, name string) (*k8srbac.RoleBinding, error) {
						return &k8srbac.RoleBinding{
							ObjectMeta: metav1.ObjectMeta{
								Name:            name,
								Namespace:       namespace,
								Labels:          map[string]string{},
								OwnerReferences: ownerRefs,
							},
							RoleRef:  roleRef,
							Subjects: subjects,
						}, nil
					},
				}

				c.mockRBInterface = &fakes.RoleBindingInterfaceMock{
					UpdateFunc: func(rb *k8srbac.RoleBinding) (*k8srbac.RoleBinding, error) {
						return nil, errors.New("Unexpected error ABC")
					},
				}
			},
			wantErr:     true,
			expectedErr: "Unexpected error ABC",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := newMockController(t)
			tt.setup(mockCtrl)
			r := mockCtrl.rbacController()
			err := r.ensureRolebinding(namespace, subject, grb)
			if (err != nil) != tt.wantErr || (tt.wantErr == true && !strings.Contains(err.Error(), tt.expectedErr)) {
				t.Errorf("ensureRolebinding() error = %v, wantErr %v, expectedErr %v", err, tt.wantErr, tt.expectedErr)
			}
		})
	}
}
