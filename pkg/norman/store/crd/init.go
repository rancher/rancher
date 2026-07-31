package crd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rancher/norman/store/proxy"
	"github.com/rancher/norman/types"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
)

type Factory struct {
	eg           errgroup.Group
	ClientGetter proxy.ClientGetter
}

func NewFactoryFromClientGetter(clientGetter proxy.ClientGetter) *Factory {
	return &Factory{
		ClientGetter: clientGetter,
	}
}

func NewFactoryFromClient(config rest.Config) (*Factory, error) {
	getter, err := proxy.NewClientGetterFromConfig(config)
	if err != nil {
		return nil, err
	}

	return &Factory{
		ClientGetter: getter,
	}, nil
}

func (f *Factory) BatchWait() error {
	return f.eg.Wait()
}

func (f *Factory) BatchCreateCRDs(ctx context.Context, storageContext types.StorageContext, typer proxy.StoreTyper, schemas *types.Schemas, version *types.APIVersion, schemaIDs ...string) {
	f.eg.Go(func() error {
		var schemasToCreate []*types.Schema

		for _, schemaID := range schemaIDs {
			s := schemas.Schema(version, schemaID)
			if s == nil {
				return fmt.Errorf("can not find schema %s", schemaID)
			}
			schemasToCreate = append(schemasToCreate, s)
		}

		err := f.AssignStores(ctx, storageContext, typer, schemasToCreate...)
		if err != nil {
			return fmt.Errorf("creating CRD store %v", err)
		}

		return nil
	})
}

func (f *Factory) AssignStores(ctx context.Context, storageContext types.StorageContext, typer proxy.StoreTyper, schemas ...*types.Schema) error {
	schemaStatus, err := f.CreateCRDs(ctx, storageContext, schemas...)
	if err != nil {
		return err
	}

	for _, schema := range schemas {
		crd, ok := schemaStatus[schema]
		if !ok {
			return fmt.Errorf("failed to create create/find CRD for %s", schema.ID)
		}

		schema.Store = proxy.NewProxyStore(ctx, f.ClientGetter,
			storageContext,
			typer,
			[]string{"apis"},
			crd.Spec.Group,
			// Even if CRD is created as v1beta1, it's served as v1 with a single element in Versions
			crd.Spec.Versions[0].Name,
			crd.Status.AcceptedNames.Kind,
			crd.Status.AcceptedNames.Plural)
	}

	return nil
}

func (f *Factory) CreateCRDs(ctx context.Context, storageContext types.StorageContext, schemas ...*types.Schema) (map[*types.Schema]*apiext.CustomResourceDefinition, error) {
	schemaStatus := map[*types.Schema]*apiext.CustomResourceDefinition{}

	apiClient, err := f.ClientGetter.APIExtClient(nil, storageContext)
	if err != nil {
		return nil, err
	}

	ready, err := f.getReadyCRDs(ctx, apiClient)
	if err != nil {
		return nil, err
	}

	for _, schema := range schemas {
		crd, err := f.createCRD(ctx, apiClient, schema, ready)
		if err != nil {
			return nil, err
		}
		schemaStatus[schema] = crd
	}

	ready, err = f.getReadyCRDs(ctx, apiClient)
	if err != nil {
		return nil, err
	}

	for schema, crd := range schemaStatus {
		if readyCrd, ok := ready[crd.Name]; ok {
			schemaStatus[schema] = readyCrd
		} else {
			if err := f.waitCRD(ctx, apiClient, crd.Name, schema, schemaStatus); err != nil {
				return nil, err
			}
		}
	}

	return schemaStatus, nil
}

func (f *Factory) waitCRD(ctx context.Context, apiClient clientset.Interface, crdName string, schema *types.Schema, schemaStatus map[*types.Schema]*apiext.CustomResourceDefinition) error {
	logrus.Infof("Waiting for CRD %s to become available", crdName)
	defer logrus.Infof("Done waiting for CRD %s to become available", crdName)

	first := true
	return wait.PollUntilContextTimeout(ctx, 500*time.Millisecond, 60*time.Second, false, func(ctx context.Context) (bool, error) {
		if !first {
			logrus.Infof("Waiting for CRD %s to become available", crdName)
		}
		first = false

		crd, err := apiClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crdName, metav1.GetOptions{})
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

func (f *Factory) createCRD(ctx context.Context, apiClient clientset.Interface, schema *types.Schema, ready map[string]*apiext.CustomResourceDefinition) (*apiext.CustomResourceDefinition, error) {
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
			Group: schema.Version.Group,
			Versions: []apiext.CustomResourceDefinitionVersion{
				{
					Name:    schema.Version.Version,
					Served:  true,
					Storage: true,
					// Using catch-all schema as it's required in v1 and we do not have enough info in `schema`
					// Note catch-all schema used in Wrangler (open schema for "spec" and "status") is not good enough
					// here as Norman CRDs often define direct fields
					Schema: &apiext.CustomResourceValidation{
						OpenAPIV3Schema: &apiext.JSONSchemaProps{
							Type:                   "object",
							XPreserveUnknownFields: &[]bool{true}[0],
						},
					},
				},
			},
			Names: apiext.CustomResourceDefinitionNames{
				Plural: plural,
				Kind:   schema.CodeName,
			},
		},
	}

	if schema.Scope == types.NamespaceScope {
		crd.Spec.Scope = apiext.NamespaceScoped
	} else {
		crd.Spec.Scope = apiext.ClusterScoped
	}

	logrus.Infof("Creating CRD %s", name)
	crd2, err := apiClient.ApiextensionsV1().CustomResourceDefinitions().Create(ctx, crd, metav1.CreateOptions{})
	if errors.IsAlreadyExists(err) {
		return crd, nil
	}
	return crd2, err
}

func (f *Factory) getReadyCRDs(ctx context.Context, apiClient clientset.Interface) (map[string]*apiext.CustomResourceDefinition, error) {
	list, err := apiClient.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
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
