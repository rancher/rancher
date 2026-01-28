package namespace

import (
	wranglercore "github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	wranglercorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
)

func NewWranglerFactory(f *wranglercore.Factory) *WranglerFactory {
	return &WranglerFactory{
		f,
	}
}

type WranglerFactory struct {
	*wranglercore.Factory
}

func (f *WranglerFactory) Core() wranglercore.Interface {
	return &wrapperWranglerCore{
		f.Factory.Core(),
	}
}

type wrapperWranglerCore struct {
	wranglercore.Interface
}

func (c *wrapperWranglerCore) V1() wranglercorev1.Interface {
	return &wrapperCoreV1{
		c.Interface.V1(),
	}
}

type wrapperCoreV1 struct {
	wranglercorev1.Interface
}

func (v *wrapperCoreV1) Namespace() wranglercorev1.NamespaceController {
	return &wrapperWranglerNamespace{
		v.Interface.Namespace(),
	}
}

type wrapperWranglerNamespace struct {
	wranglercorev1.NamespaceController
}

func (n *wrapperWranglerNamespace) Create(ns *corev1.Namespace) (*corev1.Namespace, error) {
	ApplyLabelsAndAnnotations(ns)
	return n.NamespaceController.Create(ns)
}
