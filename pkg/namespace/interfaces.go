package namespace

import (
	wranglercore "github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	wranglercorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
)

func NewFactory(f *wranglercore.Factory) *Factory {
	return &Factory{
		f,
	}
}

type Factory struct {
	*wranglercore.Factory
}

func (f *Factory) Core() wranglercore.Interface {
	return &core{
		f.Factory.Core(),
	}
}

type core struct {
	wranglercore.Interface
}

func (c *core) V1() wranglercorev1.Interface {
	return &v1{
		c.Interface.V1(),
	}
}

type v1 struct {
	wranglercorev1.Interface
}

func (v *v1) Namespace() wranglercorev1.NamespaceController {
	return &namespace{
		v.Interface.Namespace(),
	}
}

type namespace struct {
	wranglercorev1.NamespaceController
}

func (n *namespace) Create(ns *corev1.Namespace) (*corev1.Namespace, error) {
	ApplyLabelsAndAnnotations(ns)
	return n.NamespaceController.Create(ns)
}
