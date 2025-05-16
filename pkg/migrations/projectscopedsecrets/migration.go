package projectscopedsecrets

import (
	"context"
	"fmt"
	"strings"

	"github.com/rancher/rancher/pkg/migrations"
	"github.com/rancher/rancher/pkg/migrations/changes"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func init() {
	logrus.Info("Registering project scoped migration")
	migrations.Register(projectScopedSecretMigration{})
}

type projectScopedSecretMigration struct {
}

func (p projectScopedSecretMigration) Name() string {
	return "project-scoped-secret-migration"
}

func (p projectScopedSecretMigration) Changes(ctx context.Context, client changes.Interface, opts migrations.MigrationOptions) (*migrations.MigrationChanges, error) {
	logrus.Info("***Changes***")
	secrets, err := client.Resource(schema.GroupVersionResource{
		Resource: "secrets",
		Version:  "v1",
	}).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing secrets to calculate migration: %s", err)
	}

	var changes []changes.ResourceChange
	for _, s := range secrets.Items {
		var secret corev1.Secret
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(s.UnstructuredContent(), &secret)
		if err != nil {
			return nil, fmt.Errorf("could not convert %s to secret: %w", s, err)
		}

		if secret.GetLabels()["cattle.io/creator"] != "norman" {
			changes = append(changes, patchForSecret(secret))
		}
	}

	return &migrations.MigrationChanges{Changes: []migrations.ChangeSet{changes}}, nil
}

func patchForSecret(secret corev1.Secret) changes.ResourceChange {
	clusterName, projectName, found := strings.Cut(secret.GetAnnotations()["field.cattle.io/projectId"], ":")
	if !found {
		return changes.ResourceChange{}
	}
	lifecycleAnnotation := "lifecycle.cattle.io~1create.secretsController_" + clusterName
	//finalizer := "clusterscoped.controller.cattle.io~1secretsController_" + clusterName

	return changes.ResourceChange{
		Operation: changes.OperationPatch,
		Patch: &changes.PatchChange{
			ResourceRef: changes.ResourceReference{
				ObjectRef: types.NamespacedName{
					Name: secret.Name,
				},
				Resource: "secrets",
				Version:  "v1",
			},
			Operations: []changes.PatchOperation{
				{
					Operation: "add",
					Path:      "/metadata/annotations/management.cattle.io~1project-scoped-secret",
					Value:     projectName,
				},
				{
					Operation: "remove",
					Path:      "/metadata/labels/cattle.io~1creator",
				},
				{
					Operation: "remove",
					Path:      lifecycleAnnotation + clusterName,
				},
				{
					Operation: "remove",
					Path:      "/metadata/finalizers/0",
				},
			},
			Type: changes.PatchApplicationJSON,
		},
	}
}
