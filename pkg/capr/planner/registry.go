package planner

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	corev1 "k8s.io/api/core/v1"
)

// renderRegistries accepts an RKEControlPlane, namespace, and registry and generates the data needed to set up registries
func (p *Planner) renderRegistries(controlPlane *rkev1.RKEControlPlane) (registries, error) {
	var (
		files   []plan.File
		configs = map[string]interface{}{}
		data    registries
		err     error
	)

	for registryName, config := range controlPlane.Spec.Registries.Configs {
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
			if secret.Type != rkev1.AuthConfigSecretType && secret.Type != corev1.SecretTypeBasicAuth {
				return data, fmt.Errorf("secret [%s] must be of type [%s] or [%s]",
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

// toFile accepts the runtime, path, and a byte slice containing data to be written to a file on host. It returns a plan.File.
func toFile(controlPlane *rkev1.RKEControlPlane, path string, content []byte) plan.File {
	return plan.File{
		Content: base64.StdEncoding.EncodeToString(content),
		Path:    fmt.Sprintf("%s/etc/%s", capr.GetDataDir(controlPlane), path),
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
