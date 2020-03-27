package common

import (
	"context"
	"fmt"

	"github.com/rancher/steve/pkg/attributes"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/wrangler/pkg/ratelimit"
	"k8s.io/apimachinery/pkg/apis/meta/internalversion"
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

type ColumnDefinition struct {
	metav1.TableColumnDefinition `json:",inline"`
	Field                        string `json:"field,omitempty"`
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

func (d *DynamicColumns) SetColumns(ctx context.Context, schema *types.APISchema) error {
	if attributes.Columns(schema) != nil {
		return nil
	}

	gvr := attributes.GVR(schema)
	if gvr.Resource == "" {
		return nil
	}

	r := d.client.Get()
	if gvr.Group == "" {
		r.Prefix("api")
	} else {
		r.Prefix("apis", gvr.Group)
	}
	r.Prefix(gvr.Version)
	r.Prefix(gvr.Resource)
	r.VersionedParams(&metav1.ListOptions{
		Limit: 1,
	}, metav1.ParameterCodec)

	obj, err := r.Do(ctx).Get()
	if err != nil {
		attributes.SetTable(schema, false)
		return nil
	}
	t, ok := obj.(*metav1.Table)
	if !ok {
		return nil
	}

	if len(t.ColumnDefinitions) > 0 {
		var cols []ColumnDefinition
		for i, colDef := range t.ColumnDefinitions {
			cols = append(cols, ColumnDefinition{
				TableColumnDefinition: colDef,
				Field:                 fmt.Sprintf("$.metadata.fields[%d]", i),
			})
		}
		attributes.SetColumns(schema, cols)
	}

	return nil
}

func newClient(config *rest.Config) (*rest.RESTClient, error) {
	scheme := runtime.NewScheme()
	if err := internalversion.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := metav1.AddMetaToScheme(scheme); err != nil {
		return nil, err
	}
	if err := metav1beta1.AddMetaToScheme(scheme); err != nil {
		return nil, err
	}

	config = rest.CopyConfig(config)
	config.RateLimiter = ratelimit.None
	config.UserAgent = rest.DefaultKubernetesUserAgent()
	config.AcceptContentTypes = "application/json;as=Table;v=v1;g=meta.k8s.io"
	config.GroupVersion = &schema.GroupVersion{}
	config.NegotiatedSerializer = serializer.NewCodecFactory(scheme)
	config.APIPath = "/"
	return rest.RESTClientFor(config)
}
