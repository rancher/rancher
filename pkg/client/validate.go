package client

import (
	"encoding/base64"
	"net/url"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func Validate(api string, token string, cert string) error {
	u, err := url.Parse(api)
	if err != nil {
		return err
	}
	caBytes, err := base64.StdEncoding.DecodeString(cert)
	if err != nil {
		return err
	}
	config := &rest.Config{
		Host:        u.Host,
		Prefix:      u.Path,
		BearerToken: token,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: caBytes,
		},
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	_, err = clientset.Discovery().ServerVersion()
	return err
}
