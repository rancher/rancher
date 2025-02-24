package example

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rancher/rancher/pkg/migrations"
	"github.com/rancher/rancher/pkg/migrations/changes"
	"github.com/rancher/rancher/pkg/migrations/test"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// batchedMigration is a simple example that supports the Continue mechanism.
type batchedMigration struct {
}

// Name implements the Migration interface.
func (t batchedMigration) Name() string {
	return "batched-migration"
}

// Changes implements the Migration interface.
//
// It counts up to 2.
func (t batchedMigration) Changes(ctx context.Context, client changes.Interface, opts migrations.MigrationOptions) (*migrations.MigrationChanges, error) {
	var migrationContinue struct {
		Start int64 `json:"start"`
	}
	if opts.Continue != "" {
		err := json.Unmarshal([]byte(opts.Continue), &migrationContinue)
		if err != nil {
			return nil, err
		}
	}

	svc := test.NewService(func(svc *corev1.Service) {
		svc.Name = fmt.Sprintf("test-%v", migrationContinue.Start)
	})

	migrationContinue.Start += 1
	newContinue, err := json.Marshal(migrationContinue)
	if err != nil {
		return nil, err
	}
	if migrationContinue.Start > 1 {
		newContinue = nil
	}

	uns, err := toUnstructured(svc)
	if err != nil {
		return nil, err
	}

	changes := []changes.ResourceChange{
		{
			Operation: changes.OperationCreate,
			Create: &changes.CreateChange{
				Resource: uns,
			},
		},
	}

	return &migrations.MigrationChanges{Continue: string(newContinue), Changes: changes}, nil
}

func toUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	return &unstructured.Unstructured{Object: raw}, nil
}
