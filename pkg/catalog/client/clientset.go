package client

import (
	catalogv1 "github.com/rancher/types/apis/catalog.cattle.io/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type V1 struct {
	CoreClientV1    *kubernetes.Clientset
	CatalogClientV1 catalogv1.Interface
}

func NewClientSetV1(config string) (*V1, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", config)
	if err != nil {
		return nil, err
	}
	coreClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	catalogClient, err := catalogv1.NewForConfig(*cfg)
	if err != nil {
		return nil, err
	}

	clientSet := &V1{coreClient, catalogClient}
	return clientSet, nil
}
