package clients

import (
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/project"
	corev1 "k8s.io/api/core/v1"
)

func handler(ns *corev1.Namespace) {
	if !namespace.GetMutator().Strict {
		namespace.ApplyLabelsAndAnnotations(ns)
		return
	}

	if _, ok := ns.Annotations[project.ProjectIDAnnotation]; ok {
		return
	}

	namespace.ApplyLabelsAndAnnotations(ns)
}
