package changes

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

const (
	// Patch Operation.
	OperationPatch string = "patch"

	// Create Operation.
	OperationCreate string = "create"

	// Delete Operation.
	OperationDelete string = "delete"

	// Content-Type for json-patch formatted Patches.
	PatchApplicationJSON string = "application/json-patch+json"
)

// ApplyOptions modifies application of resources.
type ApplyOptions struct {
	DryRun bool
}

// PatchOperation describes a JSON Patch operation.
type PatchOperation struct {
	Operation string `json:"op"`
	Path      string `json:"path"`
	Value     any    `json:"value"`
}

// PatchChange is a patch update to a resource.
type PatchChange struct {
	ResourceRef ResourceReference `json:"resourceRef"`
	Operations  []PatchOperation  `json:"operations"`
	Type        string            `json:"type"`
}

// CreateChange is resource creation operation.
type CreateChange struct {
	Resource *unstructured.Unstructured
}

// DeleteChange describes a resource to be deleted.
type DeleteChange struct {
	ResourceRef ResourceReference `json:"resourceRef"`
}

// ResourceReference refers to a resource to be updated.
type ResourceReference struct {
	ObjectRef types.NamespacedName `json:"objectRef"`
	Group     string               `json:"group"`
	Version   string               `json:"version"`
	Resource  string               `json:"resource"`
}

// GVR returns the GroupVersionResource for this reference.
func (r ResourceReference) GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    r.Group,
		Version:  r.Version,
		Resource: r.Resource,
	}
}

// ResourceChange is a change to be applied in-cluster.
type ResourceChange struct {
	Operation string `json:"op"`

	Patch  *PatchChange  `json:"patch,omitempty"`
	Create *CreateChange `json:"create,omitempty"`
	Delete *DeleteChange `json:"delete,omitempty"`
}

// Validate returns an error if the ResourceChange is not valid.
func (r *ResourceChange) Validate() error {
	switch r.Operation {
	case OperationCreate:
		if r.Create == nil {
			return errors.New("Create operation has no creation configuration")
		}
	case OperationPatch:
		if r.Patch == nil {
			return errors.New("Patch operation has no patch configuration")
		}
	}

	return nil
}

type restMapper interface {
	RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error)
}

// ApplyChanges applies a set of ResourceChanges to the cluster.
func ApplyChanges(ctx context.Context, client dynamic.Interface, changes []ResourceChange, options ApplyOptions, mapper restMapper) (*ApplyMetrics, error) {
	var metrics ApplyMetrics
	for _, change := range changes {
		switch change.Operation {
		case OperationPatch:
			metrics.Patch += 1
			if err := applyPatchChangesToResource(ctx, client, *change.Patch, options); err != nil {
				metrics.Errors += 1
				return &metrics, err
			}
		case OperationCreate:
			metrics.Create += 1
			if err := applyCreateChange(ctx, client, *change.Create, mapper, options); err != nil {
				metrics.Errors += 1
				return &metrics, err
			}
		case OperationDelete:
			metrics.Delete += 1
			if err := applyDeleteChange(ctx, client, *change.Delete, options); err != nil {
				metrics.Errors += 1
				return &metrics, err
			}
		default:
			metrics.Errors += 1
			return &metrics, fmt.Errorf("unknown operation: %q", change.Operation)
		}
	}

	return &metrics, nil
}

func resourceName(u *unstructured.Unstructured) string {
	var elements []string
	if ns := u.GetNamespace(); ns != "" {
		elements = append(elements, ns)
	}

	elements = append(elements, u.GetName())

	return strings.Join(elements, "/")
}

func applyCreateChange(ctx context.Context, client dynamic.Interface, patch CreateChange, mapper restMapper, options ApplyOptions) error {
	createKind := patch.Resource.GetObjectKind().GroupVersionKind()
	if createKind.Empty() {
		return fmt.Errorf("GVK missing from resource: %s", resourceName(patch.Resource))
	}

	mapping, err := mapper.RESTMapping(createKind.GroupKind(), createKind.Version)
	if err != nil {
		return fmt.Errorf("unable to get resource mapping for %s", createKind.GroupKind().WithVersion(createKind.Version))
	}

	createOptions := metav1.CreateOptions{}
	if options.DryRun {
		createOptions.DryRun = []string{metav1.DryRunAll}
	}

	// TODO: What to do about non-namespaced resources?
	if _, err := client.Resource(mapping.Resource).Namespace(patch.Resource.GetNamespace()).Create(ctx, patch.Resource, createOptions); err != nil {
		return fmt.Errorf("failed to apply Create change - creating resource: %w", err)
	}

	return nil
}

func applyDeleteChange(ctx context.Context, client dynamic.Interface, change DeleteChange, options ApplyOptions) error {
	resources := client.Resource(change.ResourceRef.GVR()).Namespace(
		change.ResourceRef.ObjectRef.Namespace)

	deleteOptions := metav1.DeleteOptions{}
	if options.DryRun {
		deleteOptions.DryRun = []string{metav1.DryRunAll}
	}

	err := resources.Delete(
		ctx, change.ResourceRef.ObjectRef.Name, deleteOptions)
	if err != nil {
		return fmt.Errorf("failed to delete resource: %w", err)
	}

	return nil
}

func applyPatchChangesToResource(ctx context.Context, client dynamic.Interface, patch PatchChange, options ApplyOptions) error {
	resources := client.Resource(patch.ResourceRef.GVR()).Namespace(
		patch.ResourceRef.ObjectRef.Namespace)

	res, err := resources.Get(
		ctx, patch.ResourceRef.ObjectRef.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get resource for patching: %w", err)
	}

	res, err = ApplyPatchChanges(res, patch)
	if err != nil {
		return fmt.Errorf("failed to apply Patch change - applying patch: %w", err)
	}

	updateOptions := metav1.UpdateOptions{}
	if options.DryRun {
		updateOptions.DryRun = []string{metav1.DryRunAll}
	}

	// TODO: PATCH!
	if _, err := resources.Update(ctx, res, updateOptions); err != nil {
		return fmt.Errorf("failed to apply update to resource %s %s: %w", res.GetKind(), nameFromUnstructured(res), err)
	}

	return nil
}
