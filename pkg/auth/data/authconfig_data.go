package data

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/rancher/rancher/pkg/auth/providers/azure"
	localprovider "github.com/rancher/rancher/pkg/auth/providers/local"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/controllers/management/auth"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// authConfigBaseFields are the top-level fields present on an AuthConfig that has never
// had any provider-specific data set on it (i.e. only the fields written by addAuthConfig,
// plus the fields Kubernetes and the AuthConfig controller always populate).
var authConfigBaseFields = []string{
	"apiVersion",
	"kind",
	"metadata",
	"type",
	"enabled",
	"logoutAllSupported",
	"status",
}

func AuthConfigs(authConfigs v3.AuthConfigInterface) error {
	if err := addAuthConfig(localprovider.Name, client.LocalConfigType, true, authConfigs.ObjectClient()); err != nil {
		return err
	}

	return deleteEmptyDisabledAuthConfigs(authConfigs, authConfigs.ObjectClient().UnstructuredClient())
}

type objectClient interface {
	Create(obj runtime.Object) (runtime.Object, error)
	Patch(name string, o runtime.Object, patchType types.PatchType, data []byte, subresources ...string) (runtime.Object, error)
}

// unstructuredLister is the subset of objectclient.GenericClient needed to list AuthConfigs
// with their raw, unstructured content so that provider-specific fields can be detected.
type unstructuredLister interface {
	List(opts v1.ListOptions) (runtime.Object, error)
}

// authConfigDeleter is the subset of v3.AuthConfigInterface needed to delete AuthConfigs.
type authConfigDeleter interface {
	Delete(name string, options *v1.DeleteOptions) error
}

// deleteEmptyDisabledAuthConfigs removes AuthConfig resources that are disabled and that
// have never been configured with any provider-specific data.
func deleteEmptyDisabledAuthConfigs(authConfigs authConfigDeleter, lister unstructuredLister) error {
	obj, err := lister.List(v1.ListOptions{})
	if err != nil {
		return err
	}

	list, ok := obj.(*unstructured.UnstructuredList)
	if !ok {
		return fmt.Errorf("unexpected type %T returned when listing AuthConfigs", obj)
	}

	for _, item := range list.Items {
		name := item.GetName()
		if name == localprovider.Name {
			continue
		}

		if enabled, _ := item.Object["enabled"].(bool); enabled {
			continue
		}

		if !isEmptyAuthConfig(item.Object) {
			continue
		}

		logrus.Debugf("Deleting AuthConfig %s", name)

		if err := authConfigs.Delete(name, &v1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

// isEmptyAuthConfig reports whether the given AuthConfig content has any field that isn't
// one of the base fields set at creation time, indicating a provider was configured.
func isEmptyAuthConfig(content map[string]any) bool {
	for key := range content {
		if !slices.Contains(authConfigBaseFields, key) {
			return false
		}
	}

	return true
}

func addAuthConfig(name, aType string, enabled bool, authConfigs objectClient) error {
	annotations := make(map[string]string)
	if name == azure.ProviderName {
		annotations[azure.GraphEndpointMigratedAnnotation] = "true"
	}
	annotations[auth.CleanupAnnotation] = auth.CleanupRancherLocked
	createdOrKnown, err := authConfigs.Create(&v3.AuthConfig{
		ObjectMeta: v1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
		},
		Type:               aType,
		Enabled:            enabled,
		LogoutAllSupported: false,
	})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}

		// Make sure the logoutAllSupported field is set correctly for the existing authConfig.
		// Use patch to avoid fetching the object first.
		patch, err := json.Marshal([]struct {
			Op    string `json:"op"`
			Path  string `json:"path"`
			Value any    `json:"value"`
		}{{
			Op:    "add",
			Path:  "/logoutAllSupported",
			Value: false,
		}})
		if err != nil {
			return err
		}

		_, err = authConfigs.Patch(name, createdOrKnown, types.JSONPatchType, patch)
		if err != nil {
			return err
		}
	}

	return nil
}
