package namespace

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

const (
	AnnotationManagedNamespace     = "cattle.io/managed-namespace"
	AnnotationManagedNamespceTrue  = "true"
	AnnotationManagedNamespceFalse = "false"
)

var (
	labels      map[string]string
	annotations map[string]string
)

type provider struct {
	Enabled     bool              `json:"enabled"`
	Annotations map[string]string `json:"annotations"`
	Labels      map[string]string `json:"labels"`
}

func SetNamespaceOptions(s string) error {
	var p provider

	if err := json.Unmarshal([]byte(s), &p); err != nil {
		return fmt.Errorf("failed marshalling namespace options: %w", err)
	}

	if p.Enabled {
		annotations = p.Annotations
		annotations[AnnotationManagedNamespace] = AnnotationManagedNamespceTrue

		labels = p.Labels
	}

	return nil
}

// Copy copies all key/value pairs in src adding them to dst and returns whether the dst map was updated. If a key
// exist in both it will be overwritten in dst.
func copy[M1, M2 map[K]V, K comparable, V comparable](m1 M1, m2 M2) bool {
	var updated bool

	for k2, v2 := range m2 {
		if v1, ok := m1[k2]; !ok || v1 != v2 {
			updated = updated || true
		}

		m1[k2] = v2
	}

	return updated
}

// todo: how to handle overwriting existing labels and annotations
func ApplyLabelsAndAnnotations(ns *corev1.Namespace) bool {
	var updated bool

	if ns.Annotations == nil && len(annotations) > 0 {
		ns.Annotations = make(map[string]string, len(annotations))
	}
	updated = updated || copy(ns.Annotations, annotations)

	if ns.Labels == nil && len(labels) > 0 {
		ns.Labels = make(map[string]string, len(labels))
	}
	updated = updated || copy(ns.Labels, labels)

	return updated
}
