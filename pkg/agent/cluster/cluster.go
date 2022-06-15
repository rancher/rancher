package cluster

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/serviceaccounttoken"
	"github.com/rancher/wrangler/pkg/kubeconfig"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	rancherCredentialsFolder = "/cattle-credentials"
	urlFilename              = "url"
	tokenFilename            = "token"
	namespaceFilename        = "namespace"

	kubernetesServiceHostKey = "KUBERNETES_SERVICE_HOST"
	kubernetesServicePortKey = "KUBERNETES_SERVICE_PORT"
)

func Namespace() (string, error) {
	ns, err := readKey(namespaceFilename)
	if os.IsNotExist(err) {
		return "", nil
	}
	return ns, err
}

func TokenAndURL() (string, string, error) {
	url, err := readKey(urlFilename)
	if err != nil {
		return "", "", err
	}
	token, err := readKey(tokenFilename)
	return token, url, err
}

func CAChecksum() string {
	return os.Getenv("CATTLE_CA_CHECKSUM")
}

func getTokenFromAPI() ([]byte, []byte, error) {
	cfg, err := kubeconfig.GetNonInteractiveClientConfig("").ClientConfig()
	if err != nil {
		return nil, nil, err
	}
	k8s, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, nil, err
	}
	sa, err := k8s.CoreV1().ServiceAccounts(namespace.System).Get(context.Background(), "cattle", metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find service account %s/%s: %w", namespace.System, "cattle", err)
	}
	secret, err := serviceaccounttoken.CreateSecretForServiceAccount(context.Background(), k8s, sa)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create secret for service account %s/%s: %w", namespace.System, "cattle", err)
	}
	cm, err := k8s.CoreV1().ConfigMaps(namespace.System).Get(context.Background(), "kube-root-ca.crt", metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find configmap containing the CA crt: %w", err)
	}
	return []byte(cm.Data["ca.crt"]), []byte(secret.Data[coreV1.ServiceAccountTokenKey]), nil
}

func Params() (map[string]interface{}, error) {
	caData, token, err := getTokenFromAPI()
	if err != nil {
		return nil, errors.Wrapf(err, "looking up %s/%s ca/token", namespace.System, "cattle")
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
