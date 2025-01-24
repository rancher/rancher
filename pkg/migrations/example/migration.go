package example

import (
	"context"

	"github.com/rancher/rancher/pkg/migrations"
	"github.com/rancher/rancher/pkg/migrations/changes"
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
func (t exampleMigration) Changes(ctx context.Context, client changes.Interface, opts migrations.MigrationOptions) (*migrations.MigrationChanges, error) {
	// TODO: Only return a change if the Service needs to be changed
	return &migrations.MigrationChanges{
		Changes: []changes.ResourceChange{
			{
				Operation: changes.OperationPatch,
				Patch: &changes.PatchChange{
					ResourceRef: changes.ResourceReference{
						ObjectRef: types.NamespacedName{
							Name:      "test-svc",
							Namespace: "default",
						},
						Resource: "services",
						Version:  "v1",
					},
					Operations: []changes.PatchOperation{
						{
							Operation: "replace",
							Path:      "/spec/ports/0/targetPort",
							Value:     9371,
						},
					},
					Type: changes.PatchApplicationJSON,
				},
			},
		},
	}, nil
}
