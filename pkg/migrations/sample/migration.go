package sample

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/rancher/rancher/pkg/migrations"
	"github.com/rancher/rancher/pkg/migrations/changes"
)

func init() {
	migrations.Register(namespaceMigration{})
}

type namespaceMigration struct {
}

// Name implements the Migration interface.
func (t namespaceMigration) Name() string {
	return "namespace-migration"
}

// Changes implements the Migration interface.
func (t namespaceMigration) Changes(ctx context.Context, client changes.Interface, opts migrations.MigrationOptions) (*migrations.MigrationChanges, error) {
	namespaces, err := client.Resource(schema.GroupVersionResource{
		Resource: "namespaces",
		Version:  "v1",
	}).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing namespaces to calculate migration: %s", err)
	}

	var changes []changes.ResourceChange
	for _, uns := range namespaces.Items {
		var ns corev1.Namespace
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(uns.UnstructuredContent(), &ns)
		if err != nil {
			// TODO: should we do this? at least improve the message
			return nil, err
		}
		if ns.GetAnnotations()["lifecycle.cattle.io/create.namespace-auth"] != "" {
			changes = append(changes, patchForNS(ns))
		}
	}

	return &migrations.MigrationChanges{Changes: changes}, nil
}

func patchForNS(ns corev1.Namespace) changes.ResourceChange {
	return changes.ResourceChange{
		Operation: changes.OperationPatch,
		Patch: &changes.PatchChange{
			ResourceRef: changes.ResourceReference{
				ObjectRef: types.NamespacedName{
					Name: ns.Name,
				},
				Resource: "namespaces",
				Version:  "v1",
			},
			Operations: []changes.PatchOperation{
				{
					Operation: "add",
					Path:      "/metadata/labels/example.com~1migration",
					Value:     "migrated",
				},
			},
			Type: changes.PatchApplicationJSON,
		},
	}
}
