package pipelineexecution

import (
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math"
	"math/big"
	"time"

	"github.com/rancher/rancher/pkg/controllers/managementagent/nslabels"
	v3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rke/pki/cert"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

func getProjectID(ns *corev1.Namespace) string {
	if ns.Annotations != nil {
		return ns.Annotations[nslabels.ProjectIDFieldLabel]
	}
	return ""
}

func getSigningDuration(lister v3.PipelineSettingLister, projectID string) time.Duration {
	defaultDuration, _ := time.ParseDuration(utils.SettingSigningDurationDefault)
	setting, err := lister.Get(projectID, utils.SettingSigningDuration)
	if err != nil {
		logrus.Warn(err)
		return defaultDuration
	}
	duration, err := time.ParseDuration(setting.Value)
	if err != nil {
		logrus.Warn(err)
		return defaultDuration
	}
	return duration
}

// newSignedCert creates a signed certificate using the given CA certificate and key
func newSignedCertWithDuration(cfg cert.Config, duration time.Duration, key *rsa.PrivateKey, caCert *x509.Certificate, caKey *rsa.PrivateKey) (*x509.Certificate, error) {
	serial, err := cryptorand.Int(cryptorand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	if err != nil {
		return nil, err
	}
	if len(cfg.CommonName) == 0 {
		return nil, errors.New("must specify a CommonName")
	}
	if len(cfg.Usages) == 0 {
		return nil, errors.New("must specify at least one ExtKeyUsage")
	}

	certTmpl := x509.Certificate{
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: cfg.Organization,
		},
		DNSNames:     cfg.AltNames.DNSNames,
		IPAddresses:  cfg.AltNames.IPs,
		SerialNumber: serial,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(duration), //.UTC(),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  cfg.Usages,
	}
	certDERBytes, err := x509.CreateCertificate(cryptorand.Reader, &certTmpl, caCert, key.Public(), caKey)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(certDERBytes)
}
