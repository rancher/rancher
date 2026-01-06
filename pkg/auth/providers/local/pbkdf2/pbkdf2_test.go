package pbkdf2

import (
	"encoding/json"
	"errors"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestCreatePassword(t *testing.T) {
	ctlr := gomock.NewController(t)
	fakeUserID := "fake-user-id"
	fakePassword := "fake-password"
	fakePasswordHash := "fake-password-hash"
	fakePasswordSalt := "fake-password-salt"
	fakeUUID := "fake-uuid"
	fakeUser := &v3.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: fakeUserID,
			UID:  types.UID(fakeUUID),
		},
	}

	tests := map[string]struct {
		user               *v3.User
		password           string
		mockHashKey        func(password string, salt []byte, iter, keyLength int) ([]byte, error)
		mockSaltGenerator  func() ([]byte, error)
		mockSecretClient   func() *fake.MockClientInterface[*v1.Secret, *v1.SecretList]
		mockUserCache      func() *fake.MockNonNamespacedCacheInterface[*v3.User]
		expectErrorMessage string
	}{
		"a secret with the hashed password and salt is created": {
			user:     fakeUser,
			password: fakePassword,
			mockHashKey: func(password string, salt []byte, iter, keyLength int) ([]byte, error) {
				return []byte(fakePasswordHash), nil
			},
			mockSaltGenerator: func() ([]byte, error) {
				return []byte(fakePasswordSalt), nil
			},
			mockUserCache: func() *fake.MockNonNamespacedCacheInterface[*v3.User] {
				mock := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctlr)
				mock.EXPECT().Get(fakeUserID).Return(fakeUser, nil)

				return mock
			},
			mockSecretClient: func() *fake.MockClientInterface[*v1.Secret, *v1.SecretList] {
				mock := fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctlr)
				mock.EXPECT().Create(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fakeUserID,
						Namespace: LocalUserPasswordsNamespace,
						Annotations: map[string]string{
							passwordHashAnnotation: pbkdf2sha3512Hash,
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       fakeUser.Name,
								UID:        fakeUser.UID,
								APIVersion: "management.cattle.io/v3",
								Kind:       "User",
							},
						},
					},
					Data: map[string][]byte{
						"password": []byte(fakePasswordHash),
						"salt":     []byte(fakePasswordSalt),
					},
				}).Return(nil, nil)
				return mock
			},
		},
		"error when salt can't be generated": {
			user:     fakeUser,
			password: fakePassword,
			mockSaltGenerator: func() ([]byte, error) {
				return nil, errors.New("unexpected error")
			},
			mockUserCache: func() *fake.MockNonNamespacedCacheInterface[*v3.User] {
				mock := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctlr)
				mock.EXPECT().Get(fakeUserID).Return(fakeUser, nil)

				return mock
			},
			mockSecretClient: func() *fake.MockClientInterface[*v1.Secret, *v1.SecretList] {
				return fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctlr)
			},
			expectErrorMessage: "failed to generate salt: unexpected error",
		},
		"error when creating a hash for a password": {
			user:     fakeUser,
			password: fakePassword,
			mockHashKey: func(password string, salt []byte, iter, keyLength int) ([]byte, error) {
				return nil, errors.New("unexpected error")
			},
			mockSaltGenerator: func() ([]byte, error) {
				return []byte(fakePasswordSalt), nil
			},
			mockUserCache: func() *fake.MockNonNamespacedCacheInterface[*v3.User] {
				mock := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctlr)
				mock.EXPECT().Get(fakeUserID).Return(fakeUser, nil)

				return mock
			},
			mockSecretClient: func() *fake.MockClientInterface[*v1.Secret, *v1.SecretList] {
				return fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctlr)
			},
			expectErrorMessage: "failed to hash password: unexpected error",
		},
		"error when secret can't be created": {
			user:     fakeUser,
			password: fakePassword,
			mockHashKey: func(password string, salt []byte, iter, keyLength int) ([]byte, error) {
				return []byte(fakePasswordHash), nil
			},
			mockSaltGenerator: func() ([]byte, error) {
				return []byte(fakePasswordSalt), nil
			},
			mockUserCache: func() *fake.MockNonNamespacedCacheInterface[*v3.User] {
				mock := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctlr)
				mock.EXPECT().Get(fakeUserID).Return(fakeUser, nil)

				return mock
			},
			mockSecretClient: func() *fake.MockClientInterface[*v1.Secret, *v1.SecretList] {
				mock := fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctlr)
				mock.EXPECT().Create(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fakeUserID,
						Namespace: LocalUserPasswordsNamespace,
						Annotations: map[string]string{
							passwordHashAnnotation: pbkdf2sha3512Hash,
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       fakeUser.Name,
								UID:        fakeUser.UID,
								APIVersion: "management.cattle.io/v3",
								Kind:       "User",
							},
						},
					},
					Data: map[string][]byte{
						"password": []byte(fakePasswordHash),
						"salt":     []byte(fakePasswordSalt),
					},
				}).Return(nil, errors.New("unexpected error"))
				return mock
			},
			expectErrorMessage: "failed to create secret: unexpected error",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := Pbkdf2{
				secretClient:  test.mockSecretClient(),
				hashKey:       test.mockHashKey,
				saltGenerator: test.mockSaltGenerator,
			}
			err := p.CreatePassword(test.user, test.password)
			if test.expectErrorMessage == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.expectErrorMessage)
			}
		})
	}
}

func TestUpdatePassword(t *testing.T) {
	ctlr := gomock.NewController(t)
	fakeUserID := "fake-user-id"
	fakePassword := "fake-password"
	fakeNewPasswordHash := "fake-new-password-hash"
	fakePasswordSalt := "fake-password-salt"
	fakeNewPasswordSalt := "fake-new-password-salt"
	fakeNewPasswordBcryptHash := "fake-new-password-bcrypt-hash"

	tests := map[string]struct {
		userID             string
		password           string
		mockHashKey        func(password string, salt []byte, iter, keyLength int) ([]byte, error)
		mockBcryptKey      func(password []byte, cost int) ([]byte, error)
		mockSecretClient   func() *fake.MockClientInterface[*v1.Secret, *v1.SecretList]
		mockSecretCache    func() *fake.MockCacheInterface[*v1.Secret]
		mockSaltGenerator  func() ([]byte, error)
		expectErrorMessage string
	}{
		"the secret is updated with the new hashed password": {
			userID:   fakeUserID,
			password: fakePassword,
			mockHashKey: func(password string, salt []byte, iter, keyLength int) ([]byte, error) {
				return []byte(fakeNewPasswordHash), nil
			},
			mockSecretCache: func() *fake.MockCacheInterface[*v1.Secret] {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctlr)
				mock.EXPECT().Get(LocalUserPasswordsNamespace, fakeUserID).Return(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fakeUserID,
						Namespace: LocalUserPasswordsNamespace,
						Annotations: map[string]string{
							passwordHashAnnotation: pbkdf2sha3512Hash,
						},
					},
					Data: map[string][]byte{
						"salt": []byte(fakePasswordSalt),
					},
				}, nil)

				return mock
			},
			mockSecretClient: func() *fake.MockClientInterface[*v1.Secret, *v1.SecretList] {
				mock := fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctlr)
				patch, _ := json.Marshal([]struct {
					Op    string `json:"op"`
					Path  string `json:"path"`
					Value any    `json:"value"`
				}{{
					Op:   "replace",
					Path: "/data",
					Value: map[string][]byte{
						"password": []byte(fakeNewPasswordHash),
						"salt":     []byte(fakeNewPasswordSalt),
					},
				}})
				mock.EXPECT().Patch(LocalUserPasswordsNamespace, fakeUserID, types.JSONPatchType, patch).Return(nil, nil)

				return mock
			},
			mockSaltGenerator: func() ([]byte, error) {
				return []byte(fakeNewPasswordSalt), nil
			},
		},
		"the bcrypt secret is updated with the new bcrypt hashed password": {
			userID:   fakeUserID,
			password: fakePassword,
			mockBcryptKey: func(_ []byte, _ int) ([]byte, error) {
				return []byte(fakeNewPasswordBcryptHash), nil
			},
			mockSecretCache: func() *fake.MockCacheInterface[*v1.Secret] {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctlr)
				mock.EXPECT().Get(LocalUserPasswordsNamespace, fakeUserID).Return(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fakeUserID,
						Namespace: LocalUserPasswordsNamespace,
						Annotations: map[string]string{
							passwordHashAnnotation: bcryptHash,
						},
					},
					Data: map[string][]byte{},
				}, nil)

				return mock
			},
			mockSecretClient: func() *fake.MockClientInterface[*v1.Secret, *v1.SecretList] {
				mock := fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctlr)
				patch, _ := json.Marshal([]struct {
					Op    string `json:"op"`
					Path  string `json:"path"`
					Value any    `json:"value"`
				}{{
					Op:   "replace",
					Path: "/data",
					Value: map[string][]byte{
						"password": []byte(fakeNewPasswordBcryptHash),
					},
				}})
				mock.EXPECT().Patch(LocalUserPasswordsNamespace, fakeUserID, types.JSONPatchType, patch).Return(nil, nil)

				return mock
			},
			mockSaltGenerator: func() ([]byte, error) {
				return []byte(fakeNewPasswordSalt), nil
			},
		},
		"error when secret can't be fetched": {
			userID:   fakeUserID,
			password: fakePassword,
			mockSecretCache: func() *fake.MockCacheInterface[*v1.Secret] {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctlr)
				mock.EXPECT().Get(LocalUserPasswordsNamespace, fakeUserID).Return(nil, errors.New("unexpected error"))

				return mock
			},
			mockSecretClient: func() *fake.MockClientInterface[*v1.Secret, *v1.SecretList] {
				return fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctlr)
			},
			mockSaltGenerator: func() ([]byte, error) {
				return []byte(fakeNewPasswordSalt), nil
			},
			expectErrorMessage: "failed to get password secret: unexpected error",
		},
		"error when creating a hash for a password": {
			userID:   fakeUserID,
			password: fakePassword,
			mockHashKey: func(password string, salt []byte, iter, keyLength int) ([]byte, error) {
				return nil, errors.New("unexpected error")
			},
			mockSecretCache: func() *fake.MockCacheInterface[*v1.Secret] {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctlr)
				mock.EXPECT().Get(LocalUserPasswordsNamespace, fakeUserID).Return(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fakeUserID,
						Namespace: LocalUserPasswordsNamespace,
						Annotations: map[string]string{
							passwordHashAnnotation: pbkdf2sha3512Hash,
						},
					},
					Data: map[string][]byte{
						"salt": []byte(fakePasswordSalt),
					},
				}, nil)

				return mock
			},
			mockSecretClient: func() *fake.MockClientInterface[*v1.Secret, *v1.SecretList] {
				return fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctlr)
			},
			mockSaltGenerator: func() ([]byte, error) {
				return []byte(fakeNewPasswordSalt), nil
			},
			expectErrorMessage: "failed to hash password: unexpected error",
		},
		"error when secret can't be patched": {
			userID:   fakeUserID,
			password: fakePassword,
			mockHashKey: func(password string, salt []byte, iter, keyLength int) ([]byte, error) {
				return []byte(fakeNewPasswordHash), nil
			},
			mockSecretCache: func() *fake.MockCacheInterface[*v1.Secret] {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctlr)
				mock.EXPECT().Get(LocalUserPasswordsNamespace, fakeUserID).Return(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fakeUserID,
						Namespace: LocalUserPasswordsNamespace,
						Annotations: map[string]string{
							passwordHashAnnotation: pbkdf2sha3512Hash,
						},
					},
					Data: map[string][]byte{
						"salt": []byte(fakePasswordSalt),
					},
				}, nil)

				return mock
			},
			mockSecretClient: func() *fake.MockClientInterface[*v1.Secret, *v1.SecretList] {
				mock := fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctlr)
				patch, _ := json.Marshal([]struct {
					Op    string `json:"op"`
					Path  string `json:"path"`
					Value any    `json:"value"`
				}{{
					Op:   "replace",
					Path: "/data",
					Value: map[string][]byte{
						"password": []byte(fakeNewPasswordHash),
						"salt":     []byte(fakeNewPasswordSalt),
					},
				}})
				mock.EXPECT().Patch(LocalUserPasswordsNamespace, fakeUserID, types.JSONPatchType, patch).Return(nil, errors.New("unexpected error"))

				return mock
			},
			mockSaltGenerator: func() ([]byte, error) {
				return []byte(fakeNewPasswordSalt), nil
			},
			expectErrorMessage: "failed to patch secret: unexpected error",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := Pbkdf2{
				secretClient:  test.mockSecretClient(),
				secretLister:  test.mockSecretCache(),
				hashKey:       test.mockHashKey,
				bcryptKey:     test.mockBcryptKey,
				saltGenerator: test.mockSaltGenerator,
			}
			err := p.UpdatePassword(test.userID, test.password)
			if test.expectErrorMessage == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.expectErrorMessage)
			}
		})
	}
}

func TestVerifyAndUpdatePassword(t *testing.T) {
	ctlr := gomock.NewController(t)
	fakeUserID := "fake-user-id"
	fakeNewPassword := "fake-new-password"
	fakeCurrentPassword := "fake-current-password"
	fakeNewPasswordHash := "fake-new-password-hash"
	fakeCurrentPasswordHash := "fake-password-hash"
	fakePasswordSalt := "fake-password-salt"
	fakeNewPasswordSalt := "fake-password-new-salt"

	tests := map[string]struct {
		userID             string
		currentPassword    string
		newPassword        string
		mockHashKey        func(password string, salt []byte, iter, keyLength int) ([]byte, error)
		mockSecretClient   func() *fake.MockClientInterface[*v1.Secret, *v1.SecretList]
		mockSecretCache    func() *fake.MockCacheInterface[*v1.Secret]
		expectErrorMessage string
		mockSaltGenerator  func() ([]byte, error)
	}{
		"the secret is updated with the new hashed password": {
			userID:          fakeUserID,
			currentPassword: fakeCurrentPassword,
			newPassword:     fakeNewPassword,
			mockHashKey: func(password string, salt []byte, iter, keyLength int) ([]byte, error) {
				if password == fakeNewPassword {
					return []byte(fakeNewPasswordHash), nil
				}
				if password == fakeCurrentPassword {
					return []byte(fakeCurrentPasswordHash), nil
				}

				return nil, errors.New("unexpected password")
			},
			mockSecretCache: func() *fake.MockCacheInterface[*v1.Secret] {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctlr)
				mock.EXPECT().Get(LocalUserPasswordsNamespace, fakeUserID).Return(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fakeUserID,
						Namespace: LocalUserPasswordsNamespace,
						Annotations: map[string]string{
							passwordHashAnnotation: pbkdf2sha3512Hash,
						},
					},
					Data: map[string][]byte{
						"salt":     []byte(fakePasswordSalt),
						"password": []byte(fakeCurrentPasswordHash),
					},
				}, nil).Times(2)

				return mock
			},
			mockSecretClient: func() *fake.MockClientInterface[*v1.Secret, *v1.SecretList] {
				mock := fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctlr)
				patch, _ := json.Marshal([]struct {
					Op    string `json:"op"`
					Path  string `json:"path"`
					Value any    `json:"value"`
				}{{
					Op:   "replace",
					Path: "/data",
					Value: map[string][]byte{
						"password": []byte(fakeNewPasswordHash),
						"salt":     []byte(fakeNewPasswordSalt),
					},
				}})
				mock.EXPECT().Patch(LocalUserPasswordsNamespace, fakeUserID, types.JSONPatchType, patch).Return(nil, nil)
				return mock
			},
			mockSaltGenerator: func() ([]byte, error) {
				return []byte(fakeNewPasswordSalt), nil
			},
		},
		"error when secret can't be fetched": {
			userID:          fakeUserID,
			currentPassword: fakeNewPassword,
			mockSecretCache: func() *fake.MockCacheInterface[*v1.Secret] {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctlr)
				mock.EXPECT().Get(LocalUserPasswordsNamespace, fakeUserID).Return(nil, errors.New("unexpected error"))

				return mock
			},
			mockSecretClient: func() *fake.MockClientInterface[*v1.Secret, *v1.SecretList] {
				return fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctlr)
			},
			mockSaltGenerator: func() ([]byte, error) {
				return []byte(fakeNewPasswordSalt), nil
			},
			expectErrorMessage: "failed to get password secret: unexpected error",
		},
		"error when creating a hash for a password": {
			userID:          fakeUserID,
			currentPassword: fakeNewPassword,
			mockHashKey: func(password string, salt []byte, iter, keyLength int) ([]byte, error) {
				return nil, errors.New("unexpected error")
			},
			mockSecretCache: func() *fake.MockCacheInterface[*v1.Secret] {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctlr)
				mock.EXPECT().Get(LocalUserPasswordsNamespace, fakeUserID).Return(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fakeUserID,
						Namespace: LocalUserPasswordsNamespace,
						Annotations: map[string]string{
							passwordHashAnnotation: pbkdf2sha3512Hash,
						},
					},
					Data: map[string][]byte{
						"salt": []byte(fakePasswordSalt),
					},
				}, nil).Times(1)

				return mock
			},
			mockSecretClient: func() *fake.MockClientInterface[*v1.Secret, *v1.SecretList] {
				return fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctlr)
			},
			mockSaltGenerator: func() ([]byte, error) {
				return []byte(fakeNewPasswordSalt), nil
			},
			expectErrorMessage: "failed to hash password: unexpected error",
		},
		"error when secret can't be patched": {
			userID:          fakeUserID,
			currentPassword: fakeCurrentPassword,
			newPassword:     fakeNewPassword,
			mockHashKey: func(password string, salt []byte, iter, keyLength int) ([]byte, error) {
				if password == fakeNewPassword {
					return []byte(fakeNewPasswordHash), nil
				}
				if password == fakeCurrentPassword {
					return []byte(fakeCurrentPasswordHash), nil
				}

				return nil, errors.New("unexpected password")
			},
			mockSecretCache: func() *fake.MockCacheInterface[*v1.Secret] {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctlr)
				mock.EXPECT().Get(LocalUserPasswordsNamespace, fakeUserID).Return(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fakeUserID,
						Namespace: LocalUserPasswordsNamespace,
						Annotations: map[string]string{
							passwordHashAnnotation: pbkdf2sha3512Hash,
						},
					},
					Data: map[string][]byte{
						"salt":     []byte(fakePasswordSalt),
						"password": []byte(fakeCurrentPasswordHash),
					},
				}, nil).Times(2)

				return mock
			},
			mockSecretClient: func() *fake.MockClientInterface[*v1.Secret, *v1.SecretList] {
				mock := fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctlr)
				patch, _ := json.Marshal([]struct {
					Op    string `json:"op"`
					Path  string `json:"path"`
					Value any    `json:"value"`
				}{{
					Op:   "replace",
					Path: "/data",
					Value: map[string][]byte{
						"password": []byte(fakeNewPasswordHash),
						"salt":     []byte(fakeNewPasswordSalt),
					},
				}})
				mock.EXPECT().Patch(LocalUserPasswordsNamespace, fakeUserID, types.JSONPatchType, patch).Return(nil, errors.New("unexpected error"))

				return mock
			},
			mockSaltGenerator: func() ([]byte, error) {
				return []byte(fakeNewPasswordSalt), nil
			},
			expectErrorMessage: "failed to patch secret: unexpected error",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := Pbkdf2{
				secretClient:  test.mockSecretClient(),
				secretLister:  test.mockSecretCache(),
				hashKey:       test.mockHashKey,
				saltGenerator: test.mockSaltGenerator,
			}
			err := p.VerifyAndUpdatePassword(test.userID, test.currentPassword, test.newPassword)
			if test.expectErrorMessage == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.expectErrorMessage)
			}
		})
	}
}

func TestVerifyPassword(t *testing.T) {
	ctlr := gomock.NewController(t)
	fakeUserID := "fake-user-id"
	fakePassword := "fake-password"
	fakePasswordHash := "fake-password-hash"
	fakePasswordSalt := "fake-password-salt"
	bcryptHashPassword, _ := bcrypt.GenerateFromPassword([]byte(fakePassword), bcrypt.DefaultCost)

	tests := map[string]struct {
		user               *v3.User
		password           string
		mockHashKey        func(password string, salt []byte, iter, keyLength int) ([]byte, error)
		mockSecretCache    func() *fake.MockCacheInterface[*v1.Secret]
		mockSecretClient   func() *fake.MockClientInterface[*v1.Secret, *v1.SecretList]
		expectErrorMessage string
		mockSaltGenerator  func() ([]byte, error)
	}{
		"verify valid password": {
			user: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: fakeUserID,
				},
			},
			password: fakePassword,
			mockHashKey: func(password string, salt []byte, iter, keyLength int) ([]byte, error) {
				if password == fakePassword {
					return []byte(fakePasswordHash), nil
				}
				return nil, errors.New("unexpected password")
			},
			mockSecretCache: func() *fake.MockCacheInterface[*v1.Secret] {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctlr)
				mock.EXPECT().Get(LocalUserPasswordsNamespace, fakeUserID).Return(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fakeUserID,
						Namespace: LocalUserPasswordsNamespace,
						Annotations: map[string]string{
							passwordHashAnnotation: pbkdf2sha3512Hash,
						},
					},
					Data: map[string][]byte{
						"salt":     []byte(fakePasswordSalt),
						"password": []byte(fakePasswordHash),
					},
				}, nil)

				return mock
			},
			mockSecretClient: func() *fake.MockClientInterface[*v1.Secret, *v1.SecretList] {
				return fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctlr)
			},
		},
		"valid bcrypt password is migrated": {
			user: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: fakeUserID,
				},
			},
			password: fakePassword,
			mockSaltGenerator: func() ([]byte, error) {
				return []byte(fakePasswordSalt), nil
			},
			mockHashKey: func(password string, salt []byte, iter, keyLength int) ([]byte, error) {
				if password == fakePassword {
					return []byte(fakePasswordHash), nil
				}
				return nil, errors.New("unexpected password")
			},
			mockSecretCache: func() *fake.MockCacheInterface[*v1.Secret] {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctlr)
				mock.EXPECT().Get(LocalUserPasswordsNamespace, fakeUserID).Return(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fakeUserID,
						Namespace: LocalUserPasswordsNamespace,
						Annotations: map[string]string{
							passwordHashAnnotation: bcryptHash,
						},
					},
					Data: map[string][]byte{
						"password": bcryptHashPassword,
					},
				}, nil)

				return mock
			},
			mockSecretClient: func() *fake.MockClientInterface[*v1.Secret, *v1.SecretList] {
				mock := fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctlr)
				patch, _ := json.Marshal([]struct {
					Op    string `json:"op"`
					Path  string `json:"path"`
					Value any    `json:"value"`
				}{
					{
						Op:   "replace",
						Path: "/data",
						Value: map[string][]byte{
							"password": []byte(fakePasswordHash),
							"salt":     []byte(fakePasswordSalt),
						},
					},
					{
						Op:    "replace",
						Path:  "/metadata/annotations/cattle.io~1password-hash",
						Value: pbkdf2sha3512Hash,
					},
				})
				mock.EXPECT().Patch(LocalUserPasswordsNamespace, fakeUserID, types.JSONPatchType, patch).Return(nil, nil)

				return mock
			},
		},
		"invalid password": {
			user: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: fakeUserID,
				},
			},
			password: "another-password",
			mockHashKey: func(password string, salt []byte, iter, keyLength int) ([]byte, error) {
				return []byte("another-hash"), nil
			},
			mockSecretCache: func() *fake.MockCacheInterface[*v1.Secret] {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctlr)
				mock.EXPECT().Get(LocalUserPasswordsNamespace, fakeUserID).Return(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fakeUserID,
						Namespace: LocalUserPasswordsNamespace,
						Annotations: map[string]string{
							passwordHashAnnotation: pbkdf2sha3512Hash,
						},
					},
					Data: map[string][]byte{
						"salt":     []byte(fakePasswordSalt),
						"password": []byte(fakePasswordHash),
					},
				}, nil)

				return mock
			},
			mockSecretClient: func() *fake.MockClientInterface[*v1.Secret, *v1.SecretList] {
				return fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctlr)
			},
			expectErrorMessage: "invalid password",
		},
		"invalid bcrypt password": {
			user: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: fakeUserID,
				},
			},
			password: "another-password",
			mockSecretCache: func() *fake.MockCacheInterface[*v1.Secret] {
				mock := fake.NewMockCacheInterface[*v1.Secret](ctlr)
				mock.EXPECT().Get(LocalUserPasswordsNamespace, fakeUserID).Return(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fakeUserID,
						Namespace: LocalUserPasswordsNamespace,
						Annotations: map[string]string{
							passwordHashAnnotation: bcryptHash,
						},
					},
					Data: map[string][]byte{
						"password": bcryptHashPassword,
					},
				}, nil)

				return mock
			},
			mockSecretClient: func() *fake.MockClientInterface[*v1.Secret, *v1.SecretList] {
				return fake.NewMockClientInterface[*v1.Secret, *v1.SecretList](ctlr)
			},
			expectErrorMessage: bcrypt.ErrMismatchedHashAndPassword.Error(),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := Pbkdf2{
				secretLister:  test.mockSecretCache(),
				secretClient:  test.mockSecretClient(),
				hashKey:       test.mockHashKey,
				saltGenerator: test.mockSaltGenerator,
			}
			err := p.VerifyPassword(test.user, test.password)
			if test.expectErrorMessage == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.expectErrorMessage)
			}
		})
	}
}
