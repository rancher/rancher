package changes

import (
	"encoding/json"
	"fmt"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// ApplyPatchChanges applies a PatchChange to an Unstructured and returns a copy
// of the Unstructured with the changes applied.
func ApplyPatchChanges(res *unstructured.Unstructured, patch PatchChange) (*unstructured.Unstructured, error) {
	objCopy := res.DeepCopy()
	var err error

	switch patch.Type {
	case PatchApplicationJSON:
		objCopy, err = applyJSONPatch(objCopy, patch.Operations)
		if err != nil {
			return nil, err
		}
	case MergePatchJSON:
		objCopy, err = applyMergePatch(objCopy, patch.MergePatch)
	default:
		return nil, fmt.Errorf("unknown patch type: %q", patch.Type)
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

func applyMergePatch(obj *unstructured.Unstructured, diff map[string]any) (*unstructured.Unstructured, error) {
	rawDiff, err := json.Marshal(diff)
	if err != nil {
		return nil, fmt.Errorf("marshaling MergePatch for application: %w", err)
	}

	return applyPatch(obj, func(b []byte) ([]byte, error) {
		return jsonpatch.MergePatch(b, rawDiff)
	})
}

type patchApplier func([]byte) ([]byte, error)

func applyPatch(obj *unstructured.Unstructured, f patchApplier) (*unstructured.Unstructured, error) {
	b, err := obj.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("marshalling resource to JSON for patching %s %s: %w", obj.GetKind(), nameFromUnstructured(obj), err)
	}

	patched, err := f(b)
	if err != nil {
		return nil, fmt.Errorf("applying patch to resource %s %s: %w", obj.GetKind(), nameFromUnstructured(obj), err)
	}

	if err := obj.UnmarshalJSON(patched); err != nil {
		return nil, fmt.Errorf("unmarshalling resource to JSON after patching %s %s: %w", obj.GetKind(), nameFromUnstructured(obj), err)
	}

	return obj, nil
}

func nameFromUnstructured(obj *unstructured.Unstructured) types.NamespacedName {
	return types.NamespacedName{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}
}
