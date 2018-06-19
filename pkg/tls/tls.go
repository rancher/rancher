package tls

import (
	"fmt"
	"io/ioutil"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ReadTLSConfig(acmeDomains []string) (*v3.ListenConfig, error) {
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
	}

	lc.CACerts, err = readPEM("/etc/rancher/ssl/cacerts.pem")
	if err != nil {
		return nil, err
	}

	lc.Key, err = readPEM("/etc/rancher/ssl/key.pem")
	if err != nil {
		return nil, err
	}

	lc.Cert, err = readPEM("/etc/rancher/ssl/cert.pem")
	if err != nil {
		return nil, err
	}

	lc.Mode = "https"
	if len(acmeDomains) > 0 {
		lc.Mode = "acme"
		lc.Domains = acmeDomains
	}

	valid := false
	if lc.Key != "" && lc.Cert != "" {
		valid = true
	} else if lc.Key == "" && lc.Cert == "" {
		valid = true
	}

	if !valid {
		return nil, fmt.Errorf("invalid SSL configuration found, please set cert/key, cert/key/cacerts, cacerts only, or none")
	}

	return lc, nil
}

func readPEM(path string) (string, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return "", nil
	}

	return string(content), nil
}
