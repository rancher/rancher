package useractivity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	exttokens "github.com/rancher/rancher/pkg/ext/stores/tokens"
	fake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8suser "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

const defaultIdleTTL = 960

var (
	ctxAdmin = request.WithUser(context.Background(), &k8suser.DefaultInfo{
		Name:   "admin",
		Groups: []string{GroupCattleAuthenticated},
		Extra: map[string][]string{
			common.ExtraRequestTokenID: {"token-12345"},
		},
	})
	mockNow          = time.Date(2025, 2, 1, 0, 54, 0, 0, time.UTC)
	expectedExiresAt = metav1.NewTime(mockNow.Add(defaultIdleTTL * time.Minute))
	userActivity     = &ext.UserActivity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "token-12345",
		},
	}
	wantUserActivity = &ext.UserActivity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "token-12345",
		},
		Status: ext.UserActivityStatus{
			ExpiresAt: expectedExiresAt.Format(time.RFC3339),
		},
	}
)

func TestStoreCreate(t *testing.T) {
	ctrl := gomock.NewController(t)

	var (
		tokenController *fake.MockNonNamespacedControllerInterface[*apiv3.Token, *apiv3.TokenList]
		tokenCache      *fake.MockNonNamespacedCacheInterface[*apiv3.Token]
		userCache       *fake.MockNonNamespacedCacheInterface[*apiv3.User]
		secrets         *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]
		secretCache     *fake.MockCacheInterface[*corev1.Secret]
		users           *fake.MockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList]
	)

	type args struct {
		ctx          context.Context
		obj          *ext.UserActivity
		validateFunc rest.ValidateObjectFunc
		options      *metav1.CreateOptions
	}

	tests := []struct {
		name      string
		args      args
		mockSetup func()
		want      runtime.Object
		wantErr   bool
	}{
		{
			name: "no seenAt provided",
			args: args{
				ctx: ctxAdmin,
				obj: userActivity,
			},
			mockSetup: func() {
				gomock.InOrder(
					userCache.EXPECT().Get("admin").Return(&apiv3.User{
						ObjectMeta: metav1.ObjectMeta{
							Name: "admin",
						},
					}, nil),

					tokenCache.EXPECT().Get("token-12345").Return(&apiv3.Token{
						ObjectMeta: metav1.ObjectMeta{
							Name: "token-12345",
						},
						AuthProvider:  "oidc",
						UserPrincipal: apiv3.Principal{},
					}, nil),
					tokenCache.EXPECT().Get("token-12345").Return(&apiv3.Token{
						ObjectMeta: metav1.ObjectMeta{
							Name: "token-12345",
							Labels: map[string]string{
								tokens.TokenKindLabel: "session",
							},
						},
						AuthProvider:  "oidc",
						UserPrincipal: apiv3.Principal{},
					}, nil),
					tokenController.EXPECT().
						Patch("token-12345", types.JSONPatchType, gomock.Any()).
						Return(&apiv3.Token{}, nil),
				)
			},
			want: wantUserActivity,
		},
		{
			name: "no seenAt provided, ext token",
			args: args{
				ctx: ctxAdmin,
				obj: userActivity,
			},
			mockSetup: func() {
				ePrincipal := ext.TokenPrincipal{
					Name:        "world",
					Provider:    "oidc",
					DisplayName: "",
					LoginName:   "hello",
				}
				ePrincipalBytes, _ := json.Marshal(ePrincipal)
				eSecret := corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "token-12345",
						Labels: map[string]string{
							exttokens.UserIDLabel:     "admin",
							exttokens.SecretKindLabel: exttokens.SecretKindLabelValue,
						},
					},
					Data: map[string][]byte{
						exttokens.FieldDescription:    []byte(""),
						exttokens.FieldEnabled:        []byte("true"),
						exttokens.FieldHash:           []byte("kla9jkdmj"),
						exttokens.FieldKind:           []byte(exttokens.IsLogin),
						exttokens.FieldLastUpdateTime: []byte("13:00:05"),
						exttokens.FieldPrincipal:      ePrincipalBytes,
						exttokens.FieldTTL:            []byte("4000"),
						exttokens.FieldUID:            []byte("2905498-kafld-lkad"),
						exttokens.FieldUserID:         []byte("lkajdlksjlkds"),
					},
				}
				gomock.InOrder(
					userCache.EXPECT().Get("admin").Return(&apiv3.User{
						ObjectMeta: metav1.ObjectMeta{
							Name: "admin",
						},
					}, nil),
					tokenCache.EXPECT().Get("token-12345").
						Return(nil, fmt.Errorf("some error")),
					secretCache.EXPECT().
						Get("cattle-tokens", "token-12345").
						Return(&eSecret, nil),
					tokenCache.EXPECT().Get("token-12345").
						Return(nil, fmt.Errorf("some error")),
					secretCache.EXPECT().
						Get("cattle-tokens", "token-12345").
						Return(&eSecret, nil),
					secrets.EXPECT().Patch("cattle-tokens", "token-12345", types.JSONPatchType, gomock.Any()).
						DoAndReturn(func(space, name string, pt types.PatchType, data []byte, subresources ...any) (*ext.Token, error) {
							// patchData = data
							return nil, nil
						}).Times(1),
				)
			},
			want: wantUserActivity,
		},
		{
			name: "seenAt is greater than lastSeenAt",
			args: args{
				ctx: ctxAdmin,
				obj: &ext.UserActivity{
					ObjectMeta: metav1.ObjectMeta{
						Name: "token-12345",
					},
					Spec: ext.UserActivitySpec{
						SeenAt: &metav1.Time{
							Time: mockNow.Add(-5 * time.Minute),
						},
					},
				},
			},
			mockSetup: func() {
				gomock.InOrder(
					userCache.EXPECT().Get("admin").Return(&apiv3.User{
						ObjectMeta: metav1.ObjectMeta{
							Name: "admin",
						},
					}, nil),

					tokenCache.EXPECT().Get("token-12345").Return(&apiv3.Token{
						ObjectMeta: metav1.ObjectMeta{
							Name: "token-12345",
						},
						AuthProvider:  "oidc",
						UserPrincipal: apiv3.Principal{},
					}, nil),
					tokenCache.EXPECT().Get("token-12345").Return(&apiv3.Token{
						ObjectMeta: metav1.ObjectMeta{
							Name: "token-12345",
							Labels: map[string]string{
								tokens.TokenKindLabel: "session",
							},
						},
						ActivityLastSeenAt: &metav1.Time{
							Time: mockNow.Add(-10 * time.Minute),
						},
						AuthProvider:  "oidc",
						UserPrincipal: apiv3.Principal{},
					}, nil),
					tokenController.EXPECT().
						Patch("token-12345", types.JSONPatchType, gomock.Any()).
						Return(&apiv3.Token{}, nil),
				)
			},
			want: &ext.UserActivity{
				ObjectMeta: metav1.ObjectMeta{
					Name: "token-12345",
				},
				Spec: ext.UserActivitySpec{
					SeenAt: &metav1.Time{
						Time: mockNow.Add(-5 * time.Minute),
					},
				},
				Status: ext.UserActivityStatus{
					ExpiresAt: mockNow.Add((defaultIdleTTL - 5) * time.Minute).Format(time.RFC3339),
				},
			},
		},
		{
			name: "seenAt is set, no lastSeenAt",
			args: args{
				ctx: ctxAdmin,
				obj: &ext.UserActivity{
					ObjectMeta: metav1.ObjectMeta{
						Name: "token-12345",
					},
					Spec: ext.UserActivitySpec{
						SeenAt: &metav1.Time{
							Time: mockNow.Add(-5 * time.Minute),
						},
					},
				},
			},
			mockSetup: func() {
				gomock.InOrder(
					userCache.EXPECT().Get("admin").Return(&apiv3.User{
						ObjectMeta: metav1.ObjectMeta{
							Name: "admin",
						},
					}, nil),

					tokenCache.EXPECT().Get("token-12345").Return(&apiv3.Token{
						ObjectMeta: metav1.ObjectMeta{
							Name: "token-12345",
						},
						AuthProvider:  "oidc",
						UserPrincipal: apiv3.Principal{},
					}, nil),
					tokenCache.EXPECT().Get("token-12345").Return(&apiv3.Token{
						ObjectMeta: metav1.ObjectMeta{
							Name: "token-12345",
							Labels: map[string]string{
								tokens.TokenKindLabel: "session",
							},
						},
						AuthProvider:  "oidc",
						UserPrincipal: apiv3.Principal{},
					}, nil),
					tokenController.EXPECT().
						Patch("token-12345", types.JSONPatchType, gomock.Any()).
						Return(&apiv3.Token{}, nil),
				)
			},
			want: &ext.UserActivity{
				ObjectMeta: metav1.ObjectMeta{
					Name: "token-12345",
				},
				Spec: ext.UserActivitySpec{
					SeenAt: &metav1.Time{
						Time: mockNow.Add(-5 * time.Minute),
					},
				},
				Status: ext.UserActivityStatus{
					ExpiresAt: mockNow.Add((defaultIdleTTL - 5) * time.Minute).Format(time.RFC3339),
				},
			},
		},
		{
			name: "seenAt is less than lastSeenAt",
			args: args{
				ctx: ctxAdmin,
				obj: &ext.UserActivity{
					ObjectMeta: metav1.ObjectMeta{
						Name: "token-12345",
					},
					Spec: ext.UserActivitySpec{
						SeenAt: &metav1.Time{
							Time: mockNow.Add(-10 * time.Minute),
						},
					},
				},
			},
			mockSetup: func() {
				gomock.InOrder(
					userCache.EXPECT().Get("admin").Return(&apiv3.User{
						ObjectMeta: metav1.ObjectMeta{
							Name: "admin",
						},
					}, nil),

					tokenCache.EXPECT().Get("token-12345").Return(&apiv3.Token{
						ObjectMeta: metav1.ObjectMeta{
							Name: "token-12345",
						},
						AuthProvider:  "oidc",
						UserPrincipal: apiv3.Principal{},
					}, nil),
					tokenCache.EXPECT().Get("token-12345").Return(&apiv3.Token{
						ObjectMeta: metav1.ObjectMeta{
							Name: "token-12345",
							Labels: map[string]string{
								tokens.TokenKindLabel: "session",
							},
						},
						ActivityLastSeenAt: &metav1.Time{
							Time: mockNow.Add(-5 * time.Minute),
						},
						AuthProvider:  "oidc",
						UserPrincipal: apiv3.Principal{},
					}, nil),
				)
			},
			want: &ext.UserActivity{
				ObjectMeta: metav1.ObjectMeta{
					Name: "token-12345",
				},
				Spec: ext.UserActivitySpec{
					SeenAt: &metav1.Time{
						Time: mockNow.Add(-10 * time.Minute),
					},
				},
				Status: ext.UserActivityStatus{
					ExpiresAt: mockNow.Add((defaultIdleTTL - 5) * time.Minute).Format(time.RFC3339),
				},
			},
		},
		{
			name: "seenAt is in the future",
			args: args{
				ctx: ctxAdmin,
				obj: &ext.UserActivity{
					ObjectMeta: metav1.ObjectMeta{
						Name: "token-12345",
					},
					Spec: ext.UserActivitySpec{
						SeenAt: &metav1.Time{
							Time: mockNow.Add(1 * time.Second),
						},
					},
				},
			},
			mockSetup: func() {
				gomock.InOrder(
					userCache.EXPECT().Get("admin").Return(&apiv3.User{
						ObjectMeta: metav1.ObjectMeta{
							Name: "admin",
						},
					}, nil),

					tokenCache.EXPECT().Get("token-12345").Return(&apiv3.Token{
						ObjectMeta: metav1.ObjectMeta{
							Name: "token-12345",
						},
						AuthProvider:  "oidc",
						UserPrincipal: apiv3.Principal{},
					}, nil),
					tokenCache.EXPECT().Get("token-12345").Return(&apiv3.Token{
						ObjectMeta: metav1.ObjectMeta{
							Name: "token-12345",
							Labels: map[string]string{
								tokens.TokenKindLabel: "session",
							},
						},
						ActivityLastSeenAt: &metav1.Time{
							Time: mockNow.Add(-5 * time.Minute),
						},
						AuthProvider:  "oidc",
						UserPrincipal: apiv3.Principal{},
					}, nil),
					tokenController.EXPECT().
						Patch("token-12345", types.JSONPatchType, gomock.Any()).
						Return(&apiv3.Token{}, nil),
				)
			},
			want: &ext.UserActivity{
				ObjectMeta: metav1.ObjectMeta{
					Name: "token-12345",
				},
				Spec: ext.UserActivitySpec{
					SeenAt: &metav1.Time{
						Time: mockNow.Add(1 * time.Second),
					},
				},
				Status: ext.UserActivityStatus{
					ExpiresAt: expectedExiresAt.Format(time.RFC3339),
				},
			},
		},
		{
			name: "username not found",
			args: args{
				ctx: request.WithUser(context.Background(), &k8suser.DefaultInfo{
					Name:   "user-xyz",
					Groups: []string{GroupCattleAuthenticated},
					Extra: map[string][]string{
						common.ExtraRequestTokenID: {"token-12345"},
					},
				}),
				obj: userActivity,
			},
			mockSetup: func() {
				gomock.InOrder(
					userCache.EXPECT().Get("user-xyz").Return(
						nil, fmt.Errorf("user not found"),
					),
				)
			},
			wantErr: true,
		},
		{
			name: "tokens dont match",
			args: args{
				ctx: ctxAdmin,
				obj: userActivity,
			},
			mockSetup: func() {
				gomock.InOrder(
					userCache.EXPECT().Get("admin").Return(&apiv3.User{
						ObjectMeta: metav1.ObjectMeta{
							Name: "admin",
						},
					}, nil),

					tokenCache.EXPECT().Get("token-12345").Return(&apiv3.Token{
						ObjectMeta: metav1.ObjectMeta{
							Name: "token-12345",
						},
						AuthProvider:  "oidc",
						UserPrincipal: apiv3.Principal{},
					}, nil),

					tokenCache.EXPECT().Get("token-12345").Return(&apiv3.Token{
						ObjectMeta: metav1.ObjectMeta{
							Name: "token-12345",
						},
						AuthProvider:  "local",
						UserPrincipal: apiv3.Principal{},
					}, nil),
				)
			},
			wantErr: true,
		},
		{
			name: "dry run",
			args: args{
				ctx: ctxAdmin,
				obj: userActivity,
				options: &metav1.CreateOptions{
					DryRun: []string{metav1.DryRunAll},
				},
			},
			mockSetup: func() {
				gomock.InOrder(
					userCache.EXPECT().Get("admin").Return(&apiv3.User{
						ObjectMeta: metav1.ObjectMeta{
							Name: "admin",
						},
					}, nil),

					tokenCache.EXPECT().Get("token-12345").Return(&apiv3.Token{
						ObjectMeta: metav1.ObjectMeta{
							Name: "token-12345",
						},
						AuthProvider:  "oidc",
						UserPrincipal: apiv3.Principal{},
					}, nil),

					tokenCache.EXPECT().Get("token-12345").Return(&apiv3.Token{
						ObjectMeta: metav1.ObjectMeta{
							Name: "token-12345",
							Labels: map[string]string{
								tokens.TokenKindLabel: "session",
							},
						},
						AuthProvider:  "oidc",
						UserPrincipal: apiv3.Principal{},
					}, nil),
				)
			},
			want: wantUserActivity,
		},
	}

	origTimeNow := timeNow
	timeNow = func() time.Time { return mockNow }
	defer func() { timeNow = origTimeNow }() // Restore original function after test

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenController = fake.NewMockNonNamespacedControllerInterface[*apiv3.Token, *apiv3.TokenList](ctrl)
			tokenCache = fake.NewMockNonNamespacedCacheInterface[*apiv3.Token](ctrl)
			userCache = fake.NewMockNonNamespacedCacheInterface[*apiv3.User](ctrl)
			secrets = fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
			secretCache = fake.NewMockCacheInterface[*corev1.Secret](ctrl)
			users = fake.NewMockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList](ctrl)
			users.EXPECT().Cache().Return(nil)
			secrets.EXPECT().Cache().Return(secretCache)

			tt.mockSetup()

			store := &Store{
				tokens:                           tokenController,
				userCache:                        userCache,
				extTokenStore:                    exttokens.NewSystem(nil, nil, secrets, users, tokenCache, nil, nil, nil, nil),
				getAuthUserSessionIdleTTLMinutes: func() int { return defaultIdleTTL },
			}

			got, err := store.Create(tt.args.ctx, tt.args.obj, tt.args.validateFunc, tt.args.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("Got error %v want %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Got\n %#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

func TestStoreGet(t *testing.T) {
	ctrl := gomock.NewController(t)

	var (
		tokenController *fake.MockNonNamespacedControllerInterface[*apiv3.Token, *apiv3.TokenList]
		tokenCache      *fake.MockNonNamespacedCacheInterface[*apiv3.Token]
		userCache       *fake.MockNonNamespacedCacheInterface[*apiv3.User]
		secrets         *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]
		secretCache     *fake.MockCacheInterface[*corev1.Secret]
		users           *fake.MockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList]
	)
	contextBG := context.Background()
	type args struct {
		ctx  context.Context
		name string
	}
	tests := []struct {
		name      string
		args      args
		mockSetup func()
		want      runtime.Object
		wantErr   bool
	}{
		{
			name: "valid useractivity retrieved",
			args: args{
				ctx:  ctxAdmin,
				name: "token-12345",
			},
			mockSetup: func() {
				tokenCache.EXPECT().Get(gomock.Any()).Return(&apiv3.Token{
					ObjectMeta: metav1.ObjectMeta{
						Name: "token-12345",
						Labels: map[string]string{
							tokens.TokenKindLabel: "session",
						},
					},
					UserID: "admin",
					ActivityLastSeenAt: &metav1.Time{
						Time: time.Date(2025, 1, 31, 0, 44, 0, 0, time.UTC),
					},
				}, nil).AnyTimes()
				userCache.EXPECT().Get(gomock.Any()).Return(
					&apiv3.User{}, nil,
				)
			},
			want: &ext.UserActivity{
				ObjectMeta: metav1.ObjectMeta{
					Name: "token-12345",
				},
				Status: ext.UserActivityStatus{
					ExpiresAt: time.Date(2025, 1, 31, 16, 44, 0, 0, time.UTC).Format(time.RFC3339),
				},
			},
			wantErr: false,
		},
		{
			name: "valid useractivity retrieved, ext token",
			args: args{
				ctx:  ctxAdmin,
				name: "token-12345",
			},
			mockSetup: func() {
				ePrincipal := ext.TokenPrincipal{
					Name:        "world",
					Provider:    "oidc",
					DisplayName: "",
					LoginName:   "hello",
				}
				ePrincipalBytes, _ := json.Marshal(ePrincipal)
				eSecret := corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "token-12345",
						Labels: map[string]string{
							exttokens.UserIDLabel:     "admin",
							exttokens.SecretKindLabel: exttokens.SecretKindLabelValue,
						},
					},
					Data: map[string][]byte{
						exttokens.FieldDescription:      []byte(""),
						exttokens.FieldEnabled:          []byte("true"),
						exttokens.FieldHash:             []byte("kla9jkdmj"),
						exttokens.FieldKind:             []byte(exttokens.IsLogin),
						exttokens.FieldLastActivitySeen: []byte("2025-01-31T00:44:00Z"),
						exttokens.FieldLastUpdateTime:   []byte("13:00:05"),
						exttokens.FieldPrincipal:        ePrincipalBytes,
						exttokens.FieldTTL:              []byte("4000"),
						exttokens.FieldUID:              []byte("2905498-kafld-lkad"),
						exttokens.FieldUserID:           []byte("lkajdlksjlkds"),
					},
				}

				tokenCache.EXPECT().Get(gomock.Any()).
					Return(nil, fmt.Errorf("some error")).
					AnyTimes()
				secretCache.EXPECT().Get("cattle-tokens", gomock.Any()).
					Return(&eSecret, nil).
					AnyTimes()
				userCache.EXPECT().Get(gomock.Any()).
					Return(&apiv3.User{}, nil)
			},
			want: &ext.UserActivity{
				ObjectMeta: metav1.ObjectMeta{
					Name: "token-12345",
				},
				Status: ext.UserActivityStatus{
					ExpiresAt: time.Date(2025, 1, 31, 16, 44, 0, 0, time.UTC).Format(time.RFC3339),
				},
			},
			wantErr: false,
		},
		{
			name: "invalid useractivity name",
			args: args{
				ctx:  contextBG,
				name: "ua_admin_token_12345",
			},
			mockSetup: func() {},
			want:      nil,
			wantErr:   true,
		},
		{
			name: "invalid token retrieved",
			args: args{
				ctx:  contextBG,
				name: "ua_admin_token-12345",
			},
			mockSetup: func() {
				tokenCache.EXPECT().Get(gomock.Any()).Return(nil, fmt.Errorf("invalid token name")).AnyTimes()
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid user name retrieved",
			args: args{
				ctx:  contextBG,
				name: "ua_user1_token-12345",
			},
			mockSetup: func() {
				tokenCache.EXPECT().Get(gomock.Any()).Return(&apiv3.Token{
					UserID: "token-12345",
					ActivityLastSeenAt: &metav1.Time{
						Time: time.Date(2025, 1, 31, 16, 44, 0, 0, time.UTC),
					},
				}, nil).AnyTimes()
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenController = fake.NewMockNonNamespacedControllerInterface[*apiv3.Token, *apiv3.TokenList](ctrl)
			tokenCache = fake.NewMockNonNamespacedCacheInterface[*apiv3.Token](ctrl)
			userCache = fake.NewMockNonNamespacedCacheInterface[*apiv3.User](ctrl)
			secrets = fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
			secretCache = fake.NewMockCacheInterface[*corev1.Secret](ctrl)
			users = fake.NewMockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList](ctrl)
			users.EXPECT().Cache().Return(nil)
			secrets.EXPECT().Cache().Return(secretCache)

			tt.mockSetup()

			store := &Store{
				tokens:                           tokenController,
				userCache:                        userCache,
				extTokenStore:                    exttokens.NewSystem(nil, nil, secrets, users, tokenCache, nil, nil, nil, nil),
				getAuthUserSessionIdleTTLMinutes: func() int { return defaultIdleTTL },
			}

			got, err := store.Get(tt.args.ctx, tt.args.name, &metav1.GetOptions{})
			if (err != nil) != tt.wantErr {
				t.Errorf("Got error %v, want %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Got\n%v\n, want\n%v", got, tt.want)
			}
		})
	}
}
