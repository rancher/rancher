package k8s

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

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
