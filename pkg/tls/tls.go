package tls

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	rancherCertFile    = "/etc/rancher/ssl/cert.pem"
	rancherKeyFile     = "/etc/rancher/ssl/key.pem"
	rancherCACertsFile = "/etc/rancher/ssl/cacerts.pem"
)

func ReadTLSConfig(acmeDomains []string, noCACerts bool) (*v3.ListenConfig, error) {
	var err error

	lc := &v3.ListenConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ListenConfig",
			APIVersion: "management.cattle.io/v3",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cli-config",
		},
		Enabled: true,
		Mode:    "https",
	}

	// ACME / Let's Encrypt
	// If --acme-domain is set, configure and return
	if len(acmeDomains) > 0 {
		lc.Mode = "acme"
		lc.Domains = acmeDomains
		return lc, nil
	}

	// Mounted certificates
	// If certificate file/key are set
	certFileExists := fileExists(rancherCertFile)
	keyFileExists := fileExists(rancherKeyFile)

	// If certificate file exists but not certificate key, or other way around, error out
	if (certFileExists && !keyFileExists) || (!certFileExists && keyFileExists) {
		return nil, fmt.Errorf("invalid SSL configuration found, please set both certificate file and certificate key file (one is missing)")
	}

	caFileExists := fileExists(rancherCACertsFile)

	// If certificate file and certificate key file exists, load files into listenConfig
	if certFileExists && keyFileExists {
		lc.Cert, err = readPEM(rancherCertFile)
		if err != nil {
			return nil, err
		}
		lc.Key, err = readPEM(rancherKeyFile)
		if err != nil {
			return nil, err
		}
		// Selfsigned needs cacerts, recognized CA needs --no-cacerts but can't be used together
		if (caFileExists && noCACerts) || (!caFileExists && !noCACerts) {
			return nil, fmt.Errorf("invalid SSL configuration found, please set cacerts when using self signed certificates or use --no-cacerts when using certificates from a recognized Certificate Authority, do not use both at the same time")
		}
		// Load cacerts if exists
		if caFileExists {
			lc.CACerts, err = readPEM(rancherCACertsFile)
			if err != nil {
				return nil, err
			}
		}
		return lc, nil
	}

	// External termination
	// We need to check if cacerts is passed or if --no-cacerts is used (when not providing certificate file and key)
	// If cacerts is passed
	if caFileExists {
		// We can't have --no-cacerts
		if noCACerts {
			return nil, fmt.Errorf("invalid SSL configuration found, please set cacerts when using self signed certificates or use --no-cacerts when using certificates from a recognized Certificate Authority, do not use both at the same time")
		}
		lc.CACerts, err = readPEM(rancherCACertsFile)
		if err != nil {
			return nil, err
		}
	}

	// No certificates mounted or only --no-cacerts used
	return lc, nil
}

func fileExists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func readPEM(path string) (string, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(content), nil
}
