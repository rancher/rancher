package app

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"

	"github.com/rancher/types/config"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/cert"
)

const (
	cattleSystemNamespace = "cattle-system"
	selfSignedSecretName  = "tls-rancher"
)

func addListenConfig(management *config.ManagementContext, cfg Config) error {
	userCACerts := cfg.ListenConfig.CACerts
	selfSigned := false
	existing, err := management.Management.ListenConfigs("").Get(cfg.ListenConfig.Name, v1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if apierrors.IsNotFound(err) {
		existing = nil
	}

	if existing != nil {
		if cfg.ListenConfig.Cert == "" {
			cfg.ListenConfig.Cert = existing.Cert
			cfg.ListenConfig.CACerts = existing.CACerts
			cfg.ListenConfig.Key = existing.Key
			cfg.ListenConfig.CAKey = existing.CAKey
			cfg.ListenConfig.CACert = existing.CACert
			cfg.ListenConfig.KnownIPs = existing.KnownIPs
		}
	}

	if (cfg.ListenConfig.Key == "" || cfg.ListenConfig.Cert == "") && cfg.ListenConfig.CACert == "" && cfg.ListenConfig.Mode != "acme" {
		caKey, err := cert.NewPrivateKey()
		if err != nil {
			return err
		}
		selfSigned = true
		caCert, err := cert.NewSelfSignedCACert(cert.Config{
			CommonName:   "cattle-ca",
			Organization: []string{"the-ranch"},
		}, caKey)
		if err != nil {
			return err
		}

		caCertBuffer := bytes.Buffer{}
		if err := pem.Encode(&caCertBuffer, &pem.Block{
			Type:  cert.CertificateBlockType,
			Bytes: caCert.Raw,
		}); err != nil {
			return err
		}

		caKeyBuffer := bytes.Buffer{}
		if err := pem.Encode(&caKeyBuffer, &pem.Block{
			Type:  cert.RSAPrivateKeyBlockType,
			Bytes: x509.MarshalPKCS1PrivateKey(caKey),
		}); err != nil {
			return err
		}

		cfg.ListenConfig.CACert = string(caCertBuffer.Bytes())
		cfg.ListenConfig.CACerts = cfg.ListenConfig.CACert
		cfg.ListenConfig.CAKey = string(caKeyBuffer.Bytes())
	}

	if cfg.NoCACerts || cfg.ListenConfig.Mode == "acme" {
		cfg.ListenConfig.CACerts = ""
	} else if userCACerts != "" {
		cfg.ListenConfig.CACerts = userCACerts
	}

	if existing == nil {
		if _, err := management.Management.ListenConfigs("").Create(cfg.ListenConfig); err != nil {
			return err
		}
	} else {
		cfg.ListenConfig.ResourceVersion = existing.ResourceVersion
		if _, err := management.Management.ListenConfigs("").Update(cfg.ListenConfig); err != nil {
			return err
		}
	}

	if !selfSigned {
		return nil
	}
	data := map[string]string{}
	data["tls.key"] = cfg.ListenConfig.CAKey
	data["tls.crt"] = cfg.ListenConfig.CACert
	secret := &corev1.Secret{
		StringData: data,
		Type:       corev1.SecretTypeTLS,
	}
	secret.Name = selfSignedSecretName
	secret.Namespace = cattleSystemNamespace
	if _, err := management.Core.Secrets(cattleSystemNamespace).Get("tls-rancher", v1.GetOptions{}); apierrors.IsNotFound(err) {
		_, err = management.Core.Secrets(cattleSystemNamespace).Create(secret)
		return err
	}
	_, err = management.Core.Secrets(cattleSystemNamespace).Update(secret)
	return err
}
