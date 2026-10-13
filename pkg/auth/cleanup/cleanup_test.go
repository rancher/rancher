package cleanup

import (
	"errors"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
				"test-eu-oidc": []byte("my user token"),
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
		"test-eu-oidc": {authConfig: &v3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "test-eu-oidc"}, Type: client.GenericOIDCConfigType}, updated: true},
		"cognito":      {authConfig: &v3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "cognito"}, Type: client.CognitoConfigType}, updated: true},
	}
	ctrl := gomock.NewController(t)

	err := CleanupUnusedSecretTokens(getSecretControllerMock(ctrl, secretStore), getAuthConfigControllerMock(ctrl, authConfigStore))
	if err != nil {
		t.Fatal(err)
	}

	if len(secretStore) != 0 {
		t.Errorf("failed to delete secrets: %#v", secretStore)
	}

	addAnnotationPatch := `{"metadata":{"annotations":{"auth.cattle.io/unused-secrets-cleaned":"true"}}}`
	for _, provider := range []string{"test-eu-oidc", "cognito"} {
		if patched := authConfigStore[provider].patched; patched != addAnnotationPatch {
			t.Errorf("didn't update the annotations for provider %s got %v", provider, patched)
		}
	}

}

func TestCleanupUnusedSecretTokensMultipleConfigsPerType(t *testing.T) {
	secretStore := map[string]*v1.Secret{
		"cattle-system:test-secret-1": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret-1",
				Namespace: tokens.SecretNamespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"genericoidc-1": []byte("my user token"),
			},
		},
		"cattle-system:test-secret-2": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret-2",
				Namespace: tokens.SecretNamespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"genericoidc-2": []byte("my user token"),
			},
		},
	}
	authConfigStore := map[string]storedAuthConfig{
		"genericoidc-1": {authConfig: &v3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "genericoidc-1"}, Type: client.GenericOIDCConfigType}, updated: true},
		"genericoidc-2": {authConfig: &v3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "genericoidc-2"}, Type: client.GenericOIDCConfigType}, updated: true},
		"other":         {authConfig: &v3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "other"}, Type: "somethingElse"}, updated: false},
	}
	ctrl := gomock.NewController(t)

	err := CleanupUnusedSecretTokens(getSecretControllerMock(ctrl, secretStore), getAuthConfigControllerMock(ctrl, authConfigStore))
	if err != nil {
		t.Fatal(err)
	}

	if len(secretStore) != 0 {
		t.Errorf("failed to delete secrets: %#v", secretStore)
	}

	addAnnotationPatch := `{"metadata":{"annotations":{"auth.cattle.io/unused-secrets-cleaned":"true"}}}`
	for _, name := range []string{"genericoidc-1", "genericoidc-2"} {
		if patched := authConfigStore[name].patched; patched != addAnnotationPatch {
			t.Errorf("didn't update the annotations for AuthConfig %s got %v", name, patched)
		}
	}

	if patched := authConfigStore["other"].patched; patched != "" {
		t.Errorf("patched an AuthConfig with an unrelated type: %v", patched)
	}
}

func TestCleanupUnusedSecretTokensMatchesAuthConfigTypes(t *testing.T) {
	// Literal values are used here to ensure that we match the Type stored on
	// AuthConfigs, and not the similarly named public provider types.
	secretStore := map[string]*v1.Secret{
		"cattle-system:test-secret-1": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret-1",
				Namespace: tokens.SecretNamespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"test-oidc":    []byte("my user token"),
				"test-cognito": []byte("my user token"),
			},
		},
	}
	authConfigStore := map[string]storedAuthConfig{
		"test-oidc":         {authConfig: &v3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "test-oidc"}, Type: "genericOIDCConfig"}, updated: true},
		"test-cognito":      {authConfig: &v3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "test-cognito"}, Type: "cognitoConfig"}, updated: true},
		"provider-oidc":     {authConfig: &v3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "provider-oidc"}, Type: "genericOIDCProvider"}, updated: false},
		"provider-cognito":  {authConfig: &v3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "provider-cognito"}, Type: "cognitoProvider"}, updated: false},
		"keycloak-oidc-cfg": {authConfig: &v3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "keycloak-oidc-cfg"}, Type: "keyCloakOIDCConfig"}, updated: false},
	}
	ctrl := gomock.NewController(t)

	err := CleanupUnusedSecretTokens(getSecretControllerMock(ctrl, secretStore), getAuthConfigControllerMock(ctrl, authConfigStore))
	if err != nil {
		t.Fatal(err)
	}

	if len(secretStore) != 0 {
		t.Errorf("failed to delete secrets: %#v", secretStore)
	}

	addAnnotationPatch := `{"metadata":{"annotations":{"auth.cattle.io/unused-secrets-cleaned":"true"}}}`
	for _, name := range []string{"test-oidc", "test-cognito"} {
		if patched := authConfigStore[name].patched; patched != addAnnotationPatch {
			t.Errorf("didn't update the annotations for AuthConfig %s got %v", name, patched)
		}
	}

	for _, name := range []string{"provider-oidc", "provider-cognito", "keycloak-oidc-cfg"} {
		if patched := authConfigStore[name].patched; patched != "" {
			t.Errorf("patched AuthConfig %s with an unrelated type: %v", name, patched)
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
		"cognito":     {authConfig: &v3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "cognito"}, Type: client.CognitoConfigType}, updated: false, err: errors.New("test error")},
		"genericoidc": {authConfig: &v3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "genericoidc"}, Type: client.GenericOIDCConfigType}, updated: true},
	}
	ctrl := gomock.NewController(t)

	err := CleanupUnusedSecretTokens(getSecretControllerMock(ctrl, secretStore), getAuthConfigControllerMock(ctrl, authConfigStore))
	if msg := err.Error(); msg != "test error" {
		t.Fatalf("got error %v", err)
	}

	if len(secretStore) != 0 {
		t.Errorf("failed to delete secrets: %#v", secretStore)
	}

	for _, provider := range []string{"genericoidc", "cognito"} {
		// Only the non-erroring configs should be updated
		addAnnotationPatch := `{"metadata":{"annotations":{"auth.cattle.io/unused-secrets-cleaned":"true"}}}`
		ap := authConfigStore[provider]
		if ap.err != nil {
			if ap.patched != "" {
				t.Errorf("patched the resource incorrectly: %s", provider)
			}
		} else {
			if ap.patched != addAnnotationPatch {
				t.Errorf("did not patch the resource correctly for %s: %s", provider, ap.patched)
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
				Type: client.GenericOIDCConfigType,
			},
			updated: false,
		},
		"cognito": {authConfig: &v3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "cognito"}, Type: client.CognitoConfigType}, updated: true},
	}
	ctrl := gomock.NewController(t)

	err := CleanupUnusedSecretTokens(getSecretControllerMock(ctrl, secretStore), getAuthConfigControllerMock(ctrl, authConfigStore))
	if err != nil {
		t.Fatal(err)
	}

	if l := len(secretStore); l != 1 {
		t.Errorf("secrets were incorrectly deleted - remaining secrets = %d", l)
	}

	for _, provider := range []string{"genericoidc", "cognito"} {
		addAnnotationPatch := `{"metadata":{"annotations":{"auth.cattle.io/unused-secrets-cleaned":"true"}}}`
		if patched := authConfigStore[provider].patched; authConfigStore[provider].updated && patched != addAnnotationPatch {
			t.Errorf("didn't update the annotations correctly for provider %s: %v", provider, patched)
		}
	}
}

type storedAuthConfig struct {
	authConfig *v3.AuthConfig
	updated    bool
	err        error

	patched string
}

func getAuthConfigControllerMock(ctrl *gomock.Controller, store map[string]storedAuthConfig) mgmtv3.AuthConfigController {
	authConfigs := fake.NewMockNonNamespacedControllerInterface[*v3.AuthConfig, *v3.AuthConfigList](ctrl)
	authConfigsCache := fake.NewMockNonNamespacedCacheInterface[*v3.AuthConfig](ctrl)
	authConfigs.EXPECT().Cache().Return(authConfigsCache).Times(1)

	configs := make([]*v3.AuthConfig, 0, len(store))
	for _, v := range store {
		configs = append(configs, v.authConfig)
	}
	authConfigsCache.EXPECT().List(gomock.Any()).Return(configs, nil)

	for _, v := range store {
		if !v.updated && v.err == nil {
			continue
		}

		authConfigs.EXPECT().Patch(v.authConfig.Name, types.MergePatchType, gomock.Any()).DoAndReturn(func(name string, _ types.PatchType, data []byte, _ ...any) (*v3.AuthConfig, error) {
			stored := store[name]
			if stored.err != nil {
				return nil, stored.err
			}
			stored.patched = string(data)
			store[name] = stored
			return nil, nil
		})
	}

	return authConfigs
}
