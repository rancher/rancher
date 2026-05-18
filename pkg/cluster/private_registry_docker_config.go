package cluster

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	v1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	kcorev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

var (
	ErrSecretDataNil            = errors.New("secret data is nil")
	ErrAuthKeyNotFound          = errors.New("secret 'rke.cattle.io/auth-config' does not have expected 'auth' key")
	ErrAuthMalformed            = errors.New("secret 'rke.cattle.io/auth-config' does not have 'auth' value in expected username:password format")
	ErrUsernameNotFound         = errors.New("secret 'kubernetes.io/basic-auth' has no 'username' field")
	ErrPasswordNotFound         = errors.New("secret 'kubernetes.io/basic-auth' has no 'password' field")
	ErrDockerConfigKeyNotFound  = errors.New("secret 'kubernetes.io/dockerconfigjson' has no '.dockerconfigjson' field")
	ErrUnsupportedSecretType    = errors.New("unsupported secret type")
	ErrDockerConfigJsonNotFound = errors.New(".dockerconfigjson not found in secret")
	ErrRegistryHostnameNotFound = errors.New("registry hostname not found in secret")
)

// ConvertToDockerConfigJson converts various types of secrets into a proper .dockerconfigjson format. Specifically, rke.cattle.io/auth-config, kubernetes.io/basic-auth,
// and kubernetes.io/dockerconfigjson secrets are supported. This is required as the Rancher UI may specify non-dockerconfigjson secrets on the management cluster.
func ConvertToDockerConfigJson(registryHost string, secret *kcorev1.Secret) ([]byte, error) {
	switch secret.Type {
	case v1.AuthConfigSecretType:
		if secret.Data == nil {
			return nil, fmt.Errorf("'rke.cattle.io/auth-config': %w", ErrSecretDataNil)
		}
		auth, ok := secret.Data["auth"]
		if !ok {
			return nil, ErrAuthKeyNotFound
		}
		username, password, found := strings.Cut(string(auth), ":")
		if !found {
			return nil, ErrAuthMalformed
		}
		return BuildDockerConfigJson(registryHost, username, password)
	case kcorev1.SecretTypeBasicAuth:
		if secret.Data == nil {
			return nil, fmt.Errorf("'kubernetes.io/basic-auth': %w", ErrSecretDataNil)
		}
		username, ok := secret.Data["username"]
		if !ok {
			return nil, ErrUsernameNotFound
		}
		password, ok := secret.Data["password"]
		if !ok {
			return nil, ErrPasswordNotFound
		}
		return BuildDockerConfigJson(registryHost, string(username), string(password))
	case kcorev1.SecretTypeDockerConfigJson:
		if secret.Data == nil {
			return nil, fmt.Errorf("'kubernetes.io/dockerconfigjson': %w", ErrSecretDataNil)
		}
		cfg, ok := secret.Data[kcorev1.DockerConfigJsonKey]
		if !ok {
			return nil, ErrDockerConfigKeyNotFound
		}
		return cfg, nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedSecretType, secret.Type)
	}
}

func BuildDockerConfigJson(registryHostname, username, password string) ([]byte, error) {
	authConfig := credentialprovider.DockerConfigJSON{
		Auths: credentialprovider.DockerConfig{
			registryHostname: credentialprovider.DockerConfigEntry{
				Username: username,
				Password: password,
			},
		},
	}
	return json.Marshal(authConfig)
}

// UnwrapDockerConfigJson takes secret data containing a .dockerconfigjson key and unwraps it, returning the username, password,
// and auth information for the specified hostname.
func UnwrapDockerConfigJson(registryHostname string, configJson map[string][]byte) (username string, password string, auth string, err error) {
	credJson, ok := configJson[kcorev1.DockerConfigJsonKey]
	if !ok {
		return "", "", "", ErrDockerConfigJsonNotFound
	}

	var cred credentialprovider.DockerConfigJSON
	err = json.Unmarshal(credJson, &cred)
	if err != nil {
		return "", "", "", err
	}

	entry, ok := cred.Auths[registryHostname]
	if !ok {
		return "", "", "", ErrRegistryHostnameNotFound
	}

	auth = fmt.Sprintf("%s:%s", entry.Username, entry.Password)
	return entry.Username, entry.Password, base64.StdEncoding.EncodeToString([]byte(auth)), nil
}
