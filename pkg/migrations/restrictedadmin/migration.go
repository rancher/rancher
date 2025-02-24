package restrictedadmin

import (
	"context"
	"errors"
	"fmt"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/migrations"
	"github.com/rancher/rancher/pkg/migrations/changes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func init() {
	migrations.Register(restrictedAdminMigration{})
}

type restrictedAdminMigration struct {
}

// Name implements the Migration interface.
func (t restrictedAdminMigration) Name() string {
	return "restricted-admin-migration"
}

// Changes implements the Migration interface.
//
// This migration finds current GlobalRoleBindings that reference the
// "restricted-admin" role and creates a new GlobalRoleBinding that references
// the new "restricted-admin-replacement" role with the same user.
func (t restrictedAdminMigration) Changes(ctx context.Context, client changes.Interface, opts migrations.MigrationOptions) (*migrations.MigrationChanges, error) {
	globalRoleBindings, err := client.Resource(schema.GroupVersionResource{
		Resource: "globalrolebindings",
		Group:    "management.cattle.io",
		Version:  "v3",
	}).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing globalrolebindings to calculate migration: %s", err)
	}

	var resourceChanges []changes.ResourceChange
	var migrationErr error
	for _, uns := range globalRoleBindings.Items {
		var grb v3.GlobalRoleBinding
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(uns.UnstructuredContent(), &grb)
		if err != nil {
			// TODO: improve this error
			migrationErr = errors.Join(migrationErr, err)
			continue
		}

		if grb.GlobalRoleName == "restricted-admin" {
			newGRB := newGlobalRoleBinding(grb.UserName, "restricted-admin-replacement")
			raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(newGRB)
			if err != nil {
				// TODO: improve this error
				migrationErr = errors.Join(migrationErr, err)
				continue
			}

			unsGRB := &unstructured.Unstructured{Object: raw}
			resourceChanges = append(resourceChanges,
				changes.ResourceChange{
					Operation: changes.OperationCreate,
					Create:    &changes.CreateChange{Resource: unsGRB},
				},
				changes.ResourceChange{
					Operation: changes.OperationDelete,
					Delete: &changes.DeleteChange{
						ResourceRef: changes.ResourceReference{
							ObjectRef: types.NamespacedName{
								Name: grb.GetName(),
							},
							Group:    "management.cattle.io",
							Resource: "globalrolebindings",
							Version:  "v3",
						},
					},
				},
			)
		}
	}

	if migrationErr != nil {
		return nil, migrationErr
	}

	return &migrations.MigrationChanges{Changes: []migrations.ChangeSet{resourceChanges}}, nil
}

func newGlobalRoleBinding(username, rolename string) *v3.GlobalRoleBinding {
	// TODO: This should add ownership?
	return &v3.GlobalRoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "GlobalRoleBinding",
			APIVersion: "management.cattle.io/v3",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "grb-",
		},
		GlobalRoleName: rolename,
		UserName:       username,
	}
}
