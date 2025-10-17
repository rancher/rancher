package useractivity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	mgmt "github.com/rancher/rancher/pkg/apis/management.cattle.io"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	authTokens "github.com/rancher/rancher/pkg/auth/tokens"
	exttokens "github.com/rancher/rancher/pkg/ext/stores/tokens"
	fake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	k8suser "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/utils/ptr"
)

const (
	defaultTokenTTL        = int64(57600000) // 16 hours
	defaultIdleTTL         = 960             // 16 hours
	tokenID                = "token-12345"
	uid                    = types.UID("07306f53-a3df-4608-ae02-d6595a24c17d")
	resourceVersion        = "12345"
	patchedResourceVersion = "12346"
)

var (
	now               = time.Now().UTC().Truncate(time.Second)
	creationTimestamp = metav1.NewTime(now.Add(-1 * time.Hour))
	user              = &apiv3.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "admin",
		},
	}
	ctxAdmin = request.WithUser(context.Background(), &k8suser.DefaultInfo{
		Name:   user.Name,
		Groups: []string{GroupCattleAuthenticated},
		Extra: map[string][]string{
			common.ExtraRequestTokenID: {tokenID},
		},
	})
	sessionLabel = map[string]string{
		authTokens.TokenKindLabel: "session",
	}
	userActivity = &ext.UserActivity{
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: creationTimestamp,
			UID:               uid,
			Name:              tokenID,
			ResourceVersion:   resourceVersion,
		},
	}
	commonAuthorizer = authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
		switch a.GetUser().GetName() {
		case user.Name:
			return authorizer.DecisionAllow, "", nil
		default:
			return authorizer.DecisionDeny, "", nil
		}
	})
	authToken = &apiv3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: tokenID,
		},
		AuthProvider:  "oidc",
		UserPrincipal: apiv3.Principal{},
	}
)

type fakeUpdatedObjectInfo struct {
	obj runtime.Object
	err error
}

func (i *fakeUpdatedObjectInfo) Preconditions() *metav1.Preconditions {
	return nil
}

func (i *fakeUpdatedObjectInfo) UpdatedObject(ctx context.Context, oldObj runtime.Object) (newObj runtime.Object, err error) {
	return i.obj, i.err
}

func TestStoreUpdate(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	var (
		tokens      *fake.MockNonNamespacedClientInterface[*apiv3.Token, *apiv3.TokenList]
		tokenCache  *fake.MockNonNamespacedCacheInterface[*apiv3.Token]
		userCache   *fake.MockNonNamespacedCacheInterface[*apiv3.User]
		secrets     *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]
		secretCache *fake.MockCacheInterface[*corev1.Secret]
		users       *fake.MockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList]
	)

	sessionToken := &apiv3.Token{
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: creationTimestamp,
			Name:              tokenID,
			Labels:            sessionLabel,
			UID:               uid,
			ResourceVersion:   resourceVersion,
		},
		AuthProvider:  "oidc",
		TTLMillis:     defaultTokenTTL,
		UserPrincipal: apiv3.Principal{},
	}
	patchedToken := sessionToken.DeepCopy()
	patchedToken.ResourceVersion = patchedResourceVersion
	patchedToken.ActivityLastSeenAt = &metav1.Time{Time: now}

	expectedExiresAt := metav1.NewTime(now.Add(defaultIdleTTL * time.Minute))
	wantUserActivity := &ext.UserActivity{
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: creationTimestamp,
			UID:               uid,
			Name:              tokenID,
			ResourceVersion:   patchedResourceVersion,
		},
		Spec: ext.UserActivitySpec{
			SeenAt: &metav1.Time{Time: now},
		},
		Status: ext.UserActivityStatus{
			ExpiresAt: expectedExiresAt.Format(time.RFC3339),
		},
	}

	tests := []struct {
		desc       string
		ctx        context.Context
		objInfo    func() rest.UpdatedObjectInfo
		options    *metav1.UpdateOptions
		setupMocks func()
		want       func() runtime.Object
		wantErr    string
	}{
		{
			desc:    "lastSeen is not set and no seenAt provided",
			ctx:     ctxAdmin,
			objInfo: func() rest.UpdatedObjectInfo { return &fakeUpdatedObjectInfo{obj: userActivity} },
			setupMocks: func() {
				gomock.InOrder(
					userCache.EXPECT().Get(user.Name).Return(user, nil),
					tokenCache.EXPECT().Get(tokenID).Return(authToken, nil),
					tokenCache.EXPECT().Get(tokenID).Return(sessionToken, nil),
					tokens.EXPECT().Patch(tokenID, types.JSONPatchType, gomock.Any()).Return(patchedToken, nil),
				)
			},
			want: func() runtime.Object { return wantUserActivity },
		},
		{
			desc:    "no seenAt provided, ext token",
			ctx:     ctxAdmin,
			objInfo: func() rest.UpdatedObjectInfo { return &fakeUpdatedObjectInfo{obj: userActivity} },
			setupMocks: func() {
				principalBytes, _ := json.Marshal(ext.TokenPrincipal{
					Name:        "world",
					Provider:    "oidc",
					DisplayName: "",
					LoginName:   "hello",
				})
				secret := corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						CreationTimestamp: creationTimestamp,
						Name:              tokenID,
						Labels: map[string]string{
							exttokens.UserIDLabel:     user.Name,
							exttokens.SecretKindLabel: exttokens.SecretKindLabelValue,
						},
						ResourceVersion: resourceVersion,
					},
					Data: map[string][]byte{
						exttokens.FieldDescription:    []byte(""),
						exttokens.FieldEnabled:        []byte("true"),
						exttokens.FieldHash:           []byte("kla9jkdmj"),
						exttokens.FieldKind:           []byte(exttokens.IsLogin),
						exttokens.FieldLastUpdateTime: []byte(creationTimestamp.Format(time.RFC3339)),
						exttokens.FieldTTL:            []byte(strconv.FormatInt(defaultTokenTTL, 10)),
						exttokens.FieldUID:            []byte(uid),
						exttokens.FieldUserID:         []byte("lkajdlksjlkds"),
						exttokens.FieldPrincipal:      principalBytes,
					},
				}
				patchedSecret := secret.DeepCopy()
				patchedSecret.ResourceVersion = patchedResourceVersion
				patchedSecret.Data[exttokens.FieldLastActivitySeen] = []byte(now.Format(time.RFC3339))

				gomock.InOrder(
					userCache.EXPECT().Get(user.Name).Return(user, nil),
					tokenCache.EXPECT().Get(tokenID).Return(nil, fmt.Errorf("some error")),
					secretCache.EXPECT().Get(exttokens.TokenNamespace, tokenID).Return(&secret, nil),
					tokenCache.EXPECT().Get(tokenID).Return(nil, fmt.Errorf("some error")),
					secretCache.EXPECT().Get(exttokens.TokenNamespace, tokenID).Return(&secret, nil),
					secrets.EXPECT().Patch(exttokens.TokenNamespace, tokenID, types.JSONPatchType, gomock.Any()).Return(patchedSecret, nil),
				)
			},
			want: func() runtime.Object { return wantUserActivity },
		},
		{
			desc: "seenAt is greater than lastSeenAt",
			ctx:  ctxAdmin,
			objInfo: func() rest.UpdatedObjectInfo {
				seenAt := metav1.NewTime(now.Add(-5 * time.Minute))
				userActivity := userActivity.DeepCopy()
				userActivity.Spec.SeenAt = &seenAt
				return &fakeUpdatedObjectInfo{obj: userActivity}
			},
			setupMocks: func() {
				sessionToken := sessionToken.DeepCopy()
				sessionToken.ActivityLastSeenAt = &metav1.Time{Time: now.Add(-10 * time.Minute)}

				patchedToken := patchedToken.DeepCopy()
				patchedToken.ActivityLastSeenAt = &metav1.Time{Time: now.Add(-5 * time.Minute)}

				gomock.InOrder(
					userCache.EXPECT().Get(user.Name).Return(user, nil),
					tokenCache.EXPECT().Get(tokenID).Return(authToken, nil),
					tokenCache.EXPECT().Get(tokenID).Return(sessionToken, nil),
					tokens.EXPECT().Patch(tokenID, types.JSONPatchType, gomock.Any()).Return(patchedToken, nil),
				)
			},
			want: func() runtime.Object {
				seenAt := metav1.NewTime(now.Add(-5 * time.Minute))
				wantUserActivity := wantUserActivity.DeepCopy()
				wantUserActivity.Spec.SeenAt = &seenAt
				wantUserActivity.Status.ExpiresAt = now.Add((defaultIdleTTL - 5) * time.Minute).Format(time.RFC3339)
				return wantUserActivity
			},
		},
		{
			desc: "seenAt is provided, lastSeenAt is not set",
			ctx:  ctxAdmin,
			objInfo: func() rest.UpdatedObjectInfo {
				seenAt := metav1.NewTime(now.Add(-5 * time.Minute))
				userActivity := userActivity.DeepCopy()
				userActivity.Spec.SeenAt = &seenAt
				return &fakeUpdatedObjectInfo{obj: userActivity}
			},
			setupMocks: func() {
				patchedToken := patchedToken.DeepCopy()
				patchedToken.ActivityLastSeenAt = &metav1.Time{Time: now.Add(-5 * time.Minute)}

				gomock.InOrder(
					userCache.EXPECT().Get(user.Name).Return(user, nil),
					tokenCache.EXPECT().Get(tokenID).Return(authToken, nil),
					tokenCache.EXPECT().Get(tokenID).Return(sessionToken, nil),
					tokens.EXPECT().Patch(tokenID, types.JSONPatchType, gomock.Any()).Return(patchedToken, nil),
				)
			},
			want: func() runtime.Object {
				seenAt := metav1.NewTime(now.Add(-5 * time.Minute))
				wantUserActivity := wantUserActivity.DeepCopy()
				wantUserActivity.Spec.SeenAt = &seenAt
				wantUserActivity.Status.ExpiresAt = now.Add((defaultIdleTTL - 5) * time.Minute).Format(time.RFC3339)
				return wantUserActivity
			},
		},
		{
			desc: "seenAt is less than lastSeenAt",
			ctx:  ctxAdmin,
			objInfo: func() rest.UpdatedObjectInfo {
				userActivity := userActivity.DeepCopy()
				seenAt := metav1.NewTime(now.Add(-10 * time.Minute))
				userActivity.Spec.SeenAt = &seenAt
				return &fakeUpdatedObjectInfo{obj: userActivity}
			},
			setupMocks: func() {
				sessionToken := sessionToken.DeepCopy()
				sessionToken.ActivityLastSeenAt = &metav1.Time{Time: now.Add(-5 * time.Minute)}

				gomock.InOrder(
					userCache.EXPECT().Get(user.Name).Return(user, nil),
					tokenCache.EXPECT().Get(tokenID).Return(authToken, nil),
					tokenCache.EXPECT().Get(tokenID).Return(sessionToken, nil),
				)
			},
			want: func() runtime.Object {
				wantUserActivity.ResourceVersion = resourceVersion // Resource version should not change.
				seenAt := metav1.NewTime(now.Add(-5 * time.Minute))
				wantUserActivity := wantUserActivity.DeepCopy()
				wantUserActivity.Spec.SeenAt = &seenAt
				wantUserActivity.Status.ExpiresAt = now.Add((defaultIdleTTL - 5) * time.Minute).Format(time.RFC3339)
				return wantUserActivity
			},
		},
		{
			desc: "seenAt is in the future",
			ctx:  ctxAdmin,
			objInfo: func() rest.UpdatedObjectInfo {
				seenAt := metav1.NewTime(now.Add(1 * time.Second))
				userActivity := userActivity.DeepCopy()
				userActivity.Spec.SeenAt = &seenAt
				return &fakeUpdatedObjectInfo{obj: userActivity}
			},
			setupMocks: func() {
				patchedToken := patchedToken.DeepCopy()
				patchedToken.ResourceVersion = resourceVersion // Resource version should not change.

				gomock.InOrder(
					userCache.EXPECT().Get(user.Name).Return(user, nil),
					tokenCache.EXPECT().Get(tokenID).Return(authToken, nil),
					tokenCache.EXPECT().Get(tokenID).Return(sessionToken, nil),
					tokens.EXPECT().Patch(tokenID, types.JSONPatchType, gomock.Any()).Return(patchedToken, nil),
				)
			},
			want: func() runtime.Object { return wantUserActivity },
		},
		{
			desc: "user doesn't exist",
			ctx: request.WithUser(context.Background(), &k8suser.DefaultInfo{
				Name:   "user-xyz",
				Groups: []string{GroupCattleAuthenticated},
				Extra: map[string][]string{
					common.ExtraRequestTokenID: {tokenID},
				},
			}),
			objInfo: func() rest.UpdatedObjectInfo { return &fakeUpdatedObjectInfo{obj: userActivity} },
			setupMocks: func() {
				gomock.InOrder(
					userCache.EXPECT().Get("user-xyz").Return(
						nil, apierrors.NewNotFound(schema.GroupResource{Group: mgmt.GroupName, Resource: "users"}, "user-xyz"),
					),
				)
			},
			wantErr: "user user-xyz is not a Rancher user",
		},
		{
			desc:    "auth providers don't match",
			ctx:     ctxAdmin,
			objInfo: func() rest.UpdatedObjectInfo { return &fakeUpdatedObjectInfo{obj: userActivity} },
			setupMocks: func() {
				sessionToken := sessionToken.DeepCopy()
				sessionToken.AuthProvider = "local"

				gomock.InOrder(
					userCache.EXPECT().Get(user.Name).Return(user, nil),
					tokenCache.EXPECT().Get(tokenID).Return(authToken, nil),
					tokenCache.EXPECT().Get(tokenID).Return(sessionToken, nil),
				)
			},
			wantErr: "auth providers don't match",
		},
		{
			desc:    "token is disabled",
			ctx:     ctxAdmin,
			objInfo: func() rest.UpdatedObjectInfo { return &fakeUpdatedObjectInfo{obj: userActivity} },
			setupMocks: func() {
				sessionToken := sessionToken.DeepCopy()
				sessionToken.Enabled = ptr.To(false)

				gomock.InOrder(
					userCache.EXPECT().Get(user.Name).Return(user, nil),
					tokenCache.EXPECT().Get(tokenID).Return(authToken, nil),
					tokenCache.EXPECT().Get(tokenID).Return(sessionToken, nil),
				)
			},
			wantErr: "token is disabled",
		},
		{
			desc:    "not a session token",
			ctx:     ctxAdmin,
			objInfo: func() rest.UpdatedObjectInfo { return &fakeUpdatedObjectInfo{obj: userActivity} },
			setupMocks: func() {
				sessionToken := sessionToken.DeepCopy()
				sessionToken.IsDerived = true // Not a session token.

				gomock.InOrder(
					userCache.EXPECT().Get(user.Name).Return(user, nil),
					tokenCache.EXPECT().Get(tokenID).Return(authToken, nil),
					tokenCache.EXPECT().Get(tokenID).Return(sessionToken, nil),
				)
			},
			wantErr: "not a session token",
		},
		{
			desc:    "session token has expired",
			ctx:     ctxAdmin,
			objInfo: func() rest.UpdatedObjectInfo { return &fakeUpdatedObjectInfo{obj: userActivity} },
			setupMocks: func() {
				sessionToken := sessionToken.DeepCopy()
				sessionToken.CreationTimestamp = metav1.NewTime(now.Add(-time.Duration(defaultTokenTTL+600000) * time.Millisecond))

				gomock.InOrder(
					userCache.EXPECT().Get(user.Name).Return(user, nil),
					tokenCache.EXPECT().Get(tokenID).Return(authToken, nil),
					tokenCache.EXPECT().Get(tokenID).Return(sessionToken, nil),
				)
			},
			wantErr: "token is expired",
		},
		{
			desc:    "session idle timeout has expired",
			ctx:     ctxAdmin,
			objInfo: func() rest.UpdatedObjectInfo { return &fakeUpdatedObjectInfo{obj: userActivity} },
			setupMocks: func() {
				sessionToken := sessionToken.DeepCopy()
				sessionToken.ActivityLastSeenAt = &metav1.Time{Time: now.Add(-(defaultIdleTTL + 1) * time.Minute)}
				sessionToken.TTLMillis = defaultTokenTTL + 1200000 // +20 min to avoid token expire before idle timeout.

				gomock.InOrder(
					userCache.EXPECT().Get(user.Name).Return(user, nil),
					tokenCache.EXPECT().Get(tokenID).Return(authToken, nil),
					tokenCache.EXPECT().Get(tokenID).Return(sessionToken, nil),
				)
			},
			wantErr: "session idle timeout expired",
		},
		{
			desc:    "dry run",
			ctx:     ctxAdmin,
			objInfo: func() rest.UpdatedObjectInfo { return &fakeUpdatedObjectInfo{obj: userActivity} },
			options: &metav1.UpdateOptions{
				DryRun: []string{metav1.DryRunAll},
			},
			setupMocks: func() {
				gomock.InOrder(
					userCache.EXPECT().Get(user.Name).Return(user, nil),
					tokenCache.EXPECT().Get(tokenID).Return(authToken, nil),
					tokenCache.EXPECT().Get(tokenID).Return(sessionToken, nil),
				)
			},
			want: func() runtime.Object {
				wantUserActivity := wantUserActivity.DeepCopy()
				wantUserActivity.ResourceVersion = resourceVersion // Resource version should not change.
				return wantUserActivity
			},
		},
	}

	origTimeNow := timeNow
	timeNow = func() time.Time { return now }
	defer func() { timeNow = origTimeNow }() // Restore original function after test

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			tokens = fake.NewMockNonNamespacedClientInterface[*apiv3.Token, *apiv3.TokenList](ctrl)
			tokenCache = fake.NewMockNonNamespacedCacheInterface[*apiv3.Token](ctrl)
			userCache = fake.NewMockNonNamespacedCacheInterface[*apiv3.User](ctrl)
			secrets = fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
			secretCache = fake.NewMockCacheInterface[*corev1.Secret](ctrl)
			users = fake.NewMockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList](ctrl)
			users.EXPECT().Cache().Return(nil)
			secrets.EXPECT().Cache().Return(secretCache)

			tt.setupMocks()

			store := &Store{
				authorizer:                       commonAuthorizer,
				tokens:                           tokens,
				userCache:                        userCache,
				extTokenStore:                    exttokens.NewSystem(nil, nil, secrets, users, tokenCache, nil, nil, nil, nil),
				getAuthUserSessionIdleTTLMinutes: func() int { return defaultIdleTTL },
			}

			var validateFuncCalled bool
			validateFunc := func(ctx context.Context, obj, old runtime.Object) error {
				validateFuncCalled = true
				return nil
			}

			got, created, err := store.Update(tt.ctx, tokenID, tt.objInfo(), nil, validateFunc, false, tt.options)
			require.False(t, created) // Should always be false.
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want(), got)
			assert.True(t, validateFuncCalled)
		})
	}
}

func TestStoreGet(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	var (
		tokens      *fake.MockNonNamespacedControllerInterface[*apiv3.Token, *apiv3.TokenList]
		tokenCache  *fake.MockNonNamespacedCacheInterface[*apiv3.Token]
		userCache   *fake.MockNonNamespacedCacheInterface[*apiv3.User]
		secrets     *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]
		secretCache *fake.MockCacheInterface[*corev1.Secret]
		users       *fake.MockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList]
	)

	wantUserActivity := &ext.UserActivity{
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: creationTimestamp,
			UID:               uid,
			Name:              tokenID,
		},
		Spec: ext.UserActivitySpec{
			SeenAt: &metav1.Time{Time: now.Add(-5 * time.Minute)},
		},
		Status: ext.UserActivityStatus{
			ExpiresAt: now.Add((defaultIdleTTL - 5) * time.Minute).Format(time.RFC3339),
		},
	}

	tests := []struct {
		desc       string
		ctx        context.Context
		name       string
		setupMocks func()
		want       runtime.Object
		wantErr    bool
	}{
		{
			desc: "no lastSeenAt is set",
			ctx:  ctxAdmin,
			name: tokenID,
			setupMocks: func() {
				tokenCache.EXPECT().Get(gomock.Any()).Return(&apiv3.Token{
					ObjectMeta: metav1.ObjectMeta{
						CreationTimestamp: creationTimestamp,
						UID:               uid,
						Name:              tokenID,
						Labels: map[string]string{
							authTokens.TokenKindLabel: "session",
						},
					},
					UserID:    user.Name,
					TTLMillis: defaultTokenTTL,
				}, nil).AnyTimes()
			},
			want: &ext.UserActivity{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: creationTimestamp,
					UID:               uid,
					Name:              tokenID,
				},
			},
			wantErr: false,
		},
		{
			desc: "lastSeenAt is set",
			ctx:  ctxAdmin,
			name: tokenID,
			setupMocks: func() {
				tokenCache.EXPECT().Get(gomock.Any()).Return(&apiv3.Token{
					ObjectMeta: metav1.ObjectMeta{
						CreationTimestamp: creationTimestamp,
						UID:               uid,
						Name:              tokenID,
						Labels: map[string]string{
							authTokens.TokenKindLabel: "session",
						},
					},
					ActivityLastSeenAt: &metav1.Time{
						Time: now.Add(-5 * time.Minute),
					},
					UserID:    user.Name,
					TTLMillis: defaultTokenTTL,
				}, nil).AnyTimes()
			},
			want:    wantUserActivity,
			wantErr: false,
		},
		{
			desc: "lastSeenAt is set, ext token",
			ctx:  ctxAdmin,
			name: tokenID,
			setupMocks: func() {
				principalBytes, _ := json.Marshal(ext.TokenPrincipal{
					Name:        "world",
					Provider:    "oidc",
					DisplayName: "",
					LoginName:   "hello",
				})
				secret := corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						CreationTimestamp: creationTimestamp,
						UID:               uid,
						Name:              tokenID,
						Labels: map[string]string{
							exttokens.UserIDLabel:     user.Name,
							exttokens.SecretKindLabel: exttokens.SecretKindLabelValue,
						},
					},
					Data: map[string][]byte{
						exttokens.FieldDescription:      []byte(""),
						exttokens.FieldEnabled:          []byte("true"),
						exttokens.FieldHash:             []byte("kla9jkdmj"),
						exttokens.FieldKind:             []byte(exttokens.IsLogin),
						exttokens.FieldLastActivitySeen: []byte(now.Add(-5 * time.Minute).Format(time.RFC3339)),
						exttokens.FieldLastUpdateTime:   []byte(creationTimestamp.Format(time.RFC3339)),
						exttokens.FieldPrincipal:        principalBytes,
						exttokens.FieldTTL:              []byte(strconv.FormatInt(defaultTokenTTL, 10)),
						exttokens.FieldUID:              []byte(uid),
						exttokens.FieldUserID:           []byte("lkajdlksjlkds"),
					},
				}

				tokenCache.EXPECT().Get(gomock.Any()).Return(nil, fmt.Errorf("some error")).AnyTimes()
				secretCache.EXPECT().Get(exttokens.TokenNamespace, gomock.Any()).Return(&secret, nil).AnyTimes()
			},
			want:    wantUserActivity,
			wantErr: false,
		},
		{
			desc:       "invalid useractivity name",
			ctx:        context.Background(),
			name:       "ua_admin_token_12345",
			setupMocks: func() {},
			want:       nil,
			wantErr:    true,
		},
		{
			desc: "invalid token retrieved",
			ctx:  context.Background(),
			name: "ua_admin_token-12345",
			setupMocks: func() {
				tokenCache.EXPECT().Get(gomock.Any()).Return(nil, errors.New("invalid token name")).AnyTimes()
			},
			want:    nil,
			wantErr: true,
		},
		{
			desc: "invalid user name retrieved",
			ctx:  context.Background(),
			name: "ua_user1_token-12345",
			setupMocks: func() {
				tokenCache.EXPECT().Get(gomock.Any()).Return(&apiv3.Token{
					UserID:             tokenID,
					ActivityLastSeenAt: &metav1.Time{Time: now.Add(-5 * time.Minute)},
				}, nil).AnyTimes()
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			tokens = fake.NewMockNonNamespacedControllerInterface[*apiv3.Token, *apiv3.TokenList](ctrl)
			tokenCache = fake.NewMockNonNamespacedCacheInterface[*apiv3.Token](ctrl)
			secrets = fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
			secretCache = fake.NewMockCacheInterface[*corev1.Secret](ctrl)
			userCache = fake.NewMockNonNamespacedCacheInterface[*apiv3.User](ctrl)
			userCache.EXPECT().Get(gomock.Any()).Return(&apiv3.User{}, nil).AnyTimes()
			users = fake.NewMockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList](ctrl)
			users.EXPECT().Cache().Return(userCache)
			secrets.EXPECT().Cache().Return(secretCache)

			tt.setupMocks()

			store := &Store{
				authorizer:                       commonAuthorizer,
				tokens:                           tokens,
				userCache:                        userCache,
				extTokenStore:                    exttokens.NewSystem(nil, nil, secrets, users, tokenCache, nil, nil, nil, nil),
				getAuthUserSessionIdleTTLMinutes: func() int { return defaultIdleTTL },
			}

			got, err := store.Get(tt.ctx, tt.name, &metav1.GetOptions{})
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			userActivity, ok := got.(*ext.UserActivity)
			require.True(t, ok)
			assert.Equal(t, tt.want, userActivity)
		})
	}
}
