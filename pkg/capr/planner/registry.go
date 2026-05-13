package planner

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/cluster"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/provisioningv2/image"
	"github.com/rancher/rancher/pkg/settings"
	corev1 "k8s.io/api/core/v1"
)

// renderRegistries accepts an RKEControlPlane and generates the data needed to set up registries.
func (p *Planner) renderRegistries(controlPlane *rkev1.RKEControlPlane) (registries, error) {
	var (
		files   []plan.File
		configs = map[string]interface{}{}
		data    registries
		err     error
	)

	GSDR := settings.SystemDefaultRegistry.Get()
	foundExistingGSDRConfiguration := false

	for registryName, config := range controlPlane.Spec.Registries.Configs {
		if registryName == GSDR {
			foundExistingGSDRConfiguration = true
		}
		registryConfig := &registryConfig{}
		if config.InsecureSkipVerify || config.TLSSecretName != "" || len(config.CABundle) > 0 {
			registryConfig.TLS = &tlsConfig{
				InsecureSkipVerify: config.InsecureSkipVerify,
			}
		}

		if config.TLSSecretName != "" {
			secret, err := p.secretCache.Get(controlPlane.Namespace, config.TLSSecretName)
			if err != nil {
				return data, err
			}
			if secret.Type != corev1.SecretTypeTLS {
				return data, fmt.Errorf("secret [%s] must be of type [%s]", config.TLSSecretName, corev1.SecretTypeTLS)
			}

			if cert := secret.Data[corev1.TLSCertKey]; len(cert) != 0 {
				file := toFile(controlPlane, fmt.Sprintf("tls/registries/%s/tls.crt", registryName), cert)
				registryConfig.TLS.CertFile = file.Path
				files = append(files, file)
			}

			if key := secret.Data[corev1.TLSPrivateKeyKey]; len(key) != 0 {
				file := toFile(controlPlane, fmt.Sprintf("tls/registries/%s/tls.key", registryName), key)
				registryConfig.TLS.KeyFile = file.Path
				files = append(files, file)
			}
		}

		if len(config.CABundle) > 0 {
			file := toFile(controlPlane, fmt.Sprintf("tls/registries/%s/ca.crt", registryName), config.CABundle)
			registryConfig.TLS.CAFile = file.Path
			files = append(files, file)
		}

		if config.AuthConfigSecretName != "" {
			secret, err := p.secretCache.Get(controlPlane.Namespace, config.AuthConfigSecretName)
			if err != nil {
				return data, err
			}

			if secret.Type != rkev1.AuthConfigSecretType && secret.Type != corev1.SecretTypeBasicAuth && secret.Type != corev1.SecretTypeDockerConfigJson {
				return data, fmt.Errorf("secret [%s] must be of type [%s] or [%s] or [%s]",
					config.AuthConfigSecretName, rkev1.AuthConfigSecretType, corev1.SecretTypeBasicAuth, corev1.SecretTypeDockerConfigJson)
			}

			if secret.Data == nil {
				return data, fmt.Errorf("secret [%s] has nil data", config.AuthConfigSecretName)
			}

			username := string(secret.Data[rkev1.UsernameAuthConfigSecretKey])
			password := string(secret.Data[rkev1.PasswordAuthConfigSecretKey])
			auth := string(secret.Data[rkev1.AuthAuthConfigSecretKey])
			identityToken := string(secret.Data[rkev1.IdentityTokenAuthConfigSecretKey])

			// need to pull out the username, password, auth, from the .dockerconfigjson key
			if secret.Type == corev1.SecretTypeDockerConfigJson {
				username, password, auth, err = cluster.UnwrapDockerConfigJson(registryName, secret.Data)
				if err != nil {
					return data, err
				}
			}

			registryConfig.Auth = &authConfig{
				Username:      username,
				Password:      password,
				Auth:          auth,
				IdentityToken: identityToken,
			}
		}

		configs[registryName] = registryConfig
	}

	// if a GSDR is defined and the user has not already configured
	// authentication for that hostname, we need to fall back to the global configuration.
	_, UsingGSDRDefault := image.GetPrivateRepoURLFromControlPlane(controlPlane)
	if GSDR != "" && UsingGSDRDefault && !foundExistingGSDRConfiguration {
		// we can only use the first pull secret set globally, since
		// provisioned clusters have a 1-to-1 mapping of hostnames to secrets.
		// Due to the format of registries.yaml, it's not possible to specify more than
		// one set of credentials per hostname.
		registry, _ := cluster.GetPrivateRegistry(nil)
		if registry != nil && len(registry.PullSecrets) > 0 {
			globalSecret := registry.PullSecretNamesAsSlice()[0]
			secret, err := p.secretCache.Get(namespaces.System, globalSecret)
			if err != nil {
				return data, err
			}

			if secret.Type != corev1.SecretTypeDockerConfigJson {
				return data, fmt.Errorf("global secret [%s] must be of type [%s]",
					globalSecret, corev1.SecretTypeDockerConfigJson)
			}

			if secret.Data == nil {
				return data, fmt.Errorf("global secret [%s] has nil data", globalSecret)
			}

			username, password, _, err := cluster.UnwrapDockerConfigJson(GSDR, secret.Data)
			if err != nil {
				return data, err
			}

			registryConfig := &registryConfig{}
			registryConfig.Auth = &authConfig{
				Username:      username,
				Password:      password,
				IdentityToken: string(secret.Data[rkev1.IdentityTokenAuthConfigSecretKey]),
			}

			configs[GSDR] = registryConfig
		}
	}

	data.registriesFileRaw, err = json.Marshal(map[string]interface{}{
		"mirrors": controlPlane.Spec.Registries.Mirrors,
		"configs": configs,
	})
	if err != nil {
		return data, err
	}

	// Sort the returned files slice because map iteration is not deterministic. This can lead to unexpected behavior where registry files are out of order.
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	data.certificateFiles = files
	return data, nil
}

// toFile accepts an RKEControlPlane, path, and a byte slice containing data to be written to a file on host. It returns
// a plan.File.
func toFile(controlPlane *rkev1.RKEControlPlane, path string, content []byte) plan.File {
	return plan.File{
		Content: base64.StdEncoding.EncodeToString(content),
		Path:    fmt.Sprintf("%s/etc/%s", capr.GetDistroDataDir(controlPlane), path),
	}
}

type registries struct {
	registriesFileRaw []byte
	certificateFiles  []plan.File
}

type registryConfig struct {
	Auth *authConfig `json:"auth"`
	TLS  *tlsConfig  `json:"tls"`
}

type tlsConfig struct {
	CAFile             string `json:"ca_file"`
	CertFile           string `json:"cert_file"`
	KeyFile            string `json:"key_file"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
}

type authConfig struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	Auth          string `json:"auth"`
	IdentityToken string `json:"identity_token"`
}
