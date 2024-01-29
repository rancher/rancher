package authprovisioningv2

import (
	"strings"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/nodetemplate"
	"github.com/rancher/rancher/pkg/controllers/management/rbac"
	"github.com/rancher/wrangler/v2/pkg/data"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func validMachineConfigGVK(gvk schema.GroupVersionKind) bool {
	return gvk.Group == "rke-machine-config.cattle.io" &&
		gvk.Version == "v1" &&
		strings.HasSuffix(gvk.Kind, "Config")
}

func (h *handler) OnMachineConfigChange(obj runtime.Object) (runtime.Object, error) {
	if obj == nil {
		return nil, nil
	}

	objMeta, err := meta.Accessor(obj)
	if err != nil || !objMeta.GetDeletionTimestamp().IsZero() {
		return nil, err
	}

	// if owner bindings annotation is present, the node template is in the proper namespace and has had
	// its creator rolebindings created
	annotations := objMeta.GetAnnotations()
	if annotations != nil && annotations[nodetemplate.OwnerBindingsAnno] == "true" {
		return obj, nil
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	creatorID, ok := annotations[rbac.CreatorIDAnn]
	if !ok {
		// If the creatorID annotation is not present, then the roles and rolebindings cannot be created.
		// We don't error here because there could be existing machine configs without this annotation.
		return obj, nil
	}

	apiVersion, kind := gvk.ToAPIVersionAndKind()
	resourceName := strings.ToLower(kind)
	if strings.HasSuffix(resourceName, "config") {
		resourceName += "s"
	}
	// Create Role and RBs if they do not exist
	if err := rbac.CreateRoleAndRoleBinding(resourceName, kind, objMeta.GetName(), objMeta.GetNamespace(), apiVersion, creatorID, []string{gvk.Group},
		objMeta.GetUID(),
		[]v32.Member{}, h.mgmtCtx); err != nil {
		return nil, err
	}

	dynamicMachineConfig, err := h.dynamic.Get(gvk, objMeta.GetNamespace(), objMeta.GetName())
	if err != nil {
		return nil, err
	}

	objData, err := data.Convert(dynamicMachineConfig.DeepCopyObject())
	if err != nil {
		return nil, err
	}

	anns := objData.Map("metadata", "annotations")
	// owner bindings annotation is meant to prevent bindings from being created again if they have been removed from creator
	anns[nodetemplate.OwnerBindingsAnno] = "true"
	objData.SetNested(anns, "metadata", "annotations")

	if _, err = h.dynamic.Update(&unstructured.Unstructured{Object: objData}); err != nil {
		return nil, err
	}

	return h.dynamic.Get(gvk, objMeta.GetNamespace(), objMeta.GetName())
}
