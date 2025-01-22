package descriptive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
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

// ApplyMetrics reports the number of changes during the process.
type ApplyMetrics struct {
	Create int64
	Delete int64
	Patch  int64
	Errors int64
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
	// Copy
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

// ApplyChanges applies a set of ResourceChanges to the cluster.
func ApplyChanges(ctx context.Context, client dynamic.Interface, changes []ResourceChange, options ApplyOptions, mapper meta.RESTMapper) (*ApplyMetrics, error) {
	var err error
	var metrics ApplyMetrics
	for _, change := range changes {
		switch change.Operation {
		case OperationPatch:
			metrics.Patch += 1
			if patchErr := applyPatchChangesToResource(ctx, client, *change.Patch); patchErr != nil {
				err = errors.Join(err, patchErr)
				metrics.Errors += 1
			}
		case OperationCreate:
			metrics.Create += 1
			if createErr := applyCreateChange(ctx, client, *change.Create, mapper); createErr != nil {
				err = errors.Join(err, createErr)
				metrics.Errors += 1
			}
		case OperationDelete:
			metrics.Delete += 1
			if deleteErr := applyDeleteChange(ctx, client, *change.Delete, options); err != nil {
				err = errors.Join(err, deleteErr)
				metrics.Errors += 1
			}
		default:
			err = errors.Join(err, fmt.Errorf("unknown operation: %s", change.Operation))
			metrics.Errors += 1
		}
	}

	return &metrics, err
}

func resourceName(u *unstructured.Unstructured) string {
	var elements []string
	if ns := u.GetNamespace(); ns != "" {
		elements = append(elements, ns)
	}

	elements = append(elements, u.GetName())

	return strings.Join(elements, "/")
}

func applyCreateChange(ctx context.Context, client dynamic.Interface, patch CreateChange, mapper meta.RESTMapper) error {
	createKind := patch.Resource.GetObjectKind().GroupVersionKind()
	if createKind.Empty() {
		return fmt.Errorf("GVK missing from resource: %s", resourceName(patch.Resource))
	}

	mapping, err := mapper.RESTMapping(createKind.GroupKind(), createKind.Version)
	if err != nil {
		return fmt.Errorf("unable to get resource mapping for %s", createKind.GroupKind().WithVersion(createKind.Version))
	}

	// TODO: What to do about non-namespaced resources?
	if _, err := client.Resource(mapping.Resource).Namespace(patch.Resource.GetNamespace()).Create(ctx, patch.Resource, metav1.CreateOptions{}); err != nil {
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

func applyPatchChangesToResource(ctx context.Context, client dynamic.Interface, patch PatchChange) error {
	resources := client.Resource(patch.ResourceRef.GVR()).Namespace(
		patch.ResourceRef.ObjectRef.Namespace)

	res, err := resources.Get(
		ctx, patch.ResourceRef.ObjectRef.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get resource for patching: %w", err)
	}

	res, err = applyPatchChanges(res, patch)
	if err != nil {
		return fmt.Errorf("failed to apply Patch change - applying patch: %w", err)
	}

	// TODO: PATCH!
	if _, err := resources.Update(ctx, res, metav1.UpdateOptions{}); err != nil {
		// TODO: Improve error
		return err
	}

	return nil
}

func applyPatchChanges(res *unstructured.Unstructured, patch PatchChange) (*unstructured.Unstructured, error) {
	objCopy := res.DeepCopy()
	var err error

	switch patch.Type {
	case PatchApplicationJSON:
		objCopy, err = applyJSONPatch(objCopy, patch.Operations)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown patch type: %s", patch.Type)
	}

	return objCopy, err
}

func applyJSONPatch(obj *unstructured.Unstructured, operations []PatchOperation) (*unstructured.Unstructured, error) {
	b, err := json.Marshal(operations)
	if err != nil {
		return nil, fmt.Errorf("failed to parse patch operations: %w", err)
	}

	patch, err := jsonpatch.DecodePatch([]byte(b))
	if err != nil {
		return nil, fmt.Errorf("decoding patch: %w", err)
	}

	return applyPatch(obj, func(b []byte) ([]byte, error) {
		return patch.Apply(b)
	})
}

type patchApplier func([]byte) ([]byte, error)

func applyPatch(obj *unstructured.Unstructured, f patchApplier) (*unstructured.Unstructured, error) {
	b, err := obj.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("marshalling resource to JSON for patching: %w", err)
	}

	patched, err := f(b)
	if err != nil {
		// TODO
		return nil, err
	}

	if err := obj.UnmarshalJSON(patched); err != nil {
		return nil, fmt.Errorf("unmarshalling resource to JSON after patching: %w", err)
	}

	return obj, nil
}
