package cleanup

import (
	"errors"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/tokens"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	wranglerfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCleanupUnusedSecretTokens(t *testing.T) {
	secretStore := map[string]*v1.Secret{
		"cattle-system:test-secret-1": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret-1",
				Namespace: tokens.SecretNamespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"genericoidc": []byte("my user token"),
			},
		},
		"cattle-system:test-secret-2": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret-2",
				Namespace: tokens.SecretNamespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"cognito": []byte("my user token"),
			},
		},
	}
	authConfigStore := map[string]storedAuthConfig{
		"genericoidc": {authConfig: &v3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "genericoidc"}}, updated: true},
		"cognito":     {authConfig: &v3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "cognito"}}, updated: true},
	}
	ctrl := gomock.NewController(t)

	err := CleanupUnusedSecretTokens(getSecretControllerMock(ctrl, secretStore), getAuthConfigControllerMock(ctrl, authConfigStore))
	if err != nil {
		t.Fatal(err)
	}

	if len(secretStore) != 0 {
		t.Errorf("failed to delete secrets: %#v", secretStore)
	}

	for _, provider := range cleanupProviders {
		if ann := authConfigStore[provider].authConfig.Annotations; ann[cleanedUpSecretsAnnotation] != "true" {
			t.Errorf("didn't update the annotations: %#v", ann)
		}
	}
}

func TestCleanupUnusedSecretTokensHandlesErrors(t *testing.T) {
	secretStore := map[string]*v1.Secret{
		"cattle-system:test-secret-1": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret-1",
				Namespace: tokens.SecretNamespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"genericoidc": []byte("my user token"),
			},
		},
	}
	authConfigStore := map[string]storedAuthConfig{
		"cognito":     {authConfig: &v3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "cognito"}}, updated: false, err: errors.New("test error")},
		"genericoidc": {authConfig: &v3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "genericoidc"}}, updated: true},
	}
	ctrl := gomock.NewController(t)

	err := CleanupUnusedSecretTokens(getSecretControllerMock(ctrl, secretStore), getAuthConfigControllerMock(ctrl, authConfigStore))
	if msg := err.Error(); msg != "test error" {
		t.Fatalf("got error %v", err)
	}

	if len(secretStore) != 0 {
		t.Errorf("failed to delete secrets: %#v", secretStore)
	}

	for _, provider := range cleanupProviders {
		// Only the non-erroring configs should be updated
		if authConfigStore[provider].err == nil {
			if ann := authConfigStore[provider].authConfig.Annotations; ann[cleanedUpSecretsAnnotation] != "true" {
				t.Errorf("didn't update the annotations: %#v", ann)
			}
		}
	}
}

func TestCleanupUnusedSecretTokensAlreadyAnnotated(t *testing.T) {
	secretStore := map[string]*v1.Secret{
		"cattle-system:test-secret-1": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret-1",
				Namespace: tokens.SecretNamespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"genericoidc": []byte("my user token"),
			},
		},
	}
	authConfigStore := map[string]storedAuthConfig{
		"genericoidc": {
			authConfig: &v3.AuthConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "genericoidc",
					Annotations: map[string]string{cleanedUpSecretsAnnotation: "true"},
				},
			},
			updated: false,
		},
		"cognito": {authConfig: &v3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "cognito"}}, updated: true},
	}
	ctrl := gomock.NewController(t)

	err := CleanupUnusedSecretTokens(getSecretControllerMock(ctrl, secretStore), getAuthConfigControllerMock(ctrl, authConfigStore))
	if err != nil {
		t.Fatal(err)
	}

	if l := len(secretStore); l != 1 {
		t.Errorf("secrets were incorrectly deleted - remaining secrets = %d", l)
	}

	for _, provider := range cleanupProviders {
		if ann := authConfigStore[provider].authConfig.Annotations; ann[cleanedUpSecretsAnnotation] != "true" {
			t.Errorf("didn't update the annotations: %#v", ann)
		}
	}
}

type storedAuthConfig struct {
	authConfig *v3.AuthConfig
	updated    bool
	err        error
}

func getAuthConfigControllerMock(ctrl *gomock.Controller, store map[string]storedAuthConfig) mgmtv3.AuthConfigController {
	authConfigs := wranglerfake.NewMockNonNamespacedControllerInterface[*v3.AuthConfig, *v3.AuthConfigList](ctrl)
	authConfigsCache := fake.NewMockNonNamespacedCacheInterface[*v3.AuthConfig](ctrl)
	authConfigs.EXPECT().Cache().Return(authConfigsCache).Times(2)

	for _, v := range store {
		authConfigsCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.AuthConfig, error) {
			stored := store[name]
			if stored.err != nil {
				return nil, stored.err
			}
			return stored.authConfig, nil
		})

		if v.updated {
			authConfigs.EXPECT().Update(gomock.Any()).DoAndReturn(func(ac *v3.AuthConfig) (*v3.AuthConfig, error) {
				stored := store[ac.GetName()]
				stored.authConfig = ac
				store[ac.GetName()] = stored
				return ac, nil
			})
		}
	}

	return authConfigs
}
