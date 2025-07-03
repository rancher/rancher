package stores

import (
	"fmt"
	"strings"
	"time"

	v1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

// EnsureNamespace tries to ensure that the namespace exists.
func EnsureNamespace(nsCache v1.NamespaceCache, nsClient v1.NamespaceClient, name string) error {
	var backoff = wait.Backoff{
		Duration: 100 * time.Millisecond,
		Factor:   2,
		Jitter:   .2,
		Steps:    7,
	}

	return wait.ExponentialBackoff(backoff, func() (bool, error) {
		_, err := nsCache.Get(name)
		if err == nil {
			return true, nil
		}

		if !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("error getting namespace %s: %w", name, err)
		}

		_, err = nsClient.Create(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return false, fmt.Errorf("error creating namespace %s: %w", name, err)
		}

		return true, nil
	})
}

// MapSpec holds the information for mapping a set of paths to different set of paths
type MapSpec map[string]MapEntry

// MapEntry holds the information to map a field path to a different field path.
type MapEntry struct {
	src fieldpath.Path
	dst fieldpath.Path // Note: path is slice, can be nil
}

// NewMapSpec converts a set of mapping entries into a proper MapSpec
func NewMapSpec(entries ...MapEntry) MapSpec {
	result := make(MapSpec)
	for _, entry := range entries {
		result[entry.src.String()] = entry
	}
	return result
}

// NewMap creates a new mapping entry from origin and destination paths
func NewMap(src fieldpath.Path, dst fieldpath.Path) MapEntry {
	return MapEntry{
		src: src,
		dst: dst,
	}
}

// MapPath maps a single path as per the set of mapping given. A `dst` of nil
// means that `src` is not mapped at all, the caller has to recognize and deal
// with that. A `src` for which there is no MapEntry is mapped to itself.
func MapPath(mapper MapSpec, path fieldpath.Path) fieldpath.Path {
	if entry, ok := mapper[path.String()]; ok {
		return entry.dst
	}
	return path
}

// MapInvert converts a map A -> B into a map B -> A.
func MapInvert(mapper MapSpec) MapSpec {
	entries := make([]MapEntry, len(mapper))

	for _, entry := range mapper {
		if entry.dst == nil {
			continue
		}
		entries = append(entries, NewMap(entry.dst, entry.src))
	}

	return NewMapSpec(entries...)
}

// MapManagedFields transform the originManagedFields under the control of the mapSpec.
func MapManagedFields(mapSpec MapSpec, originManagedFields []metav1.ManagedFieldsEntry) ([]metav1.ManagedFieldsEntry, error) {

	if originManagedFields == nil {
		return nil, nil
	}

	size := len(originManagedFields)
	if size == 0 {
		return nil, nil
	}

	destManagedFields := make([]metav1.ManagedFieldsEntry, size)

	for _, originEntry := range originManagedFields {
		originPaths := fieldpath.NewSet()
		err := originPaths.FromJSON(strings.NewReader(originEntry.FieldsV1.String()))
		if err != nil {
			return nil, err
		}

		destPaths := make([]fieldpath.Path, originPaths.Size())
		originPaths.Iterate(func(originPath fieldpath.Path) {
			destPath := MapPath(mapSpec, originPath)
			if destPath == nil {
				return
			}
			destPaths = append(destPaths, destPath)
		})

		destJSON, err := fieldpath.NewSet(destPaths...).ToJSON()
		if err != nil {
			return nil, err
		}

		destManagedFields = append(destManagedFields, metav1.ManagedFieldsEntry{
			Manager:    originEntry.Manager,
			Operation:  originEntry.Operation,
			Time:       originEntry.Time,
			FieldsType: originEntry.FieldsType,
			FieldsV1: &metav1.FieldsV1{
				Raw: destJSON,
			},
		})
	}

	return destManagedFields, nil
}
