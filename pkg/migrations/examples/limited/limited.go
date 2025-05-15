package limited

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/migrations"
	"github.com/rancher/rancher/pkg/migrations/changes"
	"github.com/rancher/rancher/pkg/migrations/test"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func init() {
	// migrations.Register(limitedMigration{})
}

type limitedMigration struct {
}

// Name implements the Migration interface.
func (t limitedMigration) Name() string {
	return "limited-migration"
}

// Changes implements the Migration interface.
func (t limitedMigration) Changes(ctx context.Context, client changes.Interface, opts migrations.MigrationOptions) (*migrations.MigrationChanges, error) {
	var limit int64 = 5
	if opts.Limit > 0 {
		limit = opts.Limit
	}

	var allChanges []changes.ResourceChange

	for i := range limit {
		svc := test.NewService(func(svc *corev1.Service) {
			svc.Name = fmt.Sprintf("test-%v", i)
		})
		uns, err := toUnstructured(svc)
		if err != nil {
			return nil, err
		}

		allChanges = append(allChanges, changes.ResourceChange{
			Operation: changes.OperationCreate,
			Create: &changes.CreateChange{
				Resource: uns,
			},
		})
	}

	// TODO: This could populate the continue.

	return &migrations.MigrationChanges{Changes: []migrations.ChangeSet{allChanges}}, nil
}

func toUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	return &unstructured.Unstructured{Object: raw}, nil
}
