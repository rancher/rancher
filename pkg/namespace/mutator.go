package namespace

import (
	corev1 "k8s.io/api/core/v1"
)

const (
	AnnotationManagedNamespace     = "cattle.io/managed-namespace"
	AnnotationManagedNamespceTrue  = "true"
	AnnotationManagedNamespceFalse = "false"
)

var (
	mutator *Mutator
)

type Mutator struct {
	Enabled     bool              `json:"enabled"`
	Annotations map[string]string `json:"annotations"`
	Labels      map[string]string `json:"labels"`
}

func (m *Mutator) Mutate(ns *corev1.Namespace) bool {
	if !m.Enabled {
		return false
	}

	var updated bool

	if len(ns.Annotations) == 0 {
		ns.Annotations = make(map[string]string, len(m.Annotations))
	}
	updated = copy(ns.Annotations, m.Annotations) || updated

	if len(ns.Labels) == 0 {
		ns.Labels = make(map[string]string, len(m.Labels))
	}
	updated = copy(ns.Labels, m.Labels) || updated

	return updated
}

func SetMutator(m *Mutator) {
	mutator = m
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

func ApplyLabelsAndAnnotations(ns *corev1.Namespace) bool {
	if mutator == nil {
		return false
	}

	return mutator.Mutate(ns)
}

func InjectValues(values map[string]any) map[string]any {
	if mutator == nil {
		return values
	}

	if values == nil {
		values = make(map[string]any, 1)
	}

	values["rancherNamespaces"] = map[string]any{
		"enabled":     mutator.Enabled,
		"annotations": mutator.Annotations,
		"labels":      mutator.Labels,
	}

	return values
}
