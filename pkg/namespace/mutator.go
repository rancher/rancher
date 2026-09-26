package namespace

import (
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const (
	AnnotationManagedNamespace      = "cattle.io/managed-namespace"
	AnnotationManagedNamespaceTrue  = "true"
	AnnotationManagedNamespaceFalse = "false"
)

var (
	// Currently mutator is only set once so no need for a mutex.
	mutator Mutator
)

// Mutator describes how rancher namespaces will be mutated at creation time.
type Mutator struct {
	Enabled     bool              `json:"enabled"`
	Strict      bool              `json:"strict"`
	Annotations map[string]string `json:"annotations"`
	Labels      map[string]string `json:"labels"`
}

// Mutate updates the given Namespace in-place and returns if any changes were made.
func (m *Mutator) mutate(ns *corev1.Namespace) bool {
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

func SetMutator(m Mutator) error {
	var offending []string
	for key := range m.Labels {
		if isRancherKey(key) {
			offending = append(offending, fmt.Sprintf("label %q", key))
		}
	}
	for key := range m.Annotations {
		if key != AnnotationManagedNamespace && isRancherKey(key) {
			offending = append(offending, fmt.Sprintf("annotation %q", key))
		}
	}
	if len(offending) > 0 {
		sort.Strings(offending)
		return fmt.Errorf("rancher namespace mutator cannot set Rancher labels or annotations: %s", strings.Join(offending, ", "))
	}

	mutator = m
	return nil
}

func isRancherKey(key string) bool {
	domain, _, ok := strings.Cut(key, "/")
	return ok && (domain == "rancher.io" || domain == "cattle.io" || strings.HasSuffix(domain, ".cattle.io"))
}

func GetMutator() Mutator {
	return mutator
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

// ApplyLabelsAndAnnotations updates the given Namespace in-place and retusn if any changes were made.
func ApplyLabelsAndAnnotations(ns *corev1.Namespace) bool {
	if !mutator.Enabled {
		return false
	}

	return mutator.mutate(ns)
}

// InjectValues injects Mutator information into a Helm Chart's values, allowing it to set the same Labels and Annotations to chart namespaces.
func InjectValues(values map[string]any) map[string]any {
	if !mutator.Enabled {
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
