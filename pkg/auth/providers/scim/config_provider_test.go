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
	assert.False(t, cfg.Enabled)
	assert.False(t, cfg.Paused)
	assert.Equal(t, UserIDUserName, cfg.UserIDAttribute)
	assert.Equal(t, GroupIDDisplayName, cfg.GroupIDAttribute)
	assert.Equal(t, 0, cfg.RateLimitRequestsPerSecond)
	assert.Equal(t, defaultRateLimitBurst, cfg.RateLimitBurst)
}

func TestGetProviderConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setup      func(*fake.MockCacheInterface[*corev1.ConfigMap])
		wantConfig providerConfig
	}{
		{
			name: "configmap not found returns defaults",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(tokenSecretNamespace, "scim-config-azuread").
					Return(nil, apierrors.NewNotFound(schema.GroupResource{}, "scim-config-azuread"))
			},
			wantConfig: defaultProviderConfig(),
		},
		{
			name: "empty configmap returns defaults",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(tokenSecretNamespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
					}, nil)
			},
			wantConfig: defaultProviderConfig(),
		},
		{
			name: "unknown keys are ignored",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(tokenSecretNamespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data:       map[string]string{"other": "value"},
					}, nil)
			},
			wantConfig: defaultProviderConfig(),
		},
		{
			name: "enabled true",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(tokenSecretNamespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data:       map[string]string{"enabled": "true"},
					}, nil)
			},
			wantConfig: providerConfig{Enabled: true, UserIDAttribute: UserIDUserName, GroupIDAttribute: GroupIDDisplayName, RateLimitBurst: defaultRateLimitBurst},
		},
		{
			name: "enabled false",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(tokenSecretNamespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data:       map[string]string{"enabled": "false"},
					}, nil)
			},
			wantConfig: defaultProviderConfig(),
		},
		{
			name: "enabled missing defaults to false",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(tokenSecretNamespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data:       map[string]string{"userIdAttribute": "externalId"},
					}, nil)
			},
			wantConfig: providerConfig{Enabled: false, UserIDAttribute: UserIDExternalID, GroupIDAttribute: GroupIDDisplayName, RateLimitBurst: defaultRateLimitBurst},
		},
		{
			name: "paused true",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(tokenSecretNamespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data:       map[string]string{"enabled": "true", "paused": "true"},
					}, nil)
			},
			wantConfig: providerConfig{Enabled: true, Paused: true, UserIDAttribute: UserIDUserName, GroupIDAttribute: GroupIDDisplayName, RateLimitBurst: defaultRateLimitBurst},
		},
		{
			name: "invalid bool value for enabled defaults to false",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(tokenSecretNamespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data:       map[string]string{"enabled": "yes"},
					}, nil)
			},
			wantConfig: defaultProviderConfig(),
		},
		{
			name: "externalId for both id attributes",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(tokenSecretNamespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data: map[string]string{
							"enabled":          "true",
							"userIdAttribute":  "externalId",
							"groupIdAttribute": "externalId",
						},
					}, nil)
			},
			wantConfig: providerConfig{Enabled: true, UserIDAttribute: UserIDExternalID, GroupIDAttribute: GroupIDExternalID, RateLimitBurst: defaultRateLimitBurst},
		},
		{
			name: "invalid attribute values fall back to defaults",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(tokenSecretNamespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data: map[string]string{
							"userIdAttribute":  "bogus",
							"groupIdAttribute": "invalid",
						},
					}, nil)
			},
			wantConfig: defaultProviderConfig(),
		},
		{
			name: "partial config only sets specified attribute",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(tokenSecretNamespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data:       map[string]string{"userIdAttribute": "externalId"},
					}, nil)
			},
			wantConfig: providerConfig{UserIDAttribute: UserIDExternalID, GroupIDAttribute: GroupIDDisplayName, RateLimitBurst: defaultRateLimitBurst},
		},
		{
			name: "rate limit config",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(tokenSecretNamespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data: map[string]string{
							"rateLimitRequestsPerSecond": "50",
							"rateLimitBurst":             "100",
						},
					}, nil)
			},
			wantConfig: providerConfig{UserIDAttribute: UserIDUserName, GroupIDAttribute: GroupIDDisplayName, RateLimitRequestsPerSecond: 50, RateLimitBurst: 100},
		},
		{
			name: "invalid rate limit values fall back to defaults",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(tokenSecretNamespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data: map[string]string{
							"rateLimitRequestsPerSecond": "not-a-number",
							"rateLimitBurst":             "also-bad",
						},
					}, nil)
			},
			wantConfig: defaultProviderConfig(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			cache := fake.NewMockCacheInterface[*corev1.ConfigMap](ctrl)
			tt.setup(cache)

			cfg := getProviderConfig(cache, "azuread")
			assert.Equal(t, tt.wantConfig, cfg)
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
