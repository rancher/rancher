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
	return &wrapperClientsetCore{
		CoreV1Interface: c.Clientset.CoreV1(),
	}
}

type wrapperClientsetCore struct {
	k8scorev1.CoreV1Interface
}

func (w *wrapperClientsetCore) Namespaces() k8scorev1.NamespaceInterface {
	return &wrapperClientsetNamespace{
		NamespaceInterface: w.CoreV1Interface.Namespaces(),
	}
}

type wrapperClientsetNamespace struct {
	k8scorev1.NamespaceInterface
}

func (n *wrapperClientsetNamespace) Create(ctx context.Context, ns *corev1.Namespace, opts metav1.CreateOptions) (*corev1.Namespace, error) {
	ApplyLabelsAndAnnotations(ns)
	return n.NamespaceInterface.Create(ctx, ns, opts)
}
