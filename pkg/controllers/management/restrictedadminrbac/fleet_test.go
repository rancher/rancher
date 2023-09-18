package restrictedadminrbac

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	k8srbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
		name    string
		setup   func(*mockController)
		wantErr bool
	}{
		{
			name: "no previously existing rolebinding",
			setup: func(c *mockController) {
				c.mockRBLister.EXPECT().
					Get(namespace, name).
					Return(nil, &errors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}})

				c.mockRBInterface.EXPECT().
					Create(expected).
					Return(expected, nil)
			},
			wantErr: false,
		},
		{
			name: "one previously existing incorrect rolebinding",
			setup: func(c *mockController) {
				c.mockRBLister.EXPECT().
					Get(namespace, name).
					Return(&k8srbac.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: namespace,
							Labels:    map[string]string{},
						},
					}, nil)

				c.mockRBInterface.EXPECT().
					Update(expected).
					Return(expected, nil)
			},
			wantErr: false,
		},
		{
			name: "one previously existing correct rolebinding",
			setup: func(c *mockController) {
				c.mockRBLister.EXPECT().
					Get(namespace, name).
					Return(expected, nil)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := newMockController(t)
			tt.setup(mockCtrl)
			r := mockCtrl.rbacController()
			if err := r.ensureRolebinding(namespace, subject, grb); (err != nil) != tt.wantErr {
				t.Errorf("ensureRolebinding() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
