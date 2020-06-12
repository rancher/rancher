package helm

import (
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/attributes"
	"github.com/rancher/steve/pkg/schema/converter"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func DropHelmData(request *types.APIRequest, resource *types.RawResource) {
	data := resource.APIObject.Data()
	if data.String("metadata", "labels", "owner") == "helm" ||
		data.String("metadata", "labels", "OWNER") == "TILLER" {
		if data.String("data", "release") != "" {
			delete(data.Map("data"), "release")
		}
	}
}

func FormatRelease(request *types.APIRequest, resource *types.RawResource) {
	obj, ok := resource.APIObject.Object.(runtime.Object)
	if !ok {
		return
	}

	release, err := ToRelease(obj, SchemeBasedNamespaceLookup(request.Schemas))
	if err == ErrNotHelmRelease {
		return
	} else if err != nil {
		logrus.Errorf("failed to render helm release: %v", err)
		return
	}

	var (
		data      = resource.APIObject.Data()
		namespace = data.String("metadata", "namespace")
		name      = data.String("metadata", "name")
	)

	switch data.String("kind") {
	case "Secret":
		resource.ID = namespace + "/s:" + name
	case "ConfigMap":
		resource.ID = namespace + "/c:" + name
	}

	resource.Links["self"] = request.URLBuilder.ResourceLink(request.Schema, resource.ID)
	resource.APIObject.Object = release
}

func SchemeBasedNamespaceLookup(schemas *types.APISchemas) IsNamespaced {
	return func(gvk schema.GroupVersionKind) bool {
		schema := schemas.LookupSchema(converter.GVKToSchemaID(gvk))
		return schema != nil && attributes.Namespaced(schema)
	}
}
