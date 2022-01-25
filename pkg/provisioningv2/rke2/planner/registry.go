package planner

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	corev1 "k8s.io/api/core/v1"
)

func (p *Planner) addRegistryConfig(config map[string]interface{}, controlPlane *rkev1.RKEControlPlane) ([]plan.File, error) {
	registry := controlPlane.Spec.Registries
	if registry == nil {
		return nil, nil
	}

	registryConfig, files, err := p.toRegistryConfig(rke2.GetRuntime(controlPlane.Spec.KubernetesVersion), controlPlane.Namespace, registry)
	if err != nil {
		return nil, err
	}

	config["private-registry"] = string(registryConfig)
	return files, nil
}

func (p *Planner) toRegistryConfig(runtime, namespace string, registry *rkev1.Registry) ([]byte, []plan.File, error) {
	var (
		files   []plan.File
		configs = map[string]interface{}{}
	)

	for registryName, config := range registry.Configs {
		registryConfig := &registryConfig{}
		if config.InsecureSkipVerify || config.TLSSecretName != "" || len(config.CABundle) > 0 {
			registryConfig.TLS = &tlsConfig{
				InsecureSkipVerify: config.InsecureSkipVerify,
			}
		}

		if config.TLSSecretName != "" {
			secret, err := p.secretCache.Get(namespace, config.TLSSecretName)
			if err != nil {
				return nil, nil, err
			}
			if secret.Type != corev1.SecretTypeTLS {
				return nil, nil, fmt.Errorf("secret [%s] must be of type [%s]", config.TLSSecretName, corev1.SecretTypeTLS)
			}

			if cert := secret.Data[corev1.TLSCertKey]; len(cert) != 0 {
				file := toFile(runtime, fmt.Sprintf("tls/registries/%s/tls.crt", registryName), cert)
				registryConfig.TLS.CertFile = file.Path
				files = append(files, file)
			}

			if key := secret.Data[corev1.TLSPrivateKeyKey]; len(key) != 0 {
				file := toFile(runtime, fmt.Sprintf("tls/registries/%s/tls.key", registryName), key)
				registryConfig.TLS.KeyFile = file.Path
				files = append(files, file)
			}
		}

		if len(config.CABundle) > 0 {
			file := toFile(runtime, fmt.Sprintf("tls/registries/%s/ca.crt", registryName), config.CABundle)
			registryConfig.TLS.CAFile = file.Path
			files = append(files, file)
		}

		if config.AuthConfigSecretName != "" {
			secret, err := p.secretCache.Get(namespace, config.AuthConfigSecretName)
			if err != nil {
				return nil, nil, err
			}
			if secret.Type != rkev1.AuthConfigSecretType && secret.Type != corev1.SecretTypeBasicAuth {
				return nil, nil, fmt.Errorf("secret [%s] must be of type [%s] or [%s]",
					config.AuthConfigSecretName, rkev1.AuthConfigSecretType, corev1.SecretTypeBasicAuth)
			}
			registryConfig.Auth = &authConfig{
				Username:      string(secret.Data[rkev1.UsernameAuthConfigSecretKey]),
				Password:      string(secret.Data[rkev1.PasswordAuthConfigSecretKey]),
				Auth:          string(secret.Data[rkev1.AuthAuthConfigSecretKey]),
				IdentityToken: string(secret.Data[rkev1.IdentityTokenAuthConfigSecretKey]),
			}
		}

		configs[registryName] = registryConfig
	}

	data, err := json.Marshal(map[string]interface{}{
		"mirrors": registry.Mirrors,
		"configs": configs,
	})
	if err != nil {
		return nil, nil, err
	}

	return data, files, nil
}

func toFile(runtime, path string, content []byte) plan.File {
	return plan.File{
		Content: base64.StdEncoding.EncodeToString(content),
		Path:    fmt.Sprintf("/var/lib/rancher/%s/etc/%s", runtime, path),
	}
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
