package userpreferences

import (
	"github.com/rancher/steve/pkg/schemaserver/store/empty"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/steve/pkg/server/store/proxy"
	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/data/convert"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

type configMapStore struct {
	empty.Store
	cg proxy.ClientGetter
}

func (e *configMapStore) getClient(apiOp *types.APIRequest) (dynamic.ResourceInterface, error) {
	cmSchema := apiOp.Schemas.LookupSchema("configmap")
	if cmSchema == nil {
		return nil, validation.NotFound
	}

	return e.cg.AdminClient(apiOp, cmSchema, "kube-system")
}

func (e *configMapStore) ByID(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	u := getUser(apiOp)
	client, err := e.getClient(apiOp)
	if err != nil {
		return types.APIObject{}, err
	}

	pref := &UserPreference{
		Data: map[string]string{},
	}
	result := types.APIObject{
		Type:   "userpreference",
		ID:     u.GetName(),
		Object: pref,
	}

	obj, err := client.Get(prefName(u), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return result, nil
	}

	d := data.Object(obj.Object).Map("data")
	return result, convert.ToObj(d, &pref.Data)
}

func (e *configMapStore) List(apiOp *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
	obj, err := e.ByID(apiOp, schema, "")
	if err != nil {
		return types.APIObjectList{}, err
	}
	return types.APIObjectList{
		Objects: []types.APIObject{
			obj,
		},
	}, nil
}

func (e *configMapStore) Update(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject, id string) (types.APIObject, error) {
	u := getUser(apiOp)
	client, err := e.getClient(apiOp)
	if err != nil {
		return types.APIObject{}, err
	}

	obj, err := client.Get(prefName(u), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = client.Create(&unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": prefName(u),
				},
				"data": data.Data().Map("data"),
			},
		}, metav1.CreateOptions{})
	} else if err == nil {
		obj.Object["data"] = data.Data().Map("data")
		_, err = client.Update(obj, metav1.UpdateOptions{})
	}
	if err != nil {
		return types.APIObject{}, err
	}

	return e.ByID(apiOp, schema, "")
}

func (e *configMapStore) Delete(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	u := getUser(apiOp)
	client, err := e.getClient(apiOp)
	if err != nil {
		return types.APIObject{}, err
	}

	return types.APIObject{}, client.Delete(prefName(u), nil)
}
