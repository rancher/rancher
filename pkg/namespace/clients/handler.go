package clients

import (
	"fmt"

	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/project"
	corev1 "k8s.io/api/core/v1"
)

func handler(ns *corev1.Namespace) {
	fmt.Printf("=== [pkg.namespace.mutator::handler 000] %s ===\n", ns.Name)

	if !namespace.GetMutator().Strict {
		fmt.Printf("=== [pkg.namespace.mutator::handler 001] %s ===\n", ns.Name)
		namespace.ApplyLabelsAndAnnotations(ns)
		return
	}

	if _, ok := ns.Annotations[project.ProjectIDAnnotation]; ok {
		fmt.Printf("=== [pkg.namespace.mutator::handler 002] %s ===\n", ns.Name)
		return
	}

	fmt.Printf("=== [pkg.namespace.mutator::handler 003] %s ===\n", ns.Name)
	namespace.ApplyLabelsAndAnnotations(ns)
}
