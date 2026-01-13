package namespace

import (
	normancorev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	corev1 "k8s.io/api/core/v1"
)

func NormanInterface(i normancorev1.Interface) normancorev1.Interface {
	return &normanCoreInterface{
		Interface: i,
	}
}

type normanCoreInterface struct {
	normancorev1.Interface
}

func (i *normanCoreInterface) Namespaces(ns string) normancorev1.NamespaceInterface {
	return &wrapperNormanNamespace{
		i.Interface.Namespaces(ns),
	}
}

type wrapperNormanNamespace struct {
	normancorev1.NamespaceInterface
}

func (nn *wrapperNormanNamespace) Create(ns *corev1.Namespace) (*corev1.Namespace, error) {
	ApplyLabelsAndAnnotations(ns)
	return nn.NamespaceInterface.Create(ns)
}
