package crd

import (
	"context"
	"strings"

	"github.com/rancher/norman/store/proxy"
	"github.com/rancher/norman/types"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type Store struct {
	Factory      *Factory
	k8sClient    rest.Interface
	schemaStores map[string]types.Store
}

func NewCRDStoreFromConfig(config rest.Config) (*Store, error) {
	dynamicConfig := config
	if dynamicConfig.NegotiatedSerializer == nil {
		configConfig := dynamic.ContentConfig()
		dynamicConfig.NegotiatedSerializer = configConfig.NegotiatedSerializer
	}

	k8sClient, err := rest.UnversionedRESTClientFor(&dynamicConfig)
	if err != nil {
		return nil, err
	}

	apiExtClient, err := clientset.NewForConfig(&dynamicConfig)
	if err != nil {
		return nil, err
	}

	return NewCRDStoreFromClients(apiExtClient, k8sClient), nil
}

func NewCRDStoreFromClients(apiExtClientSet apiextclientset.Interface, k8sClient rest.Interface) *Store {
	return &Store{
		Factory: &Factory{
			APIExtClientSet: apiExtClientSet,
		},
		k8sClient:    k8sClient,
		schemaStores: map[string]types.Store{},
	}
}

func key(schema *types.Schema) string {
	if !strings.EqualFold(schema.BaseType, schema.ID) {
		return schema.Version.Path + "/" + schema.BaseType
	}

	return schema.Version.Path + "/" + schema.ID
}

func (c *Store) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	store, ok := c.schemaStores[key(schema)]
	if !ok {
		return nil, nil
	}
	return store.ByID(apiContext, schema, id)
}

func (c *Store) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	store, ok := c.schemaStores[key(schema)]
	if !ok {
		return nil, nil
	}
	return store.Delete(apiContext, schema, id)
}

func (c *Store) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	store, ok := c.schemaStores[key(schema)]
	if !ok {
		return nil, nil
	}
	return store.List(apiContext, schema, opt)
}

func (c *Store) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	store, ok := c.schemaStores[key(schema)]
	if !ok {
		return nil, nil
	}
	return store.Watch(apiContext, schema, opt)
}

func (c *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	store, ok := c.schemaStores[key(schema)]
	if !ok {
		return nil, nil
	}
	return store.Update(apiContext, schema, data, id)
}

func (c *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	store, ok := c.schemaStores[key(schema)]
	if !ok {
		return nil, nil
	}
	return store.Create(apiContext, schema, data)
}

func (c *Store) AddSchemas(ctx context.Context, schemas ...*types.Schema) error {
	schemaStatus := map[*types.Schema]*apiext.CustomResourceDefinition{}
	var allSchemas []*types.Schema

	for _, schema := range schemas {
		if schema.Store != nil || !schema.CanList(nil) || !strings.EqualFold(schema.BaseType, schema.ID) {
			continue
		}

		schema.Store = c
		allSchemas = append(allSchemas, schema)
	}

	schemaStatus, err := c.Factory.AddSchemas(ctx, allSchemas...)
	if err != nil {
		return err
	}

	for schema, crd := range schemaStatus {
		c.schemaStores[key(schema)] = proxy.NewProxyStore(c.k8sClient,
			[]string{"apis"},
			crd.Spec.Group,
			crd.Spec.Version,
			crd.Status.AcceptedNames.Kind,
			crd.Status.AcceptedNames.Plural)
	}

	return nil
}
