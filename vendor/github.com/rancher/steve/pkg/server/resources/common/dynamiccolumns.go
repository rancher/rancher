package common

import (
	"net/http"

	"github.com/rancher/steve/pkg/attributes"
	"github.com/rancher/steve/pkg/schema/table"
	"github.com/rancher/steve/pkg/schemaserver/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
)

type DynamicColumns struct {
	client *rest.RESTClient
}

func NewDynamicColumns(config *rest.Config) (*DynamicColumns, error) {
	c, err := newClient(config)
	if err != nil {
		return nil, err
	}
	return &DynamicColumns{
		client: c,
	}, nil
}

func hasGet(methods []string) bool {
	for _, method := range methods {
		if method == http.MethodGet {
			return true
		}
	}
	return false
}

func (d *DynamicColumns) SetColumns(schema *types.APISchema) error {
	if attributes.Columns(schema) != nil {
		return nil
	}

	gvr := attributes.GVR(schema)
	if gvr.Resource == "" {
		return nil
	}
	nsed := attributes.Namespaced(schema)

	if !hasGet(schema.CollectionMethods) {
		return nil
	}

	r := d.client.Get()
	if gvr.Group == "" {
		r.Prefix("api")
	} else {
		r.Prefix("apis", gvr.Group)
	}
	r.Prefix(gvr.Version)
	if nsed {
		r.Prefix("namespaces", "default")
	}
	r.Prefix(gvr.Resource)

	obj, err := r.Do().Get()
	if err != nil {
		return err
	}
	t, ok := obj.(*metav1.Table)
	if !ok {
		return nil
	}

	var cols []table.Column
	for _, cd := range t.ColumnDefinitions {
		cols = append(cols, table.Column{
			Name:   cd.Name,
			Field:  "metadata.computed.fields." + cd.Name,
			Type:   cd.Type,
			Format: cd.Format,
		})
	}

	if len(cols) > 0 {
		attributes.SetColumns(schema, cols)
		schema.Attributes["server-side-column"] = "true"
	}

	return nil
}

func newClient(config *rest.Config) (*rest.RESTClient, error) {
	scheme := runtime.NewScheme()
	if err := metav1.AddMetaToScheme(scheme); err != nil {
		return nil, err
	}
	if err := metav1beta1.AddMetaToScheme(scheme); err != nil {
		return nil, err
	}

	config = rest.CopyConfig(config)
	config.UserAgent = rest.DefaultKubernetesUserAgent()
	config.AcceptContentTypes = "application/json;as=Table;v=v1beta1;g=meta.k8s.io"
	config.ContentType = "application/json;as=Table;v=v1beta1;g=meta.k8s.io"
	config.GroupVersion = &schema.GroupVersion{}
	config.NegotiatedSerializer = serializer.NewCodecFactory(scheme)
	config.APIPath = "/"
	return rest.RESTClientFor(config)
}
