package app

import (
	"io/ioutil"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ReadTLSConfig(config *Config) error {
	var err error

	if config.ListenConfig != nil {
		return nil
	}

	lc := &v3.ListenConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ListenConfig",
			APIVersion: "management.cattle.io/v3",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cli-config",
		},
		Enabled: true,
	}

	lc.CACerts, err = readPEM("/etc/rancher/ssl/cacerts.pem")
	if err != nil {
		return err
	}

	lc.Key, err = readPEM("/etc/rancher/ssl/key.pem")
	if err != nil {
		return err
	}

	lc.Cert, err = readPEM("/etc/rancher/ssl/cert.pem")
	if err != nil {
		return err
	}

	lc.Mode = "https"
	if config.HTTPOnly {
		lc.Mode = "http"
	} else if len(config.ACMEDomains) > 0 {
		lc.Mode = "acme"
		lc.Domains = config.ACMEDomains
	}

	config.ListenConfig = lc

	return nil
}

func readPEM(path string) (string, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return "", nil
	}

	return string(content), nil
}
