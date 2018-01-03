package k8s

import (
	"bytes"
	"time"

	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	DefaultRetries      = 5
	DefaultSleepSeconds = 5
)

type k8sCall func(*kubernetes.Clientset, interface{}) error

func NewClient(kubeConfigPath string) (*kubernetes.Clientset, error) {
	// use the current admin kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, err
	}
	K8sClientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return K8sClientSet, nil
}

func decodeYamlResource(resource interface{}, yamlManifest string) error {
	decoder := yamlutil.NewYAMLToJSONDecoder(bytes.NewReader([]byte(yamlManifest)))
	return decoder.Decode(&resource)
}

func retryTo(runFunc k8sCall, k8sClient *kubernetes.Clientset, resource interface{}, retries, sleepSeconds int) error {
	var err error
	if retries == 0 {
		retries = DefaultRetries
	}
	if sleepSeconds == 0 {
		sleepSeconds = DefaultSleepSeconds
	}
	for i := 0; i < retries; i++ {
		if err = runFunc(k8sClient, resource); err != nil {
			time.Sleep(time.Second * time.Duration(sleepSeconds))
			continue
		}
		return nil
	}
	return err
}
