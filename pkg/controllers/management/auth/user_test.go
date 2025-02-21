package auth

import (
	"encoding/json"
	"fmt"
	"testing"

	management "github.com/rancher/rancher/pkg/apis/management.cattle.io"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	fakes "github.com/rancher/rancher/pkg/controllers/management/auth/fakes"
	"github.com/rancher/rancher/pkg/controllers/management/auth/project_cluster"
	exttokens "github.com/rancher/rancher/pkg/ext/stores/tokens"
	wranglerfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
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
					Annotations: map[string]string{project_cluster.CreatorIDAnnotation: "creator"},
				},
				PrincipalIDs: []string{},
			},
			mockSetup: func() {
				mockUserManager.EXPECT().CreateNewUserClusterRoleBinding("testuser", defaultCRTB.UID).Return(nil)
			},
			expectedUser: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "testuser",
					Annotations: map[string]string{project_cluster.CreatorIDAnnotation: "creator"},
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
					Annotations: map[string]string{project_cluster.CreatorIDAnnotation: "creator"},
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
		// The ext token store is set per test case. enables per-test mock setups
	}

	tests := []struct {
		name      string
		inputUser *v3.User
		mockSetup func(
			secrets *wranglerfake.MockControllerInterface[*v1.Secret, *v1.SecretList],
			scache *wranglerfake.MockCacheInterface[*v1.Secret],
			support *exttokens.MocktimeHandler)
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
			mockSetup: func(
				secrets *wranglerfake.MockControllerInterface[*v1.Secret, *v1.SecretList],
				scache *wranglerfake.MockCacheInterface[*v1.Secret],
				support *exttokens.MocktimeHandler) {
				mockUserManager.EXPECT().
					CreateNewUserClusterRoleBinding("testuser", defaultCRTB.UID).
					Return(fmt.Errorf("error updating user"))
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
			mockSetup: func(
				secrets *wranglerfake.MockControllerInterface[*v1.Secret, *v1.SecretList],
				scache *wranglerfake.MockCacheInterface[*v1.Secret],
				support *exttokens.MocktimeHandler) {
				mockUserManager.EXPECT().
					CreateNewUserClusterRoleBinding("testuser", defaultCRTB.UID).
					Return(nil)
				secrets.EXPECT().
					List("cattle-tokens", gomock.Any()).
					Return(&v1.SecretList{}, nil).
					AnyTimes()
			},
			expectedUser: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "testuser",
					Annotations: map[string]string{project_cluster.CreatorIDAnnotation: "creator"},
				},
				PrincipalIDs: []string{"local://testuser"},
			},
			expectedError: false,
		},
		{
			name: "user was updated, login ext token will be deleted",
			inputUser: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testuser",
				},
				PrincipalIDs: []string{},
				Enabled:      pointer.Bool(false),
			},
			mockSetup: func(
				secrets *wranglerfake.MockControllerInterface[*v1.Secret, *v1.SecretList],
				scache *wranglerfake.MockCacheInterface[*v1.Secret],
				support *exttokens.MocktimeHandler) {
				mockUserManager.EXPECT().
					CreateNewUserClusterRoleBinding("testuser", defaultCRTB.UID).
					Return(nil)
				principalBytes, _ := json.Marshal(v3.Principal{
					ObjectMeta:  metav1.ObjectMeta{Name: "world"},
					Provider:    "somebody",
					LoginName:   "hello",
					DisplayName: "myself",
				})
				scache.EXPECT().
					List("cattle-tokens", gomock.Any()).
					Return([]*v1.Secret{
						&v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name: "testuser-token",
							},
							Data: map[string][]byte{
								exttokens.FieldAnnotations:    []byte("null"),
								exttokens.FieldEnabled:        []byte("true"),
								exttokens.FieldHash:           []byte("kla9jkdmj"),
								exttokens.FieldKind:           []byte(exttokens.IsLogin),
								exttokens.FieldLabels:         []byte("null"),
								exttokens.FieldLastUpdateTime: []byte("13:00:05"),
								exttokens.FieldPrincipal:      principalBytes,
								exttokens.FieldTTL:            []byte("4000"),
								exttokens.FieldUID:            []byte("2905498-kafld-lkad"),
								exttokens.FieldUserID:         []byte("testuser"),
							},
						},
					}, nil).AnyTimes()
				secrets.EXPECT().
					Delete("cattle-tokens", "testuser-token", gomock.Any()).
					Return(nil)
			},
			expectedUser: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "testuser",
					Annotations: map[string]string{project_cluster.CreatorIDAnnotation: "creator"},
				},
				PrincipalIDs: []string{"local://testuser"},
			},
			expectedError: false,
		},
		{
			name: "user was updated, derived ext token will be disabled",
			inputUser: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testuser",
				},
				PrincipalIDs: []string{},
				Enabled:      pointer.Bool(false),
			},
			mockSetup: func(
				secrets *wranglerfake.MockControllerInterface[*v1.Secret, *v1.SecretList],
				scache *wranglerfake.MockCacheInterface[*v1.Secret],
				support *exttokens.MocktimeHandler) {
				mockUserManager.EXPECT().
					CreateNewUserClusterRoleBinding("testuser", defaultCRTB.UID).
					Return(nil)
				// Fake current time
				support.EXPECT().Now().Return("this is a fake now")
				secrets.EXPECT().
					Update(gomock.Any()).
					DoAndReturn(func(s *v1.Secret) (*v1.Secret, error) {
						// copy data over for the regen done by the token store
						for k, v := range s.StringData {
							s.Data[k] = []byte(v)
						}

						return s, nil
					})
				principalBytes, _ := json.Marshal(v3.Principal{
					ObjectMeta:  metav1.ObjectMeta{Name: "world"},
					Provider:    "somebody",
					LoginName:   "hello",
					DisplayName: "myself",
				})
				theTokenSecret := v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "testuser-token",
					},
					Data: map[string][]byte{
						exttokens.FieldAnnotations:    []byte("null"),
						exttokens.FieldEnabled:        []byte("true"),
						exttokens.FieldHash:           []byte("kla9jkdmj"),
						exttokens.FieldKind:           []byte(""),
						exttokens.FieldLabels:         []byte("null"),
						exttokens.FieldLastUpdateTime: []byte("13:00:05"),
						exttokens.FieldPrincipal:      principalBytes,
						exttokens.FieldTTL:            []byte("4000"),
						exttokens.FieldUID:            []byte("2905498-kafld-lkad"),
						exttokens.FieldUserID:         []byte("testuser"),
					},
				}
				scache.EXPECT().
					List("cattle-tokens", gomock.Any()).
					Return([]*v1.Secret{&theTokenSecret}, nil).
					AnyTimes()
				scache.EXPECT().
					Get("cattle-tokens", "testuser-token").
					Return(&theTokenSecret, nil).
					AnyTimes()
			},
			expectedUser: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "testuser",
					Annotations: map[string]string{project_cluster.CreatorIDAnnotation: "creator"},
				},
				PrincipalIDs: []string{"local://testuser"},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secrets := wranglerfake.NewMockControllerInterface[*v1.Secret, *v1.SecretList](ctrl)
			scache := wranglerfake.NewMockCacheInterface[*v1.Secret](ctrl)
			secrets.EXPECT().Cache().Return(scache)

			users := wranglerfake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
			users.EXPECT().Cache().Return(nil)

			timer := exttokens.NewMocktimeHandler(ctrl)

			store := exttokens.NewSystem(nil, secrets, users, nil, timer, nil, nil)
			ul.extTokenStore = store

			tt.mockSetup(secrets, scache, timer)

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
	tests := []struct {
		name          string
		inputCRTB     []*v3.ClusterRoleTemplateBinding
		mockSetup     func(crtbMock *wranglerfake.MockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList])
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
			mockSetup: func(crtbMock *wranglerfake.MockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList]) {
				crtbMock.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
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
			mockSetup: func(crtbMock *wranglerfake.MockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList]) {
				crtbMock.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
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
			mockSetup: func(crtbMock *wranglerfake.MockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList]) {
				crtbMock.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
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
			mockSetup: func(crtbMock *wranglerfake.MockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList]) {
				crtbMock.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
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
			mockSetup: func(crtbMock *wranglerfake.MockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList]) {
				gomock.InOrder(
					crtbMock.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil),
					crtbMock.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("namespaced crtb not deleted")),
				)
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
			mockSetup: func(crtbMock *wranglerfake.MockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList]) {
				crtbMock.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("some error"))
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			crtbMock := wranglerfake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)

			tt.mockSetup(crtbMock)

			ul := &userLifecycle{
				crtb: crtbMock,
			}

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
	tests := []struct {
		name          string
		inputPRTB     []*v3.ProjectRoleTemplateBinding
		mockSetup     func(*wranglerfake.MockControllerInterface[*v3.ProjectRoleTemplateBinding, *v3.ProjectRoleTemplateBindingList])
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
			mockSetup: func(prtbMock *wranglerfake.MockControllerInterface[*v3.ProjectRoleTemplateBinding, *v3.ProjectRoleTemplateBindingList]) {
				prtbMock.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
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
			mockSetup: func(prtbMock *wranglerfake.MockControllerInterface[*v3.ProjectRoleTemplateBinding, *v3.ProjectRoleTemplateBindingList]) {
				prtbMock.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
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
			mockSetup: func(prtbMock *wranglerfake.MockControllerInterface[*v3.ProjectRoleTemplateBinding, *v3.ProjectRoleTemplateBindingList]) {
				prtbMock.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("some error"))
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
			mockSetup: func(prtbMock *wranglerfake.MockControllerInterface[*v3.ProjectRoleTemplateBinding, *v3.ProjectRoleTemplateBindingList]) {
				prtbMock.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("some error"))
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			prtbMock := wranglerfake.NewMockControllerInterface[*v3.ProjectRoleTemplateBinding, *v3.ProjectRoleTemplateBindingList](ctrl)

			tt.mockSetup(prtbMock)

			ul := &userLifecycle{
				prtb: prtbMock,
			}
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
	ctrl := gomock.NewController(t)
	namespaceMock := wranglerfake.NewMockNonNamespacedControllerInterface[*v1.Namespace, *v1.NamespaceList](ctrl)
	namespaceListerMock := wranglerfake.NewMockNonNamespacedCacheInterface[*v1.Namespace](ctrl)

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
				namespaceListerMock.EXPECT().Get(gomock.Any()).Return(&v1.Namespace{}, nil)
				namespaceMock.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedError: false,
		},
		{
			name:     "error getting namespace",
			username: "testuser",
			mockSetup: func() {
				namespaceListerMock.EXPECT().Get(gomock.Any()).Return(nil, fmt.Errorf("some error"))
			},
			expectedError: true,
		},
		{
			name:     "error deleting namespace",
			username: "testuser",
			mockSetup: func() {
				namespaceListerMock.EXPECT().Get(gomock.Any()).Return(&v1.Namespace{}, nil)
				namespaceMock.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(fmt.Errorf("some error"))
			},
			expectedError: true,
		},
		{
			name:     "namespace is in termination state",
			username: "testuser",
			mockSetup: func() {
				namespaceListerMock.EXPECT().Get(gomock.Any()).Return(&v1.Namespace{
					Status: v1.NamespaceStatus{
						Phase: v1.NamespaceTerminating,
					},
				}, nil)
			},
			expectedError: false,
		},
		{
			name:     "namespace was not found",
			username: "testuser",
			mockSetup: func() {
				namespaceListerMock.EXPECT().Get(gomock.Any()).Return(nil, errors.NewNotFound(schema.GroupResource{
					Group:    management.GroupName,
					Resource: "Namespace",
				}, "testns"))
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
	ctrl := gomock.NewController(t)
	secretsMock := wranglerfake.NewMockControllerInterface[*v1.Secret, *v1.SecretList](ctrl)
	secretsListerMock := wranglerfake.NewMockCacheInterface[*v1.Secret](ctrl)

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
				secretsListerMock.EXPECT().Get(gomock.Any(), gomock.Any()).Return(&v1.Secret{}, nil)
				secretsMock.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedError: false,
		},
		{
			name:     "error getting secret",
			username: "testuser",
			mockSetup: func() {
				secretsListerMock.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("some error"))
			},
			expectedError: true,
		},
		{
			name:     "error deleting secret",
			username: "testuser",
			mockSetup: func() {
				secretsListerMock.EXPECT().Get(gomock.Any(), gomock.Any()).Return(&v1.Secret{}, nil)
				secretsMock.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("some error"))
			},
			expectedError: true,
		},
		{
			name:     "secret not found",
			username: "testuser",
			mockSetup: func() {
				secretsListerMock.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, errors.NewNotFound(schema.GroupResource{
					Group:    management.GroupName,
					Resource: "Secrets",
				}, "testsecret"))
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
	ctrl := gomock.NewController(t)
	//usersMock := &managementFakes.UserInterfaceMock{}
	usersMock := wranglerfake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)

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
				usersMock.EXPECT().Update(gomock.Any()).Return(
					&v3.User{
						ObjectMeta: metav1.ObjectMeta{
							Name: "testuser",
							Finalizers: []string{
								"controller.cattle.io/test-finalizer",
							},
						},
					}, nil)
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
				usersMock.EXPECT().Update(gomock.Any()).Return(nil, fmt.Errorf("some error"))
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
