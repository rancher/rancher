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

// MapSpec holds the information for mapping a set of field paths, specified by
// their string rep, to different set of paths
type MapSpec map[string]fieldpath.Path

// MapManagedFields transform the originManagedFields under the control of the mapSpec.
func MapManagedFields(mapSpec MapSpec, originManagedFields []metav1.ManagedFieldsEntry) ([]metav1.ManagedFieldsEntry, error) {

	size := len(originManagedFields)
	if size == 0 {
		return nil, nil
	}

	destManagedFields := make([]metav1.ManagedFieldsEntry, 0, size)

	for _, originEntry := range originManagedFields {
		if originEntry.FieldsV1 == nil {
			destManagedFields = append(destManagedFields, originEntry)
			continue
		}

		originPaths := fieldpath.NewSet()
		err := originPaths.FromJSON(strings.NewReader(originEntry.FieldsV1.String()))
		if err != nil {
			return nil, err
		}

		destPaths := make([]fieldpath.Path, 0, originPaths.Size())

		originPaths.Iterate(func(originPath fieldpath.Path) {
			if destPath, ok := mapSpec[originPath.String()]; ok {
				if destPath == nil {
					return
				}
				destPaths = append(destPaths, destPath)
				return
			}
			destPaths = append(destPaths, originPath)
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
