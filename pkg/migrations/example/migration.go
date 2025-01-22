package example

import (
	"context"

	"github.com/rancher/rancher/pkg/migrations"
	"github.com/rancher/rancher/pkg/migrations/descriptive"
	"k8s.io/apimachinery/pkg/types"
)

var example exampleMigration

func init() {
	migrations.Register(example)
}

type exampleMigration struct {
}

// Name implements the Migration interface.
func (t exampleMigration) Name() string {
	return "example-migration"
}

// Changes implements the Migration interface.
func (t exampleMigration) Changes(ctx context.Context, client descriptive.Interface) ([]descriptive.ResourceChange, error) {
	// TODO: Only return a change if the Service needs to be changed
	return []descriptive.ResourceChange{
		{
			Operation: descriptive.OperationPatch,
			Patch: &descriptive.PatchChange{
				ResourceRef: descriptive.ResourceReference{
					ObjectRef: types.NamespacedName{
						Name:      "test-svc",
						Namespace: "default",
					},
					Resource: "services",
					Version:  "v1",
				},
				Operations: []descriptive.PatchOperation{
					{
						Operation: "replace",
						Path:      "/spec/ports/0/targetPort",
						Value:     9371,
					},
				},
				Type: descriptive.PatchApplicationJSON,
			},
		},
	}, nil
}
