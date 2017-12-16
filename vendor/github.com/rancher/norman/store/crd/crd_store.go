package crd

import (
	"context"
	"strings"
	"time"

	"github.com/rancher/norman/store/proxy"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/sirupsen/logrus"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type Store struct {
	apiExtClientSet apiextclientset.Interface
	k8sClient       rest.Interface
	schemaStores    map[string]*proxy.Store
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
		apiExtClientSet: apiExtClientSet,
		k8sClient:       k8sClient,
		schemaStores:    map[string]*proxy.Store{},
	}
}

func key(schema *types.Schema) string {
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

func (c *Store) List(apiContext *types.APIContext, schema *types.Schema, opt types.QueryOptions) ([]map[string]interface{}, error) {
	store, ok := c.schemaStores[key(schema)]
	if !ok {
		return nil, nil
	}
	return store.List(apiContext, schema, opt)
}

func (c *Store) Watch(apiContext *types.APIContext, schema *types.Schema, opt types.QueryOptions) (chan map[string]interface{}, error) {
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
		if schema.Store != nil || !schema.CanList() {
			continue
		}

		schema.Store = c
		allSchemas = append(allSchemas, schema)
	}

	ready, err := c.getReadyCRDs()
	if err != nil {
		return err
	}

	for _, schema := range allSchemas {
		crd, err := c.createCRD(schema, ready)
		if err != nil {
			return err
		}
		schemaStatus[schema] = crd
	}

	ready, err = c.getReadyCRDs()
	if err != nil {
		return err
	}

	for schema, crd := range schemaStatus {
		if readyCrd, ok := ready[crd.Name]; ok {
			schemaStatus[schema] = readyCrd
		} else {
			if err := c.waitCRD(ctx, crd.Name, schema, schemaStatus); err != nil {
				return err
			}
		}
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

func contains(list []string, s string) bool {
	for _, i := range list {
		if i == s {
			return true
		}
	}

	return false
}

func (c *Store) waitCRD(ctx context.Context, crdName string, schema *types.Schema, schemaStatus map[*types.Schema]*apiext.CustomResourceDefinition) error {
	logrus.Infof("Waiting for CRD %s to become available", crdName)
	defer logrus.Infof("Done waiting for CRD %s to become available", crdName)

	first := true
	return wait.Poll(500*time.Millisecond, 60*time.Second, func() (bool, error) {
		if !first {
			logrus.Infof("Waiting for CRD %s to become available", crdName)
		}
		first = false

		crd, err := c.apiExtClientSet.ApiextensionsV1beta1().CustomResourceDefinitions().Get(crdName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		for _, cond := range crd.Status.Conditions {
			switch cond.Type {
			case apiext.Established:
				if cond.Status == apiext.ConditionTrue {
					schemaStatus[schema] = crd
					return true, err
				}
			case apiext.NamesAccepted:
				if cond.Status == apiext.ConditionFalse {
					logrus.Infof("Name conflict on %s: %v\n", crdName, cond.Reason)
				}
			}
		}

		return false, ctx.Err()
	})
}

func (c *Store) createCRD(schema *types.Schema, ready map[string]*apiext.CustomResourceDefinition) (*apiext.CustomResourceDefinition, error) {
	plural := strings.ToLower(schema.PluralName)
	name := strings.ToLower(plural + "." + schema.Version.Group)

	crd, ok := ready[name]
	if ok {
		return crd, nil
	}

	crd = &apiext.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: apiext.CustomResourceDefinitionSpec{
			Group:   schema.Version.Group,
			Version: schema.Version.Version,
			Names: apiext.CustomResourceDefinitionNames{
				Plural: plural,
				Kind:   convert.Capitalize(schema.ID),
			},
		},
	}

	if schema.Scope == types.NamespaceScope {
		crd.Spec.Scope = apiext.NamespaceScoped
	} else {
		crd.Spec.Scope = apiext.ClusterScoped
	}

	logrus.Infof("Creating CRD %s", name)
	crd, err := c.apiExtClientSet.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd)
	if errors.IsAlreadyExists(err) {
		return crd, nil
	}
	return crd, err
}

func (c *Store) getReadyCRDs() (map[string]*apiext.CustomResourceDefinition, error) {
	list, err := c.apiExtClientSet.ApiextensionsV1beta1().CustomResourceDefinitions().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := map[string]*apiext.CustomResourceDefinition{}

	for i, crd := range list.Items {
		for _, cond := range crd.Status.Conditions {
			switch cond.Type {
			case apiext.Established:
				if cond.Status == apiext.ConditionTrue {
					result[crd.Name] = &list.Items[i]
				}
			}
		}
	}

	return result, nil
}
