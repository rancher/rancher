package scimconfig

import (
	"testing"

	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestConfigUserID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		cfg        Config
		userName   string
		externalID string
		want       string
	}{
		{
			name:     "default returns userName",
			cfg:      Default(),
			userName: "john.doe", externalID: "ext-123",
			want: "john.doe",
		},
		{
			name:     "explicit userName returns userName",
			cfg:      Config{UserIDAttribute: UserIDUserName},
			userName: "john.doe", externalID: "ext-123",
			want: "john.doe",
		},
		{
			name:     "externalId returns externalId",
			cfg:      Config{UserIDAttribute: UserIDExternalID},
			userName: "john.doe", externalID: "ext-123",
			want: "ext-123",
		},
		{
			name:     "externalId with empty externalId returns empty",
			cfg:      Config{UserIDAttribute: UserIDExternalID},
			userName: "john.doe",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.cfg.UserID(tt.userName, tt.externalID))
		})
	}
}

func TestConfigGroupID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		cfg         Config
		displayName string
		externalID  string
		want        string
	}{
		{
			name:        "default returns displayName",
			cfg:         Default(),
			displayName: "Engineering",
			externalID:  "ext-grp-1",
			want:        "Engineering",
		},
		{
			name:        "explicit displayName returns displayName",
			cfg:         Config{GroupIDAttribute: GroupIDDisplayName},
			displayName: "Engineering",
			externalID:  "ext-grp-1",
			want:        "Engineering",
		},
		{
			name:        "externalId returns externalId",
			cfg:         Config{GroupIDAttribute: GroupIDExternalID},
			displayName: "Engineering",
			externalID:  "ext-grp-1",
			want:        "ext-grp-1",
		},
		{
			name:        "externalId with empty externalId returns empty",
			cfg:         Config{GroupIDAttribute: GroupIDExternalID},
			displayName: "Engineering",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.cfg.GroupID(tt.displayName, tt.externalID))
		})
	}
}

func TestDefault(t *testing.T) {
	t.Parallel()

	cfg := Default()
	assert.False(t, cfg.Enabled)
	assert.False(t, cfg.Paused)
	assert.Equal(t, UserIDUserName, cfg.UserIDAttribute)
	assert.Equal(t, GroupIDDisplayName, cfg.GroupIDAttribute)
	assert.Equal(t, 0, cfg.RateLimitRequestsPerSecond)
	assert.Equal(t, defaultRateLimitBurst, cfg.RateLimitBurst)
}

func TestGet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setup      func(*fake.MockCacheInterface[*corev1.ConfigMap])
		wantConfig Config
	}{
		{
			name: "configmap not found returns defaults",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(Namespace, "scim-config-azuread").
					Return(nil, apierrors.NewNotFound(schema.GroupResource{}, "scim-config-azuread"))
			},
			wantConfig: Default(),
		},
		{
			name: "empty configmap returns defaults",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(Namespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
					}, nil)
			},
			wantConfig: Default(),
		},
		{
			name: "unknown keys are ignored",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(Namespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data:       map[string]string{"other": "value"},
					}, nil)
			},
			wantConfig: Default(),
		},
		{
			name: "enabled true",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(Namespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data:       map[string]string{"enabled": "true"},
					}, nil)
			},
			wantConfig: Config{Enabled: true, UserIDAttribute: UserIDUserName, GroupIDAttribute: GroupIDDisplayName, RateLimitBurst: defaultRateLimitBurst},
		},
		{
			name: "enabled false",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(Namespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data:       map[string]string{"enabled": "false"},
					}, nil)
			},
			wantConfig: Default(),
		},
		{
			name: "enabled missing defaults to false",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(Namespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data:       map[string]string{"userIdAttribute": "externalId"},
					}, nil)
			},
			wantConfig: Config{Enabled: false, UserIDAttribute: UserIDExternalID, GroupIDAttribute: GroupIDDisplayName, RateLimitBurst: defaultRateLimitBurst},
		},
		{
			name: "paused true",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(Namespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data:       map[string]string{"enabled": "true", "paused": "true"},
					}, nil)
			},
			wantConfig: Config{Enabled: true, Paused: true, UserIDAttribute: UserIDUserName, GroupIDAttribute: GroupIDDisplayName, RateLimitBurst: defaultRateLimitBurst},
		},
		{
			name: "invalid bool value for enabled defaults to false",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(Namespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data:       map[string]string{"enabled": "yes"},
					}, nil)
			},
			wantConfig: Default(),
		},
		{
			name: "externalId for both id attributes",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(Namespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data: map[string]string{
							"enabled":          "true",
							"userIdAttribute":  "externalId",
							"groupIdAttribute": "externalId",
						},
					}, nil)
			},
			wantConfig: Config{Enabled: true, UserIDAttribute: UserIDExternalID, GroupIDAttribute: GroupIDExternalID, RateLimitBurst: defaultRateLimitBurst},
		},
		{
			name: "invalid attribute values fall back to defaults",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(Namespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data: map[string]string{
							"userIdAttribute":  "bogus",
							"groupIdAttribute": "invalid",
						},
					}, nil)
			},
			wantConfig: Default(),
		},
		{
			name: "partial config only sets specified attribute",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(Namespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data:       map[string]string{"userIdAttribute": "externalId"},
					}, nil)
			},
			wantConfig: Config{UserIDAttribute: UserIDExternalID, GroupIDAttribute: GroupIDDisplayName, RateLimitBurst: defaultRateLimitBurst},
		},
		{
			name: "rate limit config",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(Namespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data: map[string]string{
							"rateLimitRequestsPerSecond": "50",
							"rateLimitBurst":             "100",
						},
					}, nil)
			},
			wantConfig: Config{UserIDAttribute: UserIDUserName, GroupIDAttribute: GroupIDDisplayName, RateLimitRequestsPerSecond: 50, RateLimitBurst: 100},
		},
		{
			name: "invalid rate limit values fall back to defaults",
			setup: func(cache *fake.MockCacheInterface[*corev1.ConfigMap]) {
				cache.EXPECT().Get(Namespace, "scim-config-azuread").
					Return(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "scim-config-azuread"},
						Data: map[string]string{
							"rateLimitRequestsPerSecond": "not-a-number",
							"rateLimitBurst":             "also-bad",
						},
					}, nil)
			},
			wantConfig: Default(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			cache := fake.NewMockCacheInterface[*corev1.ConfigMap](ctrl)
			tt.setup(cache)

			cfg := Get(cache, "azuread")
			assert.Equal(t, tt.wantConfig, cfg)
		})
	}
}

func TestEnabled(t *testing.T) {
	tests := []struct {
		name    string
		prime   bool
		feature bool
		data    map[string]string
		want    bool
	}{
		{name: "not a Prime build", prime: false, feature: true, data: map[string]string{"enabled": "true"}, want: false},
		{name: "feature off", prime: true, feature: false, data: map[string]string{"enabled": "true"}, want: false},
		{name: "enabled", prime: true, feature: true, data: map[string]string{"enabled": "true"}, want: true},
		{name: "paused counts as enabled", prime: true, feature: true, data: map[string]string{"enabled": "true", "paused": "true"}, want: true},
		{name: "disabled", prime: true, feature: true, data: map[string]string{"enabled": "false"}, want: false},
		{name: "enabled key missing", prime: true, feature: true, data: map[string]string{}, want: false},
		{name: "invalid enabled value", prime: true, feature: true, data: map[string]string{"enabled": "yes please"}, want: false},
		{name: "no configmap", prime: true, feature: true, data: nil, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Not parallel: the feature flag and the environment are global.
			primeValue := ""
			if tt.prime {
				primeValue = "prime"
			}
			t.Setenv("RANCHER_VERSION_TYPE", primeValue)
			features.SCIM.Set(tt.feature)
			t.Cleanup(features.SCIM.Unset)

			ctrl := gomock.NewController(t)
			cache := fake.NewMockCacheInterface[*corev1.ConfigMap](ctrl)
			if tt.data == nil {
				cache.EXPECT().Get(Namespace, "scim-config-okta").
					Return(nil, apierrors.NewNotFound(schema.GroupResource{}, "scim-config-okta")).AnyTimes()
			} else {
				cache.EXPECT().Get(Namespace, "scim-config-okta").
					Return(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "scim-config-okta"}, Data: tt.data}, nil).AnyTimes()
			}

			assert.Equal(t, tt.want, Enabled(cache, "okta"))
		})
	}
}
