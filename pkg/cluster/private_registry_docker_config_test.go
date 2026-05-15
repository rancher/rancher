package cluster

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	v1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

func TestConvertToDockerConfigJson(t *testing.T) {
	t.Parallel()

	const host = "registry.example.com"

	// buildExpectedDockerConfigJSON constructs the expected JSON output for a given username and password
	buildExpectedDockerConfigJSON := func(username, password string) []byte {
		raw, err := BuildDockerConfigJson(host, username, password)
		assert.NoError(t, err)
		return raw
	}

	tests := []struct {
		name        string
		secret      *corev1.Secret
		expectedErr error
		validate    func(t *testing.T, got []byte)
	}{
		{
			name: "rke auth-config: valid",
			secret: &corev1.Secret{
				Type: v1.AuthConfigSecretType,
				Data: map[string][]byte{"auth": []byte("myuser:mypass")},
			},
			validate: func(t *testing.T, got []byte) {
				assert.JSONEq(t, string(buildExpectedDockerConfigJSON("myuser", "mypass")), string(got))
			},
		},
		{
			name: "rke auth-config: missing auth key",
			secret: &corev1.Secret{
				Type: v1.AuthConfigSecretType,
				Data: map[string][]byte{},
			},
			expectedErr: ErrAuthKeyNotFound,
		},
		{
			name: "rke auth-config: malformed auth value (no colon delimiter)",
			secret: &corev1.Secret{
				Type: v1.AuthConfigSecretType,
				Data: map[string][]byte{"auth": []byte("myusermypass")},
			},
			expectedErr: ErrAuthMalformed,
		},
		{
			name: "basic-auth: valid",
			secret: &corev1.Secret{
				Type: corev1.SecretTypeBasicAuth,
				Data: map[string][]byte{
					"username": []byte("myuser"),
					"password": []byte("mypass"),
				},
			},
			validate: func(t *testing.T, got []byte) {
				assert.JSONEq(t, string(buildExpectedDockerConfigJSON("myuser", "mypass")), string(got))
			},
		},
		{
			name: "basic-auth: missing username",
			secret: &corev1.Secret{
				Type: corev1.SecretTypeBasicAuth,
				Data: map[string][]byte{"password": []byte("mypass")},
			},
			expectedErr: ErrUsernameNotFound,
		},
		{
			name: "basic-auth: missing password",
			secret: &corev1.Secret{
				Type: corev1.SecretTypeBasicAuth,
				Data: map[string][]byte{"username": []byte("myuser")},
			},
			expectedErr: ErrPasswordNotFound,
		},
		{
			name: "dockerconfigjson: valid passthrough",
			secret: &corev1.Secret{
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					corev1.DockerConfigJsonKey: buildExpectedDockerConfigJSON("myuser", "mypass"),
				},
			},
			validate: func(t *testing.T, got []byte) {
				assert.JSONEq(t, string(buildExpectedDockerConfigJSON("myuser", "mypass")), string(got))
			},
		},
		{
			name: "dockerconfigjson: missing key",
			secret: &corev1.Secret{
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{},
			},
			expectedErr: ErrDockerConfigKeyNotFound,
		},
		{
			name: "unsupported secret type",
			secret: &corev1.Secret{
				Type: "some.other/type",
				Data: map[string][]byte{},
			},
			expectedErr: ErrUnsupportedSecretType,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ConvertToDockerConfigJson(host, tt.secret)
			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			tt.validate(t, got)
		})
	}
}

func TestBuildDockerConfigJson(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		host     string
		username string
		password string
	}{
		{
			name:     "standard credentials",
			host:     "registry.example.com",
			username: "myuser",
			password: "mypass",
		},
		{
			name:     "empty credentials still produces valid JSON",
			host:     "registry.example.com",
			username: "",
			password: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := BuildDockerConfigJson(tt.host, tt.username, tt.password)
			require.NoError(t, err)

			var parsed credentialprovider.DockerConfigJSON
			require.NoError(t, json.Unmarshal(got, &parsed))

			entry, ok := parsed.Auths[tt.host]
			require.True(t, ok, "expected auths entry for host %q", tt.host)
			assert.Equal(t, tt.username, entry.Username)
			assert.Equal(t, tt.password, entry.Password)
		})
	}
}

func TestUnwrapDockerConfigJson(t *testing.T) {
	t.Parallel()

	const host = "registry.example.com"

	makeConfigJSON := func(host, username, password string) map[string][]byte {
		raw, _ := json.Marshal(credentialprovider.DockerConfigJSON{
			Auths: credentialprovider.DockerConfig{
				host: credentialprovider.DockerConfigEntry{
					Username: username,
					Password: password,
				},
			},
		})
		return map[string][]byte{corev1.DockerConfigJsonKey: raw}
	}

	tests := []struct {
		name             string
		host             string
		data             map[string][]byte
		expectedUsername string
		expectedPassword string
		expectedAuth     string
		expectedErr      error
		// errContains is used instead of expectedErr when the error format may vary (e.g. stdlib errors).
		errContains string
	}{
		{
			name:             "valid config with matching hostname",
			host:             host,
			data:             makeConfigJSON(host, "myuser", "mypass"),
			expectedUsername: "myuser",
			expectedPassword: "mypass",
			expectedAuth:     base64.StdEncoding.EncodeToString([]byte("myuser:mypass")),
		},
		{
			name:        "missing .dockerconfigjson key",
			host:        host,
			data:        map[string][]byte{},
			expectedErr: ErrDockerConfigJsonNotFound,
		},
		{
			name:        "invalid JSON",
			host:        host,
			data:        map[string][]byte{corev1.DockerConfigJsonKey: []byte("not-json")},
			errContains: "invalid character",
		},
		{
			name:        "hostname not found in auths",
			host:        "other.registry.example.com",
			data:        makeConfigJSON(host, "myuser", "mypass"),
			expectedErr: ErrRegistryHostnameNotFound,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			username, password, auth, err := UnwrapDockerConfigJson(tt.host, tt.data)
			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
				return
			}
			if tt.errContains != "" {
				assert.ErrorContains(t, err, tt.errContains)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedUsername, username)
			assert.Equal(t, tt.expectedPassword, password)
			assert.Equal(t, tt.expectedAuth, auth)
		})
	}
}
