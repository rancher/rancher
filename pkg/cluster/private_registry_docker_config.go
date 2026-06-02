package cluster

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	v1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	kcorev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

const (
	ErrSecretDataNil            = "pull secret %q for registry %q has nil data"
	ErrAuthKeyNotFound          = "pull secret %q for registry %q of type 'rke.cattle.io/auth-config' is missing the 'auth' key"
	ErrAuthMalformed            = "pull secret %q for registry %q of type 'rke.cattle.io/auth-config' has malformed 'auth' value: expected username:password format"
	ErrUsernameNotFound         = "pull secret %q for registry %q of type 'kubernetes.io/basic-auth' is missing the 'username' field"
	ErrPasswordNotFound         = "pull secret %q for registry %q of type 'kubernetes.io/basic-auth' is missing the 'password' field"
	ErrDockerConfigKeyNotFound  = "pull secret %q for registry %q of type 'kubernetes.io/dockerconfigjson' is missing the '.dockerconfigjson' field"
	ErrUnsupportedSecretType    = "pull secret %q for registry %q has unsupported type %q"
	ErrDockerConfigJsonNotFound = "pull secret for registry %q is missing the '.dockerconfigjson' key"
	ErrRegistryHostnameNotFound = "registry hostname %q not found in pull secret"
)

// ConvertToDockerConfigJson converts various types of secrets into a proper .dockerconfigjson format. Specifically, rke.cattle.io/auth-config, kubernetes.io/basic-auth,
// and kubernetes.io/dockerconfigjson secrets are supported. This is required as the Rancher UI may specify non-dockerconfigjson secrets on the management cluster.
func ConvertToDockerConfigJson(registryHost string, secret *kcorev1.Secret) ([]byte, error) {
	if secret.Type == kcorev1.SecretTypeDockerConfigJson {
		if secret.Data == nil {
			return nil, fmt.Errorf(ErrSecretDataNil, secret.Name, registryHost)
		}
		cfg, ok := secret.Data[kcorev1.DockerConfigJsonKey]
		if !ok {
			return nil, fmt.Errorf(ErrDockerConfigKeyNotFound, secret.Name, registryHost)
		}
		return cfg, nil
	}

	username, password, err := ExtractUsernamePasswordFromPullSecret(registryHost, secret)
	if err != nil {
		return nil, err
	}

	return BuildDockerConfigJson(registryHost, username, password)
}

func ExtractUsernamePasswordFromPullSecret(registryHost string, secret *kcorev1.Secret) (string, string, error) {
	switch secret.Type {
	case v1.AuthConfigSecretType:
		if secret.Data == nil {
			return "", "", fmt.Errorf(ErrSecretDataNil, secret.Name, registryHost)
		}
		auth, ok := secret.Data["auth"]
		if !ok {
			return "", "", fmt.Errorf(ErrAuthKeyNotFound, secret.Name, registryHost)
		}
		username, password, found := strings.Cut(string(auth), ":")
		if !found {
			return "", "", fmt.Errorf(ErrAuthMalformed, secret.Name, registryHost)
		}
		return username, password, nil
	case kcorev1.SecretTypeBasicAuth:
		if secret.Data == nil {
			return "", "", fmt.Errorf(ErrSecretDataNil, secret.Name, registryHost)
		}
		username, ok := secret.Data["username"]
		if !ok {
			return "", "", fmt.Errorf(ErrUsernameNotFound, secret.Name, registryHost)
		}
		password, ok := secret.Data["password"]
		if !ok {
			return "", "", fmt.Errorf(ErrPasswordNotFound, secret.Name, registryHost)
		}
		return string(username), string(password), nil
	case kcorev1.SecretTypeDockerConfigJson:
		if secret.Data == nil {
			return "", "", fmt.Errorf(ErrSecretDataNil, secret.Name, registryHost)
		}
		username, password, _, err := UnwrapDockerConfigJson(registryHost, secret.Data)
		return username, password, err
	default:
		return "", "", fmt.Errorf(ErrUnsupportedSecretType, secret.Name, registryHost, secret.Type)
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
		return "", "", "", fmt.Errorf(ErrDockerConfigJsonNotFound, registryHostname)
	}

	var cred credentialprovider.DockerConfigJSON
	err = json.Unmarshal(credJson, &cred)
	if err != nil {
		return "", "", "", err
	}

	entry, ok := cred.Auths[registryHostname]
	if !ok {
		return "", "", "", fmt.Errorf(ErrRegistryHostnameNotFound, registryHostname)
	}

	auth = fmt.Sprintf("%s:%s", entry.Username, entry.Password)
	return entry.Username, entry.Password, base64.StdEncoding.EncodeToString([]byte(auth)), nil
}

// FilterDockerConfigJson extracts the specific Auth entry for the given registry hostname from a .dockerconfigjson
// secret and returns a new .dockerconfigjson containing only that entry.
func FilterDockerConfigJson(registryHostname string, configJson map[string][]byte) ([]byte, error) {
	credJson, ok := configJson[kcorev1.DockerConfigJsonKey]
	if !ok {
		return nil, fmt.Errorf(ErrDockerConfigJsonNotFound, registryHostname)
	}

	var cred credentialprovider.DockerConfigJSON
	err := json.Unmarshal(credJson, &cred)
	if err != nil {
		return nil, err
	}

	entry, ok := cred.Auths[registryHostname]
	if !ok {
		return nil, fmt.Errorf(ErrRegistryHostnameNotFound, registryHostname)
	}

	filtered := credentialprovider.DockerConfigJSON{
		Auths: credentialprovider.DockerConfig{
			registryHostname: entry,
		},
	}

	return json.Marshal(filtered)
}
