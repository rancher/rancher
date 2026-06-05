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
	"github.com/sirupsen/logrus"
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
	registryConfigs := map[string]rkev1.RegistryConfig{}
	registryMirrors := map[string]rkev1.Mirror{}

	if controlPlane.Spec.Registries != nil {
		registryConfigs = controlPlane.Spec.Registries.Configs
		registryMirrors = controlPlane.Spec.Registries.Mirrors
	}

	for registryName, config := range registryConfigs {
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
			// we need to re-encode the auth block for containerd to leverage it properly.
			// The secret cache automatically decodes data values for us to make them easier to work with,
			// but containerd will refuse to work with an unencoded auth block.
			auth := base64.StdEncoding.EncodeToString(secret.Data[rkev1.AuthAuthConfigSecretKey])
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
		// since provisioned clusters have a 1-to-1 mapping of hostnames to secrets (
		// due to the format of registries.yaml), it's not possible to specify more than
		// one set of credentials per hostname. We iterate across all pull secrets
		// until we find the first valid entry for the given registry host. If we don't find any,
		// it's assumed that the URL is unauthenticated, and registres.yaml is left empty.
		registry, _ := cluster.GetPrivateRegistry(nil)
		if registry != nil && len(registry.PullSecrets) > 0 {
			var foundCredentials bool
			for _, secretName := range registry.PullSecretNamesAsSlice() {
				secret, err := p.secretCache.Get(namespaces.System, secretName)
				if err != nil {
					logrus.Debugf("[planner] skipping global pull secret %q for registry %q: failed to fetch: %v", secretName, GSDR, err)
					continue
				}
				if secret.Type != corev1.SecretTypeDockerConfigJson {
					logrus.Debugf("[planner] skipping global pull secret %q: expected type %q, got %q", secretName, corev1.SecretTypeDockerConfigJson, secret.Type)
					continue
				}
				if secret.Data == nil {
					logrus.Debugf("[planner] skipping global pull secret %q: nil data", secretName)
					continue
				}
				username, password, auth, err := cluster.UnwrapDockerConfigJson(GSDR, secret.Data)
				if err != nil {
					logrus.Debugf("[planner] skipping global pull secret %q: does not contain credentials for registry %q: %v", secretName, GSDR, err)
					continue
				}
				if username == "" && password == "" && auth == "" && string(secret.Data[rkev1.IdentityTokenAuthConfigSecretKey]) == "" {
					logrus.Debugf("[planner] skipping global pull secret %q: no credentials found for registry %q", secretName, GSDR)
					continue
				}
				registryConfig := &registryConfig{}
				registryConfig.Auth = &authConfig{
					Username:      username,
					Password:      password,
					Auth:          auth,
					IdentityToken: string(secret.Data[rkev1.IdentityTokenAuthConfigSecretKey]),
				}
				configs[GSDR] = registryConfig
				foundCredentials = true
				break
			}
			if !foundCredentials {
				logrus.Warnf("[planner] no global pull secret contained credentials for registry %q. registries.yaml will be unpopulated.", GSDR)
			}
		}
	}

	data.registriesFileRaw, err = json.Marshal(map[string]interface{}{
		"mirrors": registryMirrors,
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
