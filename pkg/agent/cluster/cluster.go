package cluster

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/pkg/errors"
)

const (
	rancherCredentialsFolder = "/cattle-credentials"
	urlFilename              = "url"
	tokenFilename            = "token"
	namespaceFilename        = "namespace"

	kubernetesServiceHostKey = "KUBERNETES_SERVICE_HOST"
	kubernetesServicePortKey = "KUBERNETES_SERVICE_PORT"

	tokenFile  = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	rootCAFile = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

func Namespace() (string, error) {
	return readKey(namespaceFilename)
}

func TokenAndURL() (string, string, error) {
	url, err := readKey(urlFilename)
	if err != nil {
		return "", "", err
	}
	token, err := readKey(tokenFilename)
	return token, url, err
}

func Params() (map[string]interface{}, error) {
	caData, err := ioutil.ReadFile(rootCAFile)
	if err != nil {
		return nil, errors.Wrapf(err, "reading %s", rootCAFile)
	}

	token, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		return nil, errors.Wrapf(err, "reading %s", tokenFile)
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
			"token":   strings.TrimSpace(string(token)),
			"caCert":  base64.StdEncoding.EncodeToString(caData),
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

func readKey(key string) (string, error) {
	bytes, err := ioutil.ReadFile(path.Join(rancherCredentialsFolder, key))
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
