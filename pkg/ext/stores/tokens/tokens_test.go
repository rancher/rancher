package tokens

import (
	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"

	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Test_SystemTokenStore_Get(t *testing.T) {

	someerror := fmt.Errorf("bogus")
	userIdMissingError := fmt.Errorf("userId missing")
	hashMissingError := fmt.Errorf("token hash missing")
	authProviderMissingError := fmt.Errorf("auth provider missing")
	lastUpdateMissingError := fmt.Errorf("last update time missing")
	_, parseBoolError := strconv.ParseBool("")
	_, parseIntError := strconv.ParseInt("", 10, 64)
	var up v3.Principal
	jsonSyntaxError := json.Unmarshal([]byte(""), &up)

	tests := []struct {
		name    string                                          // test name
		store   func(ctrl *gomock.Controller) *SystemTokenStore // create store to test, with mock clients
		tokname string                                          // name of token to retrieve
		opts    *metav1.GetOptions                              // retrieval options
		err     error                                           // expected op result, error
		tok     *ext.Token                                      // expected op result, token
	}{
		{
			name: "backing secret not found",
			store: func(ctrl *gomock.Controller) *SystemTokenStore {
				// mock clients ...
				secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
				uattrs := fake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
				users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)

				// configure mocks for expected calls and responses
				secrets.EXPECT().
					Get("cattle-tokens", "bogus", gomock.Any()).
					Return(nil, apierrors.NewNotFound(schema.GroupResource{}, "")).
					AnyTimes()

				// assemble store talking to mocks
				return NewSystemTokenStore(secrets, uattrs, users)
			},
			tokname: "bogus",
			opts:    nil,
			err:     apierrors.NewNotFound(schema.GroupResource{}, ""),
			tok:     nil,
		},
		{
			name: "some other error",
			store: func(ctrl *gomock.Controller) *SystemTokenStore {
				// mock clients ...
				secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
				uattrs := fake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
				users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)

				// configure mocks for expected calls and responses
				secrets.EXPECT().
					Get("cattle-tokens", "bogus", gomock.Any()).
					Return(nil, someerror).
					AnyTimes()

				// assemble store talking to mocks
				return NewSystemTokenStore(secrets, uattrs, users)
			},
			tokname: "bogus",
			opts:    nil,
			err:     apierrors.NewInternalError(fmt.Errorf("failed to retrieve token %s: %w", "bogus", someerror)),
			tok:     nil,
		},
		{
			name: "empty secret",
			store: func(ctrl *gomock.Controller) *SystemTokenStore {
				// mock clients ...
				secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
				uattrs := fake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
				users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)

				// configure mocks for expected calls and responses
				secrets.EXPECT().
					Get("cattle-tokens", "bogus", gomock.Any()).
					Return(&corev1.Secret{}, nil).
					AnyTimes()

				// assemble store talking to mocks
				return NewSystemTokenStore(secrets, uattrs, users)
			},
			tokname: "bogus",
			opts:    nil,
			err:     apierrors.NewInternalError(fmt.Errorf("failed to extract token %s: %w", "bogus", parseBoolError)),
			tok:     nil,
		},
		{
			name: "part-filled secret (enabled)",
			store: func(ctrl *gomock.Controller) *SystemTokenStore {
				// mock clients ...
				secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
				uattrs := fake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
				users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)

				// configure mocks for expected calls and responses
				secrets.EXPECT().
					Get("cattle-tokens", "bogus", gomock.Any()).
					Return(&corev1.Secret{
						Data: map[string][]byte{
							"enabled": []byte("false"),
						},
					}, nil).
					AnyTimes()

				// assemble store talking to mocks
				return NewSystemTokenStore(secrets, uattrs, users)
			},
			tokname: "bogus",
			opts:    nil,
			err:     apierrors.NewInternalError(fmt.Errorf("failed to extract token %s: %w", "bogus", parseBoolError)),
			tok:     nil,
		},
		{
			name: "part-filled secret (enabled, is-login)",
			store: func(ctrl *gomock.Controller) *SystemTokenStore {
				// mock clients ...
				secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
				uattrs := fake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
				users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)

				// configure mocks for expected calls and responses
				secrets.EXPECT().
					Get("cattle-tokens", "bogus", gomock.Any()).
					Return(&corev1.Secret{
						Data: map[string][]byte{
							"enabled":  []byte("false"),
							"is-login": []byte("true"),
						},
					}, nil).
					AnyTimes()

				// assemble store talking to mocks
				return NewSystemTokenStore(secrets, uattrs, users)
			},
			tokname: "bogus",
			opts:    nil,
			err:     apierrors.NewInternalError(fmt.Errorf("failed to extract token %s: %w", "bogus", parseIntError)),
			tok:     nil,
		},
		{
			name: "part-filled secret (enabled, is-login, ttl)",
			store: func(ctrl *gomock.Controller) *SystemTokenStore {
				// mock clients ...
				secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
				uattrs := fake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
				users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)

				// configure mocks for expected calls and responses
				secrets.EXPECT().
					Get("cattle-tokens", "bogus", gomock.Any()).
					Return(&corev1.Secret{
						Data: map[string][]byte{
							"enabled":  []byte("false"),
							"is-login": []byte("true"),
							"ttl":      []byte("4000"),
						},
					}, nil).
					AnyTimes()

				// assemble store talking to mocks
				return NewSystemTokenStore(secrets, uattrs, users)
			},
			tokname: "bogus",
			opts:    nil,
			err:     apierrors.NewInternalError(fmt.Errorf("failed to extract token %s: %w", "bogus", userIdMissingError)),
			tok:     nil,
		},
		{
			name: "part-filled secret (enabled, is-login, ttl, user id)",
			store: func(ctrl *gomock.Controller) *SystemTokenStore {
				// mock clients ...
				secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
				uattrs := fake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
				users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)

				// configure mocks for expected calls and responses
				secrets.EXPECT().
					Get("cattle-tokens", "bogus", gomock.Any()).
					Return(&corev1.Secret{
						Data: map[string][]byte{
							"enabled":  []byte("false"),
							"is-login": []byte("true"),
							"ttl":      []byte("4000"),
							"userID":   []byte("lkajdlksjlkds"),
						},
					}, nil).
					AnyTimes()

				// assemble store talking to mocks
				return NewSystemTokenStore(secrets, uattrs, users)
			},
			tokname: "bogus",
			opts:    nil,
			err:     apierrors.NewInternalError(fmt.Errorf("failed to extract token %s: %w", "bogus", hashMissingError)),
			tok:     nil,
		},
		{
			name: "part-filled secret (enabled, is-login, ttl, user id, hash)",
			store: func(ctrl *gomock.Controller) *SystemTokenStore {
				// mock clients ...
				secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
				uattrs := fake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
				users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)

				// configure mocks for expected calls and responses
				secrets.EXPECT().
					Get("cattle-tokens", "bogus", gomock.Any()).
					Return(&corev1.Secret{
						Data: map[string][]byte{
							"enabled":  []byte("false"),
							"is-login": []byte("true"),
							"ttl":      []byte("4000"),
							"userID":   []byte("lkajdlksjlkds"),
							"hash":     []byte("kla9jkdmj"),
						},
					}, nil).
					AnyTimes()

				// assemble store talking to mocks
				return NewSystemTokenStore(secrets, uattrs, users)
			},
			tokname: "bogus",
			opts:    nil,
			err:     apierrors.NewInternalError(fmt.Errorf("failed to extract token %s: %w", "bogus", authProviderMissingError)),
			tok:     nil,
		},
		{
			name: "part-filled secret (enabled, is-login, ttl, user id, hash, auth provider)",
			store: func(ctrl *gomock.Controller) *SystemTokenStore {
				// mock clients ...
				secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
				uattrs := fake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
				users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)

				// configure mocks for expected calls and responses
				secrets.EXPECT().
					Get("cattle-tokens", "bogus", gomock.Any()).
					Return(&corev1.Secret{
						Data: map[string][]byte{
							"enabled":       []byte("false"),
							"is-login":      []byte("true"),
							"ttl":           []byte("4000"),
							"userID":        []byte("lkajdlksjlkds"),
							"hash":          []byte("kla9jkdmj"),
							"auth-provider": []byte("somebody"),
						},
					}, nil).
					AnyTimes()

				// assemble store talking to mocks
				return NewSystemTokenStore(secrets, uattrs, users)
			},
			tokname: "bogus",
			opts:    nil,
			err:     apierrors.NewInternalError(fmt.Errorf("failed to extract token %s: %w", "bogus", lastUpdateMissingError)),
			tok:     nil,
		},
		{
			name: "part-filled secret (enabled, is-login, ttl, user id, hash, auth provider, last update)",
			store: func(ctrl *gomock.Controller) *SystemTokenStore {
				// mock clients ...
				secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
				uattrs := fake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
				users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)

				// configure mocks for expected calls and responses
				secrets.EXPECT().
					Get("cattle-tokens", "bogus", gomock.Any()).
					Return(&corev1.Secret{
						Data: map[string][]byte{
							"enabled":          []byte("false"),
							"is-login":         []byte("true"),
							"ttl":              []byte("4000"),
							"userID":           []byte("lkajdlksjlkds"),
							"hash":             []byte("kla9jkdmj"),
							"auth-provider":    []byte("somebody"),
							"last-update-time": []byte("13:00:05"),
						},
					}, nil).
					AnyTimes()

				// assemble store talking to mocks
				return NewSystemTokenStore(secrets, uattrs, users)
			},
			tokname: "bogus",
			opts:    nil,
			err:     apierrors.NewInternalError(fmt.Errorf("failed to extract token %s: %w", "bogus", jsonSyntaxError)),
			tok:     nil,
		},
		{
			name: "filled secret",
			store: func(ctrl *gomock.Controller) *SystemTokenStore {
				// mock clients ...
				secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
				uattrs := fake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
				users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)

				// configure mocks for expected calls and responses
				secrets.EXPECT().
					Get("cattle-tokens", "bogus", gomock.Any()).
					Return(&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name: "bogus",
						},
						Data: map[string][]byte{
							"enabled":          []byte("false"),
							"is-login":         []byte("true"),
							"ttl":              []byte("4000"),
							"userID":           []byte("lkajdlksjlkds"),
							"hash":             []byte("kla9jkdmj"),
							"auth-provider":    []byte("somebody"),
							"last-update-time": []byte("13:00:05"),
							"user-principal":   []byte("{}"),
							// Should actually add tests for the structure of the user principal
							// and check the same in the store code.
						},
					}, nil).
					AnyTimes()

				// assemble store talking to mocks
				return NewSystemTokenStore(secrets, uattrs, users)
			},
			tokname: "bogus",
			opts:    nil,
			err:     nil,
			tok: &ext.Token{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Token",
					APIVersion: "ext.cattle.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "bogus",
				},
				Spec: ext.TokenSpec{
					UserID:      "lkajdlksjlkds",
					Description: "",
					ClusterName: "",
					TTL:         4000,
					Enabled:     false,
					IsLogin:     true,
				},
				Status: ext.TokenStatus{
					TokenValue:     "",
					TokenHash:      "kla9jkdmj",
					Expired:        true,
					ExpiresAt:      "0001-01-01T00:00:04Z",
					AuthProvider:   "somebody",
					LastUpdateTime: "13:00:05",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			tok, err := test.store(ctrl).Get(test.tokname, test.opts)
			if test.err != nil {
				assert.Equal(t, test.err, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tok, test.tok)
		})
	}
}

// NewSystemTokenStore	core constructor // no admin checks
// NewTokenStore		     		 // admin checks -> SAR mocking

// Basic mocking of secret client/cache, userattr, user client

// cache: update, get, delete
// note: creation from wrangler may properly share the cache among several token stores
// alt:  move creation of system token store up the call chain, make it part of wrangler context?

// Create - token store only, not system store - todo expose that for future system-internal token creation

// Update
// 	(TokenStore: fail to check admin
// 		fail to check user full permissions)

// 	fail on retrieval of backing secret
// 	fail on conversion of secret to token
// 		fail to parse enabled			/bool
// 		fail to parse is-login			/bool
// 		fail to parse tll			/int
// 		fail to unmarshal user principal	/json
// 		fail to set expired
// 			fail to to marshal time

// 	fail on change of user id
// 	fail on change of cluster name
// 	fail on change of `IsDerived`, i.e. flag `is this a login token`

// 	fail on change of TTL extension for non-admin	// can we test `update`, note lower-case

// 	fail on conversion of token to secret
// 		fail to marshal user principal	/json

// 	fail on update of backing secret
// 	fail on reading back token (2nd conversion of secret to token)

// Get
// 	fail on retrieval of backing secret
// 	fail on conversion of secret to token	(Details s.a.)

// 	(TokenStore: fail to check admin)

// List
// 	(TokenStore: fail to check user full permissions)

// 	fail on failure to list secrets
// 	note: secret to token conversion errors are ignored

// Delete
// 	fail on retrieval of backing secret
// 	fail on conversion of secret to token	(Details s.a.)
// 	fail on failure to delete secret
