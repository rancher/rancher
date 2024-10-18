package tokens

import (
	"testing"

	"github.com/golang/mock/gomock" // <-- fake | "go.uber.org/mock/gomock"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Test_SystemTokenStore_Get(t *testing.T) {
	tests := []struct {
		name    string                                          // test name
		store   func(ctrl *gomock.Controller) *SystemTokenStore // create store to test, with mock clients
		tokname string                                          // name of token to retrieve
		opts    *metav1.GetOptions                              // retrieval options
		err     error                                           // expected op result, error
		tok     *Token                                          // expected op result, token
	}{
		{
			name: "no backing secret",
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
