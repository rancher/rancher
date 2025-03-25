package preferences

import (
	"context"
	"errors"
	"fmt"

	"github.com/rancher/rancher/pkg/migrations"
	"github.com/rancher/rancher/pkg/migrations/changes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func init() {
	// NOTE: This is not registered.
	// migrations.Register(preferencesMigration{})
}

type preferencesMigration struct {
}

// Name implements the Migration interface.
func (t preferencesMigration) Name() string {
	return "preferences-migration"
}

// Changes implements the Migration interface.
//
// This combines multiple `v3.Preference` resources into a single ConfigMap for
// each user.
//
// This would be a good example to use the MigrationOption.Continue and
// per-user ChangeSets.
func (t preferencesMigration) Changes(ctx context.Context, client changes.Interface, opts migrations.MigrationOptions) (*migrations.MigrationChanges, error) {
	userPreferences, err := client.Resource(schema.GroupVersionResource{
		Resource: "preferences",
		Group:    "management.cattle.io",
		Version:  "v3",
	}).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing preferences to calculate migration: %s", err)
	}

	combined, err := combinePreferences(userPreferences.Items)
	if err != nil {
		return nil, fmt.Errorf("combining preferences to calculate migration: %s", err)
	}

	var resourceChanges []changes.ResourceChange
	var migrationErr error
	for k, v := range combined {
		configMap := newConfigMap(k+"-preferences", k, v.data)
		raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(configMap)
		if err != nil {
			// TODO: improve this error
			migrationErr = errors.Join(migrationErr, err)
			continue
		}

		resourceChanges = append(resourceChanges,
			changes.ResourceChange{
				Operation: changes.OperationCreate,
				Create: &changes.CreateChange{
					Resource: &unstructured.Unstructured{Object: raw}},
			},
		)

		for _, preferenceName := range v.resources {
			resourceChanges = append(resourceChanges,
				changes.ResourceChange{
					Operation: changes.OperationDelete,
					Delete: &changes.DeleteChange{
						ResourceRef: changes.ResourceReference{
							ObjectRef: types.NamespacedName{
								Name:      preferenceName.Name,
								Namespace: preferenceName.Namespace,
							},
							Group:    "management.cattle.io",
							Resource: "preferences",
							Version:  "v3",
						},
					},
				})
		}
	}

	if migrationErr != nil {
		return nil, migrationErr
	}

	return &migrations.MigrationChanges{Changes: []migrations.ChangeSet{resourceChanges}}, nil
}

// Combine a set of preferences by user returning a map with the combined
// preferences.

type combinedPreferences struct {
	data      map[string]string
	resources []types.NamespacedName
}

func combinePreferences(preferences []unstructured.Unstructured) (map[string]*combinedPreferences, error) {
	combined := map[string]*combinedPreferences{}

	for _, preference := range preferences {
		username := preference.GetNamespace()

		current := combined[username]
		if current == nil {
			current = &combinedPreferences{data: map[string]string{}}
		}
		preferenceName := types.NamespacedName{Name: preference.GetName(), Namespace: preference.GetNamespace()}
		// These are to be removed.
		current.resources = append(current.resources, preferenceName)

		key := preference.GetName()
		value, found, err := unstructured.NestedString(preference.Object, "value")
		if err != nil {
			return nil, fmt.Errorf("invalid preference resource %s: %w", preferenceName, err)
		}
		if !found || value == "" {
			// If there's no value - skip
			continue
		}
		current.data[key] = value
		combined[username] = current
	}

	return combined, nil
}

func newConfigMap(username, namespace string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      username,
			Namespace: namespace,
		},
		Data: data,
	}
}
