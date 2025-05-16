package changes

import (
	"encoding/json"
	"fmt"

	jsonpatch "github.com/evanphx/json-patch"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

var accessor = meta.NewAccessor()

// CreateMergePatchChange creates and returns a PatchChange with a patch for the
// resource.
func CreateMergePatchChange(oldObject, newObject runtime.Object, mapper restMapper) (*PatchChange, error) {
	ref, err := resourceReferenceFromObject(oldObject, mapper)
	if err != nil {
		return nil, err
	}

	gvk := oldObject.GetObjectKind().GroupVersionKind()
	oldUnstructured, err := toUnstructured(oldObject, gvk)
	if err != nil {
		return nil, fmt.Errorf("failed to patch %s: failed to convert old object to Unstructured: %w", ref, err)
	}

	newUnstructured, err := toUnstructured(newObject, gvk)
	if err != nil {
		return nil, fmt.Errorf("failed to patch %s: failed to convert new object to Unstructured: %w", ref, err)
	}

	newJSON, err := json.Marshal(newUnstructured)
	if err != nil {
		return nil, fmt.Errorf("marshaling Unstructured: %w", err)
	}

	oldJSON, err := json.Marshal(oldUnstructured)
	if err != nil {
		return nil, fmt.Errorf("marshaling Unstructured: %w", err)
	}

	mergePatch, err := jsonpatch.CreateMergePatch(oldJSON, newJSON)
	if err != nil {
		// TODO
		return nil, fmt.Errorf("creating a MergePatch from resources: %w", err)
	}
	patchDiff := map[string]interface{}{}
	if err := json.Unmarshal(mergePatch, &patchDiff); err != nil {
		return nil, fmt.Errorf("failed to unmarshal patch data into a map: %w", err)
	}

	// Things to validate:
	//  * GVKs match
	//  * Key ObjectMeta fields match (Name, Namespace, UID?)

	return &PatchChange{
		ResourceRef: *ref,
		MergePatch:  patchDiff,
		Type:        MergePatchJSON,
	}, nil
}

func resourceReferenceFromObject(obj runtime.Object, mapper restMapper) (*ResourceReference, error) {
	name, err := accessor.Name(obj)
	if err != nil {
		return nil, fmt.Errorf("getting name from resource: %w", err)
	}
	namespace, err := accessor.Namespace(obj)
	if err != nil {
		return nil, fmt.Errorf("getting namespace from resource: %w", err)
	}

	objKind := obj.GetObjectKind().GroupVersionKind()
	if objKind.Empty() {
		return nil, fmt.Errorf("GVK missing from resource: %s", types.NamespacedName{Name: name, Namespace: namespace})
	}

	mapping, err := mapper.RESTMapping(objKind.GroupKind(), objKind.Version)
	if err != nil {
		return nil, fmt.Errorf("unable to get resource mapping for %s", objKind.GroupKind().WithVersion(objKind.Version))
	}

	return &ResourceReference{
		ObjectRef: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
		Resource: mapping.Resource.Resource,
		Version:  mapping.Resource.Version,
	}, nil
}

func toUnstructured(obj runtime.Object, gvk schema.GroupVersionKind) (*unstructured.Unstructured, error) {
	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{Object: raw}
	u.SetGroupVersionKind(gvk)

	return u, nil
}
