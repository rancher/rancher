package cluster

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"k8s.io/client-go/rest"
)

const (
	rancherCredentialsFolder = "/cattle-credentials"
	urlFilename              = "url"
	tokenFilename            = "token"

	kubernetesServiceHostKey = "KUBERNETES_SERVICE_HOST"
	kubernetesServicePortKey = "KUBERNETES_SERVICE_PORT"
)

func TokenAndURL() (string, string, error) {
	url, err := readKey(urlFilename)
	if err != nil {
		return "", "", err
	}
	token, err := readKey(tokenFilename)
	return token, url, err
}

func Params() (map[string]interface{}, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	if err := populateCAData(cfg); err != nil {
		return nil, err
	}

	kubernetesServiceHost, err := getenv(kubernetesServiceHostKey)
	if err != nil {
		return nil, err
	}
	kubernetesServicePort, err := getenv(kubernetesServicePortKey)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"cluster": map[string]interface{}{
			"address": fmt.Sprintf("%s:%s", kubernetesServiceHost, kubernetesServicePort),
			"token":   cfg.BearerToken,
			"caCert":  base64.StdEncoding.EncodeToString(cfg.CAData),
		},
	}, nil
}

func getenv(env string) (string, error) {
	value := os.Getenv(env)
	if value == "" {
		return "", fmt.Errorf("%s is empty", env)
	}
	return value, nil
}

func populateCAData(cfg *rest.Config) error {
	bytes, err := ioutil.ReadFile(cfg.CAFile)
	if err != nil {
		return err
	}
	cfg.CAData = bytes
	return nil
}

func readKey(key string) (string, error) {
	bytes, err := ioutil.ReadFile(path.Join(rancherCredentialsFolder, key))
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
