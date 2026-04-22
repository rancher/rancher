package scim

import (
	"testing"

	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestProviderConfigUserID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  providerConfig
		user scimUser
		want string
	}{
		{
			name: "default returns userName",
			cfg:  defaultProviderConfig(),
			user: scimUser{UserName: "john.doe", ExternalID: "ext-123"},
			want: "john.doe",
		},
		{
			name: "explicit userName returns userName",
			cfg:  providerConfig{UserIDAttribute: UserIDUserName},
			user: scimUser{UserName: "john.doe", ExternalID: "ext-123"},
			want: "john.doe",
		},
		{
			name: "externalId returns externalId",
			cfg:  providerConfig{UserIDAttribute: UserIDExternalID},
			user: scimUser{UserName: "john.doe", ExternalID: "ext-123"},
			want: "ext-123",
		},
		{
			name: "externalId with empty externalId returns empty",
			cfg:  providerConfig{UserIDAttribute: UserIDExternalID},
			user: scimUser{UserName: "john.doe"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.cfg.userID(tt.user))
		})
	}
}

func TestProviderConfigGroupID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		cfg         providerConfig
		displayName string
		externalID  string
		want        string
	}{
		{
			name:        "default returns displayName",
			cfg:         defaultProviderConfig(),
			displayName: "Engineering",
			externalID:  "ext-grp-1",
			want:        "Engineering",
		},
		{
			name:        "explicit displayName returns displayName",
			cfg:         providerConfig{GroupIDAttribute: GroupIDDisplayName},
			displayName: "Engineering",
			externalID:  "ext-grp-1",
			want:        "Engineering",
		},
		{
			name:        "externalId returns externalId",
			cfg:         providerConfig{GroupIDAttribute: GroupIDExternalID},
			displayName: "Engineering",
			externalID:  "ext-grp-1",
			want:        "ext-grp-1",
		},
		{
			name:        "externalId with empty externalId returns empty",
			cfg:         providerConfig{GroupIDAttribute: GroupIDExternalID},
			displayName: "Engineering",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.cfg.groupID(tt.displayName, tt.externalID))
		})
	}
}

func TestDefaultProviderConfig(t *testing.T) {
	t.Parallel()

	cfg := defaultProviderConfig()
	assert.Equal(t, UserIDUserName, cfg.UserIDAttribute)
	assert.Equal(t, GroupIDDisplayName, cfg.GroupIDAttribute)
}

func TestGetProviderConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func(*fake.MockCacheInterface[*corev1.ConfigMap])
		wantUser string
		wantGroup string
	}{
		{
			name: "configmap not found returns defaults",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(tokenSecretNamespace, "scim-config-azuread").
					Return(nil, apierrors.NewNotFound(schema.GroupResource{}, "scim-config-azuread"))
			},
			wantUser:  UserIDUserName,
			wantGroup: GroupIDDisplayName,
		},
		{
			name: "configmap missing config key returns defaults",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(tokenSecretNamespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data:       map[string]string{"other": "value"},
					}, nil)
			},
			wantUser:  UserIDUserName,
			wantGroup: GroupIDDisplayName,
		},
		{
			name: "valid config with externalId for both",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(tokenSecretNamespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data: map[string]string{
							"config": `{"userIdAttribute":"externalId","groupIdAttribute":"externalId"}`,
						},
					}, nil)
			},
			wantUser:  UserIDExternalID,
			wantGroup: GroupIDExternalID,
		},
		{
			name: "invalid JSON returns defaults",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(tokenSecretNamespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data:       map[string]string{"config": "{invalid json"},
					}, nil)
			},
			wantUser:  UserIDUserName,
			wantGroup: GroupIDDisplayName,
		},
		{
			name: "empty attribute values filled with defaults",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(tokenSecretNamespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data:       map[string]string{"config": `{}`},
					}, nil)
			},
			wantUser:  UserIDUserName,
			wantGroup: GroupIDDisplayName,
		},
		{
			name: "invalid attribute values fall back to defaults",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(tokenSecretNamespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data: map[string]string{
							"config": `{"userIdAttribute":"bogus","groupIdAttribute":"invalid"}`,
						},
					}, nil)
			},
			wantUser:  UserIDUserName,
			wantGroup: GroupIDDisplayName,
		},
		{
			name: "partial config only sets specified attribute",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(tokenSecretNamespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data: map[string]string{
							"config": `{"userIdAttribute":"externalId"}`,
						},
					}, nil)
			},
			wantUser:  UserIDExternalID,
			wantGroup: GroupIDDisplayName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			cache := fake.NewMockCacheInterface[*corev1.ConfigMap](ctrl)
			tt.setup(cache)

			cfg := getProviderConfig(cache, "azuread")
			assert.Equal(t, tt.wantUser, cfg.UserIDAttribute)
			assert.Equal(t, tt.wantGroup, cfg.GroupIDAttribute)
		})
	}
}

func TestUserPrincipalName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "okta_user://john.doe", userPrincipalName("okta", "john.doe"))
	assert.Equal(t, "azuread_user://obj-123", userPrincipalName("azuread", "obj-123"))
}

func TestGroupPrincipalName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "okta_group://Engineering", groupPrincipalName("okta", "Engineering"))
	assert.Equal(t, "azuread_group://obj-456", groupPrincipalName("azuread", "obj-456"))
}
