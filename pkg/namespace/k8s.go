package namespace

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

func NewForConfig(config *rest.Config) (*Clientset, error) {
	k8s, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Clientset{
		Clientset: k8s,
	}, nil
}

type Clientset struct {
	*kubernetes.Clientset
}

func (c *Clientset) CoreV1() k8scorev1.CoreV1Interface {
	return &wrapperK8sCore{
		CoreV1Interface: c.Clientset.CoreV1(),
	}
}

type wrapperK8sCore struct {
	k8scorev1.CoreV1Interface
}

func (w *wrapperK8sCore) Namespaces() k8scorev1.NamespaceInterface {
	return &wrapperK8sNamespace{
		NamespaceInterface: w.CoreV1Interface.Namespaces(),
	}
}

type wrapperK8sNamespace struct {
	k8scorev1.NamespaceInterface
}

func (n *wrapperK8sNamespace) Create(ctx context.Context, ns *corev1.Namespace, opts metav1.CreateOptions) (*corev1.Namespace, error) {
	ApplyLabelsAndAnnotations(ns)
	return n.NamespaceInterface.Create(ctx, ns, opts)
}
