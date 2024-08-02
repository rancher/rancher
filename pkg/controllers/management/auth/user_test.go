package auth

import (
	"fmt"
	"testing"

	management "github.com/rancher/rancher/pkg/apis/management.cattle.io"
	fakes "github.com/rancher/rancher/pkg/controllers/management/auth/fakes"
	coreFakes "github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	managementFakes "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	v12 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Test_hasLocalPrincipalID(t *testing.T) {
	type args struct {
		user *v3.User
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "has local PrincipalID",
			args: args{
				user: &v3.User{
					Username: "testuser",
					PrincipalIDs: []string{
						"ID1",
						"ID2",
						"local://testuser",
					},
				},
			},
			want: true,
		},
		{
			name: "has no local PrincipalIDs",
			args: args{
				user: &v3.User{
					Username: "testuser",
					PrincipalIDs: []string{
						"ID1",
						"ID2",
					},
				},
			},
			want: false,
		},
		{
			name: "PrincipalIDs is empty",
			args: args{
				user: &v3.User{
					Username:     "testuser",
					PrincipalIDs: []string{},
				},
			},
			want: false,
		},
		{
			name: "has multiple local PrincipalIDs",
			args: args{
				user: &v3.User{
					Username: "testuser",
					PrincipalIDs: []string{
						"ID1",
						"local://localuser",
						"ID2",
						"local://testuser",
					},
				},
			},
			want: true,
		},
		{
			name: "PrincipalIDs is nil",
			args: args{
				user: &v3.User{
					Username:     "testuser",
					PrincipalIDs: nil,
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasLocalPrincipalID(tt.args.user); got != tt.want {
				t.Errorf("hasValidPrincipalIDs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserManager := fakes.NewMockManager(ctrl)

	ul := &userLifecycle{
		userManager: mockUserManager,
	}

	tests := []struct {
		name          string
		inputUser     *v3.User
		mockSetup     func()
		expectedUser  *v3.User
		expectedError bool
	}{
		{
			name: "User without local principal IDs",
			inputUser: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "testuser",
					Annotations: map[string]string{},
				},
				PrincipalIDs: []string{},
			},
			mockSetup: func() {},
			expectedUser: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "testuser",
					Annotations: map[string]string{},
				},
				PrincipalIDs: []string{"local://testuser"},
			},
			expectedError: false,
		},
		{
			name: "User with creatorID annotation and successful role binding",
			inputUser: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "testuser",
					UID:         defaultCRTB.UID,
					Annotations: map[string]string{creatorIDAnn: "creator"},
				},
				PrincipalIDs: []string{},
			},
			mockSetup: func() {
				mockUserManager.EXPECT().CreateNewUserClusterRoleBinding("testuser", defaultCRTB.UID).Return(nil)
			},
			expectedUser: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "testuser",
					Annotations: map[string]string{creatorIDAnn: "creator"},
				},
				PrincipalIDs: []string{"local://testuser"},
			},
			expectedError: false,
		},
		{
			name: "User with creatorID annotation and role binding error",
			inputUser: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "testuser",
					Annotations: map[string]string{creatorIDAnn: "creator"},
				},
				PrincipalIDs: []string{},
			},
			mockSetup: func() {
				mockUserManager.EXPECT().CreateNewUserClusterRoleBinding("testuser", defaultCRTB.UID).Return(fmt.Errorf("role binding error"))
			},
			expectedUser:  nil,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			_, err := ul.Create(tt.inputUser)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpdated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserManager := fakes.NewMockManager(ctrl)

	ul := &userLifecycle{
		userManager: mockUserManager,
	}

	tests := []struct {
		name          string
		inputUser     *v3.User
		mockSetup     func()
		expectedUser  *v3.User
		expectedError bool
	}{
		{
			name: "user was not updated properly",
			inputUser: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testuser",
				},
				PrincipalIDs: []string{},
			},
			mockSetup: func() {
				mockUserManager.EXPECT().CreateNewUserClusterRoleBinding("testuser", defaultCRTB.UID).Return(fmt.Errorf("error updating user"))
			},
			expectedUser:  nil,
			expectedError: true,
		},
		{
			name: "user was updated",
			inputUser: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testuser",
				},
				PrincipalIDs: []string{},
			},
			mockSetup: func() {
				mockUserManager.EXPECT().CreateNewUserClusterRoleBinding("testuser", defaultCRTB.UID).Return(nil)
			},
			expectedUser: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "testuser",
					Annotations: map[string]string{creatorIDAnn: "creator"},
				},
				PrincipalIDs: []string{"local://testuser"},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			_, err := ul.Updated(tt.inputUser)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_deleteAllCRTB(t *testing.T) {
	ctrbMock := &managementFakes.ClusterRoleTemplateBindingInterfaceMock{}

	ul := &userLifecycle{
		crtb: ctrbMock,
	}

	tests := []struct {
		name          string
		inputCRTB     []*v3.ClusterRoleTemplateBinding
		mockSetup     func()
		expectedError bool
	}{
		{
			name: "crtb deleted properly",
			inputCRTB: []*v3.ClusterRoleTemplateBinding{
				&v3.ClusterRoleTemplateBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "testuser",
					},
				},
			},
			mockSetup: func() {
				ctrbMock.DeleteFunc = func(name string, options *metav1.DeleteOptions) error {
					return nil
				}
			},
			expectedError: false,
		},
		{
			name: "crtbs deleted properly",
			inputCRTB: []*v3.ClusterRoleTemplateBinding{
				&v3.ClusterRoleTemplateBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "testuser",
					},
				},
				&v3.ClusterRoleTemplateBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "testuser-2",
					},
				},
			},
			mockSetup: func() {
				ctrbMock.DeleteFunc = func(name string, options *metav1.DeleteOptions) error {
					return nil
				}
			},
			expectedError: false,
		},
		{
			name: "namespaced crtbs deleted properly",
			inputCRTB: []*v3.ClusterRoleTemplateBinding{
				&v3.ClusterRoleTemplateBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testuser",
						Namespace: "testnamespace",
					},
				},
				&v3.ClusterRoleTemplateBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testuser-2",
						Namespace: "testnamespace",
					},
				},
			},
			mockSetup: func() {
				ctrbMock.DeleteNamespacedFunc = func(namespace, name string, options *metav1.DeleteOptions) error {
					return nil
				}
			},
			expectedError: false,
		},
		{
			name: "crtbs (non and namespaced) deleted properly",
			inputCRTB: []*v3.ClusterRoleTemplateBinding{
				&v3.ClusterRoleTemplateBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "testuser",
					},
				},
				&v3.ClusterRoleTemplateBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testuser-2",
						Namespace: "testnamespace",
					},
				},
			},
			mockSetup: func() {
				ctrbMock.DeleteFunc = func(name string, options *metav1.DeleteOptions) error {
					return nil
				}
				ctrbMock.DeleteNamespacedFunc = func(namespace, name string, options *metav1.DeleteOptions) error {
					return nil
				}
			},
			expectedError: false,
		},
		{
			name: "crtbs (non and namespaced) not deleted properly",
			inputCRTB: []*v3.ClusterRoleTemplateBinding{
				&v3.ClusterRoleTemplateBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "testuser",
					},
				},
				&v3.ClusterRoleTemplateBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testuser-2",
						Namespace: "testnamespace",
					},
				},
			},
			mockSetup: func() {
				ctrbMock.DeleteFunc = func(name string, options *metav1.DeleteOptions) error {
					return nil
				}
				ctrbMock.DeleteNamespacedFunc = func(namespace, name string, options *metav1.DeleteOptions) error {
					return fmt.Errorf("namespaced crtb not deleted")
				}
			},
			expectedError: true,
		},
		{
			name: "crtbs not deleted properly",
			inputCRTB: []*v3.ClusterRoleTemplateBinding{
				&v3.ClusterRoleTemplateBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "testuser",
					},
				},
			},
			mockSetup: func() {
				ctrbMock.DeleteFunc = func(name string, options *metav1.DeleteOptions) error {
					return fmt.Errorf("some error")
				}
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			err := ul.deleteAllCRTB(tt.inputCRTB)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_deleteAllPRTB(t *testing.T) {
	prtbMock := &managementFakes.ProjectRoleTemplateBindingInterfaceMock{}

	ul := &userLifecycle{
		prtb: prtbMock,
	}

	tests := []struct {
		name          string
		inputPRTB     []*v3.ProjectRoleTemplateBinding
		mockSetup     func()
		expectedError bool
	}{
		{
			name: "remove namespaced prtb",
			inputPRTB: []*v3.ProjectRoleTemplateBinding{
				&v3.ProjectRoleTemplateBinding{
					UserName: "testuser",
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testprtb",
						Namespace: "testprtbns",
					},
				},
			},
			mockSetup: func() {
				prtbMock.DeleteNamespacedFunc = func(namespace, name string, options *metav1.DeleteOptions) error {
					return nil
				}
			},
			expectedError: false,
		},
		{
			name: "remove all prtb",
			inputPRTB: []*v3.ProjectRoleTemplateBinding{
				&v3.ProjectRoleTemplateBinding{
					UserName: "testuser",
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testprtb",
						Namespace: "testprtbns",
					},
				},
				&v3.ProjectRoleTemplateBinding{
					UserName: "testuser2",
					ObjectMeta: metav1.ObjectMeta{
						Name: "testprtb2",
					},
				},
			},
			mockSetup: func() {
				prtbMock.DeleteNamespacedFunc = func(namespace, name string, options *metav1.DeleteOptions) error {
					return nil
				}
				prtbMock.DeleteFunc = func(name string, options *metav1.DeleteOptions) error {
					return nil
				}
			},
			expectedError: false,
		},
		{
			name: "error deleting namespaced prtb",
			inputPRTB: []*v3.ProjectRoleTemplateBinding{
				&v3.ProjectRoleTemplateBinding{
					UserName: "testuser",
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testprtb",
						Namespace: "testprtbns",
					},
				},
			},
			mockSetup: func() {
				prtbMock.DeleteNamespacedFunc = func(namespace, name string, options *metav1.DeleteOptions) error {
					return fmt.Errorf("some error")
				}
			},
			expectedError: true,
		},
		{
			name: "error deleting prtb",
			inputPRTB: []*v3.ProjectRoleTemplateBinding{
				&v3.ProjectRoleTemplateBinding{
					UserName: "testuser",
					ObjectMeta: metav1.ObjectMeta{
						Name: "testprtb",
					},
				},
			},
			mockSetup: func() {
				prtbMock.DeleteFunc = func(name string, options *metav1.DeleteOptions) error {
					return fmt.Errorf("some error")
				}
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			err := ul.deleteAllPRTB(tt.inputPRTB)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_deleteUserNamespace(t *testing.T) {
	namespaceMock := &coreFakes.NamespaceInterfaceMock{}
	namespaceListerMock := &coreFakes.NamespaceListerMock{}

	ul := &userLifecycle{
		namespaces:      namespaceMock,
		namespaceLister: namespaceListerMock,
	}

	tests := []struct {
		name          string
		username      string
		mockSetup     func()
		expectedError bool
	}{
		{
			name:     "delete namespace",
			username: "testuser",
			mockSetup: func() {
				namespaceListerMock.GetFunc = func(namespace, name string) (*v12.Namespace, error) {
					return &v12.Namespace{}, nil
				}
				namespaceMock.DeleteFunc = func(name string, options *metav1.DeleteOptions) error {
					return nil
				}
			},
			expectedError: false,
		},
		{
			name:     "error getting namespace",
			username: "testuser",
			mockSetup: func() {
				namespaceListerMock.GetFunc = func(namespace, name string) (*v12.Namespace, error) {
					return nil, fmt.Errorf("some error")
				}
			},
			expectedError: true,
		},
		{
			name:     "error deleting namespace",
			username: "testuser",
			mockSetup: func() {
				namespaceListerMock.GetFunc = func(namespace, name string) (*v12.Namespace, error) {
					return &v12.Namespace{}, nil
				}
				namespaceMock.DeleteFunc = func(name string, options *metav1.DeleteOptions) error {
					return fmt.Errorf("some error")
				}
			},
			expectedError: true,
		},
		{
			name:     "namespace is in termination state",
			username: "testuser",
			mockSetup: func() {
				namespaceListerMock.GetFunc = func(namespace, name string) (*v12.Namespace, error) {
					return &v12.Namespace{
						Status: v12.NamespaceStatus{
							Phase: v12.NamespaceTerminating,
						},
					}, nil
				}
			},
			expectedError: false,
		},
		{
			name:     "namespace was not found",
			username: "testuser",
			mockSetup: func() {
				namespaceListerMock.GetFunc = func(namespace, name string) (*v12.Namespace, error) {
					return nil, errors.NewNotFound(schema.GroupResource{
						Group:    management.GroupName,
						Resource: "Namespace",
					}, "testns")
				}
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			err := ul.deleteUserNamespace(tt.username)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_deleteUserSecret(t *testing.T) {
	secretsMock := &coreFakes.SecretInterfaceMock{}
	secretsListerMock := &coreFakes.SecretListerMock{}

	ul := &userLifecycle{
		secrets:       secretsMock,
		secretsLister: secretsListerMock,
	}

	tests := []struct {
		name          string
		username      string
		mockSetup     func()
		expectedError bool
	}{
		{
			name:     "delete secret",
			username: "testuser",
			mockSetup: func() {
				secretsListerMock.GetFunc = func(namespace, name string) (*v12.Secret, error) {
					return &v12.Secret{}, nil
				}
				secretsMock.DeleteNamespacedFunc = func(namespace, name string, options *metav1.DeleteOptions) error {
					return nil
				}
			},
			expectedError: false,
		},
		{
			name:     "error getting secret",
			username: "testuser",
			mockSetup: func() {
				secretsListerMock.GetFunc = func(namespace, name string) (*v12.Secret, error) {
					return nil, fmt.Errorf("some error")
				}
			},
			expectedError: true,
		},
		{
			name:     "error deleting secret",
			username: "testuser",
			mockSetup: func() {
				secretsListerMock.GetFunc = func(namespace, name string) (*v12.Secret, error) {
					return &v12.Secret{}, nil
				}
				secretsMock.DeleteNamespacedFunc = func(namespace, name string, options *metav1.DeleteOptions) error {
					return fmt.Errorf("some error")
				}
			},
			expectedError: true,
		},
		{
			name:     "secret not found",
			username: "testuser",
			mockSetup: func() {
				secretsListerMock.GetFunc = func(namespace, name string) (*v12.Secret, error) {
					return nil, errors.NewNotFound(schema.GroupResource{
						Group:    management.GroupName,
						Resource: "Secrets",
					}, "testsecret")
				}
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			err := ul.deleteUserSecret(tt.username)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_removeLegacyFinalizers(t *testing.T) {
	usersMock := &managementFakes.UserInterfaceMock{}

	ul := &userLifecycle{
		users: usersMock,
	}

	tests := []struct {
		name          string
		user          *v3.User
		mockSetup     func()
		expectedError bool
	}{
		{
			name: "no need to remove finalizers",
			user: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testuser",
					Finalizers: []string{
						"controller.cattle.io/test-finalizer",
					},
				},
			},
			mockSetup:     func() {},
			expectedError: false,
		},
		{
			name: "remove desired finalizer",
			user: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testuser",
					Finalizers: []string{
						"controller.cattle.io/test-finalizer",
						"controller.cattle.io/cat-user-controller",
					},
				},
			},
			mockSetup: func() {
				usersMock.UpdateFunc = func(in1 *v3.User) (*v3.User, error) {
					return &v3.User{
						ObjectMeta: metav1.ObjectMeta{
							Name: "testuser",
							Finalizers: []string{
								"controller.cattle.io/test-finalizer",
							},
						},
					}, nil
				}
			},
			expectedError: false,
		},
		{
			name: "got error when updating user",
			user: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testuser",
					Finalizers: []string{
						"controller.cattle.io/test-finalizer",
						"controller.cattle.io/cat-user-controller",
					},
				},
			},
			mockSetup: func() {
				usersMock.UpdateFunc = func(in1 *v3.User) (*v3.User, error) {
					return nil, fmt.Errorf("some error")
				}
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			user, err := ul.removeLegacyFinalizers(tt.user)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotContains(t, user.Finalizers, "controller.cattle.io/cat-user-controller")
			}
		})
	}
}
