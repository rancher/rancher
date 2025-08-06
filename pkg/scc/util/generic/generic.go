package generic

import (
	"context"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
)

func CreateUpdatePatch[T generic.RuntimeMetaObject](incoming, desired T) ([]byte, error) {
	incomingJson, err := json.Marshal(incoming)
	if err != nil {
		return nil, err
	}
	newJson, err := json.Marshal(desired)
	if err != nil {
		return nil, err
	}

	return jsonpatch.CreateMergePatch(incomingJson, newJson)
}

func PreparePatchUpdated[T generic.RuntimeMetaObject](incoming, desired T) (T, error) {
	incomingJson, err := json.Marshal(incoming)
	if err != nil {
		return incoming, err
	}

	patch, err := CreateUpdatePatch(incoming, desired)
	if err != nil {
		return incoming, err
	}

	updatedJson, err := jsonpatch.MergePatch(incomingJson, patch)
	if err != nil {
		return incoming, err
	}

	var updatedObj T
	if err := json.Unmarshal(updatedJson, &updatedObj); err != nil {
		return incoming, err
	}

	return updatedObj, nil
}

func NamespaceScopedOnChange[T generic.RuntimeMetaObject](
	ctx context.Context,
	name, namespace string,
	c generic.ControllerMeta,
	sync generic.ObjectHandler[T],
) {
	condition := namespaceScopedCondition(namespace)
	onChangeHandler := generic.FromObjectHandlerToHandler(sync)
	c.AddGenericHandler(ctx, name, func(key string, obj runtime.Object) (runtime.Object, error) {
		if condition(obj) {
			return onChangeHandler(key, obj)
		}
		return obj, nil
	})
}

// TODO(wrangler/v4): revert to use OnRemove when it supports options (https://github.com/rancher/wrangler/pull/472).
func NamespaceScopedOnRemove[T generic.RuntimeMetaObject](
	ctx context.Context,
	name, namespace string,
	c generic.ControllerMeta,
	sync generic.ObjectHandler[T],
) {
	condition := namespaceScopedCondition(namespace)
	onRemoveHandler := generic.NewRemoveHandler(name, c.Updater(), generic.FromObjectHandlerToHandler(sync))
	c.AddGenericHandler(ctx, name, func(key string, obj runtime.Object) (runtime.Object, error) {
		if condition(obj) {
			return onRemoveHandler(key, obj)
		}
		return obj, nil
	})
}

func namespaceScopedCondition(namespace string) func(obj runtime.Object) bool {
	return func(obj runtime.Object) bool { return inExpectedNamespace(obj, namespace) }
}

func inExpectedNamespace(obj runtime.Object, namespace string) bool {
	metadata, err := meta.Accessor(obj)
	if err != nil {
		return false
	}

	return metadata.GetNamespace() == namespace
}
