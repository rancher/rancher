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
	exttokenstore "github.com/rancher/rancher/pkg/ext/stores/tokens"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	wranglerfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8suser "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

func TestStoreCreate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var mockTokenControllerFake *wranglerfake.MockNonNamespacedControllerInterface[*apiv3.Token, *apiv3.TokenList]
	var mockTokenCacheFake *wranglerfake.MockNonNamespacedCacheInterface[*apiv3.Token]
	var mockUserCacheFake *wranglerfake.MockNonNamespacedCacheInterface[*apiv3.User]
	var secrets *wranglerfake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]
	var scache *wranglerfake.MockCacheInterface[*corev1.Secret]
	var users *wranglerfake.MockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList]
	var store *exttokenstore.SystemStore

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
			name: "valid useractivity is created",
			args: args{
				ctx: request.WithUser(context.Background(), &k8suser.DefaultInfo{
					Name:   "admin",
					Groups: []string{GroupCattleAuthenticated},
					Extra: map[string][]string{
						common.ExtraRequestTokenID: {"token-12345"},
					},
				}),
				obj: &ext.UserActivity{
					ObjectMeta: metav1.ObjectMeta{
						Name: "token-12345",
					},
				},
				validateFunc: nil,
				options:      nil,
			},
			mockSetup: func() {
				gomock.InOrder(
					mockUserCacheFake.EXPECT().Get("admin").Return(&v3.User{
						ObjectMeta: metav1.ObjectMeta{
							Name: "admin",
						},
					}, nil),

					mockTokenCacheFake.EXPECT().Get("token-12345").Return(&apiv3.Token{
						ObjectMeta: metav1.ObjectMeta{
							Name: "token-12345",
						},
						AuthProvider:  "oidc",
						UserPrincipal: v3.Principal{},
					}, nil),
					mockTokenCacheFake.EXPECT().Get("token-12345").Return(&apiv3.Token{
						ObjectMeta: metav1.ObjectMeta{
							Name: "token-12345",
							Labels: map[string]string{
								TokenKind: "session",
							},
						},
						AuthProvider:  "oidc",
						UserPrincipal: v3.Principal{},
					}, nil),
					mockTokenControllerFake.EXPECT().
						Patch("token-12345", types.JSONPatchType, gomock.Any()).
						Return(&apiv3.Token{}, nil),
				)
			},
			want: &ext.UserActivity{
				ObjectMeta: metav1.ObjectMeta{
					Name: "token-12345",
				},
				Status: ext.UserActivityStatus{
					ExpiresAt: metav1.NewTime(time.Date(2025, 2, 2, 0, 54, 0, 0, &time.Location{})).Format(time.RFC3339),
				},
			},
			wantErr: false,
		},
		{
			name: "valid useractivity is created, ext token",
			args: args{
				ctx: request.WithUser(context.Background(), &k8suser.DefaultInfo{
					Name:   "admin",
					Groups: []string{GroupCattleAuthenticated},
					Extra: map[string][]string{
						common.ExtraRequestTokenID: {"token-12345"},
					},
				}),
				obj: &ext.UserActivity{
					ObjectMeta: metav1.ObjectMeta{
						Name: "token-12345",
					},
				},
				validateFunc: nil,
				options:      nil,
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
					},
					Data: map[string][]byte{
						exttokenstore.FieldDescription:    []byte(""),
						exttokenstore.FieldEnabled:        []byte("true"),
						exttokenstore.FieldHash:           []byte("kla9jkdmj"),
						exttokenstore.FieldKind:           []byte(exttokenstore.IsLogin),
						exttokenstore.FieldLastUpdateTime: []byte("13:00:05"),
						exttokenstore.FieldPrincipal:      ePrincipalBytes,
						exttokenstore.FieldTTL:            []byte("4000"),
						exttokenstore.FieldUID:            []byte("2905498-kafld-lkad"),
						exttokenstore.FieldUserID:         []byte("lkajdlksjlkds"),
					},
				}
				gomock.InOrder(
					mockUserCacheFake.EXPECT().Get("admin").Return(&v3.User{
						ObjectMeta: metav1.ObjectMeta{
							Name: "admin",
						},
					}, nil),
					mockTokenCacheFake.EXPECT().Get("token-12345").
						Return(nil, fmt.Errorf("some error")),
					scache.EXPECT().
						Get("cattle-tokens", "token-12345").
						Return(&eSecret, nil),
					mockTokenCacheFake.EXPECT().Get("token-12345").
						Return(nil, fmt.Errorf("some error")),
					scache.EXPECT().
						Get("cattle-tokens", "token-12345").
						Return(&eSecret, nil),
					secrets.EXPECT().Patch("cattle-tokens", "token-12345", types.JSONPatchType, gomock.Any()).
						DoAndReturn(func(space, name string, pt types.PatchType, data []byte, subresources ...any) (*ext.Token, error) {
							// patchData = data
							return nil, nil
						}).Times(1),
				)
			},
			want: &ext.UserActivity{
				ObjectMeta: metav1.ObjectMeta{
					Name: "token-12345",
				},
				Status: ext.UserActivityStatus{
					ExpiresAt: metav1.NewTime(time.Date(2025, 2, 2, 0, 54, 0, 0, &time.Location{})).Format(time.RFC3339),
				},
			},
			wantErr: false,
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
				obj: &ext.UserActivity{
					ObjectMeta: metav1.ObjectMeta{
						Name: "token-12345",
					},
				},
				validateFunc: nil,
				options:      nil,
			},
			mockSetup: func() {
				gomock.InOrder(
					mockUserCacheFake.EXPECT().Get("user-xyz").Return(
						nil, fmt.Errorf("user not found"),
					),
				)
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "tokens dont match",
			args: args{
				ctx: request.WithUser(context.Background(), &k8suser.DefaultInfo{
					Name:   "admin",
					Groups: []string{GroupCattleAuthenticated},
					Extra: map[string][]string{
						common.ExtraRequestTokenID: {"token-12345"},
					},
				}),
				obj: &ext.UserActivity{
					ObjectMeta: metav1.ObjectMeta{
						Name: "token-12345",
					},
				},
				validateFunc: nil,
				options:      nil,
			},
			mockSetup: func() {
				gomock.InOrder(
					mockUserCacheFake.EXPECT().Get("admin").Return(&v3.User{
						ObjectMeta: metav1.ObjectMeta{
							Name: "admin",
						},
					}, nil),

					mockTokenCacheFake.EXPECT().Get("token-12345").Return(&apiv3.Token{
						ObjectMeta: metav1.ObjectMeta{
							Name: "token-12345",
						},
						AuthProvider:  "oidc",
						UserPrincipal: v3.Principal{},
					}, nil),

					mockTokenCacheFake.EXPECT().Get("token-12345").Return(&apiv3.Token{
						ObjectMeta: metav1.ObjectMeta{
							Name: "token-12345",
						},
						AuthProvider:  "local",
						UserPrincipal: v3.Principal{},
					}, nil),
				)
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "dry run",
			args: args{
				ctx: request.WithUser(context.Background(), &k8suser.DefaultInfo{
					Name:   "admin",
					Groups: []string{GroupCattleAuthenticated},
					Extra: map[string][]string{
						common.ExtraRequestTokenID: {"token-12345"},
					},
				}),
				obj: &ext.UserActivity{
					ObjectMeta: metav1.ObjectMeta{
						Name: "token-12345",
					},
				},
				validateFunc: nil,
				options: &metav1.CreateOptions{
					DryRun: []string{metav1.DryRunAll},
				},
			},
			mockSetup: func() {
				gomock.InOrder(
					mockUserCacheFake.EXPECT().Get("admin").Return(&v3.User{
						ObjectMeta: metav1.ObjectMeta{
							Name: "admin",
						},
					}, nil),

					mockTokenCacheFake.EXPECT().Get("token-12345").Return(&apiv3.Token{
						ObjectMeta: metav1.ObjectMeta{
							Name: "token-12345",
						},
						AuthProvider:  "oidc",
						UserPrincipal: v3.Principal{},
					}, nil),

					mockTokenCacheFake.EXPECT().Get("token-12345").Return(&apiv3.Token{
						ObjectMeta: metav1.ObjectMeta{
							Name: "token-12345",
							Labels: map[string]string{
								TokenKind: "session",
							},
						},
						AuthProvider:  "oidc",
						UserPrincipal: v3.Principal{},
					}, nil),
				)
			},
			want: &ext.UserActivity{
				ObjectMeta: metav1.ObjectMeta{
					Name: "token-12345",
				},
				Status: ext.UserActivityStatus{
					ExpiresAt: metav1.NewTime(time.Date(2025, 2, 2, 0, 54, 0, 0, &time.Location{})).Format(time.RFC3339),
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize and configure the mocks for each test separately

			mockTokenControllerFake = wranglerfake.NewMockNonNamespacedControllerInterface[*apiv3.Token, *apiv3.TokenList](ctrl)
			mockTokenCacheFake = wranglerfake.NewMockNonNamespacedCacheInterface[*apiv3.Token](ctrl)
			mockUserCacheFake = wranglerfake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)

			secrets = wranglerfake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
			scache = wranglerfake.NewMockCacheInterface[*corev1.Secret](ctrl)
			users = wranglerfake.NewMockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList](ctrl)

			users.EXPECT().Cache().Return(nil)
			secrets.EXPECT().Cache().Return(scache)

			store = exttokenstore.NewSystem(nil, nil, secrets, users, mockTokenCacheFake, nil, nil, nil, nil)

			uas := &Store{
				tokens:        mockTokenControllerFake,
				userCache:     mockUserCacheFake,
				extTokenStore: store,
			}

			// Mock the time function
			mockNow := time.Date(2025, 2, 1, 8, 54, 0, 0, time.UTC)
			origTimeNow := timeNow
			timeNow = func() time.Time { return mockNow }
			defer func() { timeNow = origTimeNow }() // Restore original function after test

			// Setup mocks
			tt.mockSetup()

			// Execute function
			got, err := uas.Create(tt.args.ctx, tt.args.obj, tt.args.validateFunc, tt.args.options)

			// Validate results
			if (err != nil) != tt.wantErr {
				t.Errorf("Store.Create() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Store.Create() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestStoreGet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var mockTokenControllerFake *wranglerfake.MockNonNamespacedControllerInterface[*apiv3.Token, *apiv3.TokenList]
	var mockTokenCacheFake *wranglerfake.MockNonNamespacedCacheInterface[*apiv3.Token]
	var mockUserCacheFake *wranglerfake.MockNonNamespacedCacheInterface[*apiv3.User]
	var secrets *wranglerfake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]
	var scache *wranglerfake.MockCacheInterface[*corev1.Secret]
	var users *wranglerfake.MockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList]
	var store *exttokenstore.SystemStore

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
				ctx: request.WithUser(context.Background(), &k8suser.DefaultInfo{
					Name:   "admin",
					Groups: []string{GroupCattleAuthenticated},
					Extra: map[string][]string{
						common.ExtraRequestTokenID: {"token-12345"},
					},
				}),
				name: "token-12345",
			},
			mockSetup: func() {
				mockTokenCacheFake.EXPECT().Get(gomock.Any()).Return(&apiv3.Token{
					ObjectMeta: metav1.ObjectMeta{
						Name: "token-12345",
						Labels: map[string]string{
							TokenKind: "session",
						},
					},
					UserID: "admin",
					ActivityLastSeenAt: &metav1.Time{
						Time: time.Date(2025, 1, 31, 16, 44, 0, 0, &time.Location{}),
					},
				}, nil).AnyTimes()
				mockUserCacheFake.EXPECT().Get(gomock.Any()).Return(
					&apiv3.User{}, nil,
				)
			},
			want: &ext.UserActivity{
				ObjectMeta: metav1.ObjectMeta{
					Name: "token-12345",
				},
				Status: ext.UserActivityStatus{
					ExpiresAt: time.Date(2025, 1, 31, 16, 44, 0, 0, &time.Location{}).String(),
				},
			},
			wantErr: false,
		},
		{
			name: "valid useractivity retrieved, ext token",
			args: args{
				ctx: request.WithUser(context.Background(), &k8suser.DefaultInfo{
					Name:   "admin",
					Groups: []string{GroupCattleAuthenticated},
					Extra: map[string][]string{
						common.ExtraRequestTokenID: {"token-12345"},
					},
				}),
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
					},
					Data: map[string][]byte{
						exttokenstore.FieldDescription:      []byte(""),
						exttokenstore.FieldEnabled:          []byte("true"),
						exttokenstore.FieldHash:             []byte("kla9jkdmj"),
						exttokenstore.FieldKind:             []byte(exttokenstore.IsLogin),
						exttokenstore.FieldLastActivitySeen: []byte("2025-01-31T16:44:00Z"),
						exttokenstore.FieldLastUpdateTime:   []byte("13:00:05"),
						exttokenstore.FieldPrincipal:        ePrincipalBytes,
						exttokenstore.FieldTTL:              []byte("4000"),
						exttokenstore.FieldUID:              []byte("2905498-kafld-lkad"),
						exttokenstore.FieldUserID:           []byte("lkajdlksjlkds"),
					},
				}

				mockTokenCacheFake.EXPECT().Get(gomock.Any()).
					Return(nil, fmt.Errorf("some error")).
					AnyTimes()
				scache.EXPECT().Get("cattle-tokens", gomock.Any()).
					Return(&eSecret, nil).
					AnyTimes()
				mockUserCacheFake.EXPECT().Get(gomock.Any()).
					Return(&apiv3.User{}, nil)
			},
			want: &ext.UserActivity{
				ObjectMeta: metav1.ObjectMeta{
					Name: "token-12345",
				},
				Status: ext.UserActivityStatus{
					ExpiresAt: time.Date(2025, 1, 31, 16, 44, 0, 0, &time.Location{}).String(),
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
				mockTokenCacheFake.EXPECT().Get(gomock.Any()).Return(nil, fmt.Errorf("invalid token name")).AnyTimes()
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
				mockTokenCacheFake.EXPECT().Get(gomock.Any()).Return(&apiv3.Token{
					UserID: "token-12345",
					ActivityLastSeenAt: &metav1.Time{
						Time: time.Date(2025, 1, 31, 16, 44, 0, 0, &time.Location{}),
					},
				}, nil).AnyTimes()
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		mockTokenControllerFake = wranglerfake.NewMockNonNamespacedControllerInterface[*apiv3.Token, *apiv3.TokenList](ctrl)
		mockTokenCacheFake = wranglerfake.NewMockNonNamespacedCacheInterface[*apiv3.Token](ctrl)
		mockUserCacheFake = wranglerfake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)

		secrets = wranglerfake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		scache = wranglerfake.NewMockCacheInterface[*corev1.Secret](ctrl)
		users = wranglerfake.NewMockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList](ctrl)
		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(scache)

		store = exttokenstore.NewSystem(nil, nil, secrets, users, mockTokenCacheFake, nil, nil, nil, nil)

		uas := &Store{
			tokens:        mockTokenControllerFake,
			userCache:     mockUserCacheFake,
			extTokenStore: store,
		}

		tt.mockSetup()

		t.Run(tt.name, func(t *testing.T) {
			got, err := uas.Get(tt.args.ctx, tt.args.name, &metav1.GetOptions{})
			if (err != nil) != tt.wantErr {
				t.Errorf("Store.get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Store.get() = %v, want %v", got, tt.want)
			}
		})
	}
}
