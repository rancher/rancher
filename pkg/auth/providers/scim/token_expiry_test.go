package scim

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

func newTokenSecret(name string, created time.Time) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         tokenSecretNamespace,
			UID:               types.UID(name + "-uid"),
			CreationTimestamp: metav1.NewTime(created),
			Labels: map[string]string{
				secretKindLabel:   scimAuthToken,
				authProviderLabel: "okta",
			},
		},
		Data: map[string][]byte{"token": []byte("token")},
	}
}

func TestTokenExpiryIgnoresOtherSecrets(t *testing.T) {
	t.Parallel()

	now := time.Now()
	deleting := newTokenSecret("deleting", now.Add(-2*time.Hour))
	deleting.DeletionTimestamp = &metav1.Time{Time: now}
	otherNamespace := newTokenSecret("other-namespace", now.Add(-2*time.Hour))
	otherNamespace.Namespace = "default"
	noKindLabel := newTokenSecret("no-kind-label", now.Add(-2*time.Hour))
	delete(noKindLabel.Labels, secretKindLabel)
	otherKind := newTokenSecret("other-kind", now.Add(-2*time.Hour))
	otherKind.Labels[secretKindLabel] = "something-else"

	tests := []struct {
		name   string
		secret *corev1.Secret
	}{
		{name: "nil", secret: nil},
		{name: "deleting", secret: deleting},
		{name: "other namespace", secret: otherNamespace},
		{name: "no kind label", secret: noKindLabel},
		{name: "other kind", secret: otherKind},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := &tokenExpiryHandler{
				secrets: fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl),
				expireTokensAfter: func() time.Duration {
					t.Error("expireTokensAfter called for a Secret that isn't a token")
					return time.Hour
				},
			}

			got, err := h.onSecretChange("", tt.secret)
			require.NoError(t, err)
			assert.Equal(t, tt.secret, got)
		})
	}
}

func TestTokenExpiryNeverExpires(t *testing.T) {
	t.Parallel()

	for _, ttl := range []time.Duration{0, -time.Hour} {
		t.Run(ttl.String(), func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			h := &tokenExpiryHandler{
				secrets:           fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl),
				expireTokensAfter: func() time.Duration { return ttl },
			}

			secret := newTokenSecret("old", time.Now().Add(-1000*time.Hour))
			got, err := h.onSecretChange("", secret)
			require.NoError(t, err)
			assert.Equal(t, secret, got)
		})
	}
}

func TestTokenExpiryRequeuesUnexpired(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		secrets.EXPECT().EnqueueAfter(tokenSecretNamespace, "current", 30*time.Minute).Times(1)

		h := &tokenExpiryHandler{
			secrets:           secrets,
			expireTokensAfter: func() time.Duration { return time.Hour },
		}

		secret := newTokenSecret("current", time.Now().Add(-30*time.Minute))
		got, err := h.onSecretChange("", secret)
		require.NoError(t, err)
		assert.Equal(t, secret, got)
	})
}

func TestTokenExpiryDeletesExpired(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		age       time.Duration
		deleteErr error
		wantErr   bool
	}{
		{name: "expired", age: 2 * time.Hour},
		{name: "expires now", age: time.Hour},
		{name: "already deleted", age: 2 * time.Hour, deleteErr: apierrors.NewNotFound(corev1.Resource("secrets"), "expired")},
		{name: "replaced", age: 2 * time.Hour, deleteErr: apierrors.NewConflict(corev1.Resource("secrets"), "expired", errors.New("uid mismatch"))},
		{name: "delete fails", age: 2 * time.Hour, deleteErr: errors.New("connection refused"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				secret := newTokenSecret("expired", time.Now().Add(-tt.age))

				ctrl := gomock.NewController(t)
				secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
				secrets.EXPECT().Delete(tokenSecretNamespace, "expired", &metav1.DeleteOptions{
					Preconditions: &metav1.Preconditions{UID: &secret.UID},
				}).Return(tt.deleteErr).Times(1)

				h := &tokenExpiryHandler{
					secrets:           secrets,
					expireTokensAfter: func() time.Duration { return time.Hour },
				}

				_, err := h.onSecretChange("", secret)
				if tt.wantErr {
					require.ErrorIs(t, err, tt.deleteErr)
					return
				}
				require.NoError(t, err)
			})
		})
	}
}

func TestTokenExpirySettingChange(t *testing.T) {
	t.Parallel()

	t.Run("other settings and deletes are ignored", func(t *testing.T) {
		t.Parallel()

		for _, setting := range []*v3.Setting{nil, {ObjectMeta: metav1.ObjectMeta{Name: "server-url"}}} {
			ctrl := gomock.NewController(t)
			h := &tokenExpiryHandler{
				secrets:     fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl),
				secretCache: fake.NewMockCacheInterface[*corev1.Secret](ctrl),
			}

			got, err := h.onSettingChange("", setting)
			require.NoError(t, err)
			assert.Equal(t, setting, got)
		}
	})

	t.Run("expiry setting requeues every token Secret", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		secretCache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
		secretCache.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(namespace string, selector labels.Selector) ([]*corev1.Secret, error) {
			assert.Equal(t, tokenSecretNamespace, namespace)
			assert.Equal(t, labels.Set{secretKindLabel: scimAuthToken}.AsSelector().String(), selector.String())

			return []*corev1.Secret{
				newTokenSecret("token-1", time.Now()),
				newTokenSecret("token-2", time.Now()),
			}, nil
		}).Times(1)

		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		secrets.EXPECT().Enqueue(tokenSecretNamespace, "token-1").Times(1)
		secrets.EXPECT().Enqueue(tokenSecretNamespace, "token-2").Times(1)

		h := &tokenExpiryHandler{
			secrets:     secrets,
			secretCache: secretCache,
		}

		setting := &v3.Setting{ObjectMeta: metav1.ObjectMeta{Name: settings.ExpireSCIMTokensAfter.Name}, Value: "1h"}
		got, err := h.onSettingChange("", setting)
		require.NoError(t, err)
		assert.Equal(t, setting, got)
	})

	t.Run("list error is returned", func(t *testing.T) {
		t.Parallel()

		listErr := errors.New("cache not ready")

		ctrl := gomock.NewController(t)
		secretCache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
		secretCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, listErr).Times(1)

		h := &tokenExpiryHandler{
			secrets:     fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl),
			secretCache: secretCache,
		}

		_, err := h.onSettingChange("", &v3.Setting{ObjectMeta: metav1.ObjectMeta{Name: settings.ExpireSCIMTokensAfter.Name}})
		require.ErrorIs(t, err, listErr)
	})
}

func TestRegisterTokenExpiry(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ctrl := gomock.NewController(t)

	secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
	secrets.EXPECT().Cache().Return(fake.NewMockCacheInterface[*corev1.Secret](ctrl)).AnyTimes()
	secrets.EXPECT().OnChange(ctx, "scim-token-expiry", gomock.Any()).Times(1)

	settingController := fake.NewMockNonNamespacedControllerInterface[*v3.Setting, *v3.SettingList](ctrl)
	settingController.EXPECT().OnChange(ctx, "scim-token-expiry-setting", gomock.Any()).Times(1)

	RegisterTokenExpiry(ctx, secrets, settingController)
}
