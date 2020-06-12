package userpreferences

import (
	"github.com/rancher/apiserver/pkg/store/empty"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/attributes"
	"github.com/rancher/steve/pkg/stores/proxy"
	"github.com/rancher/wrangler/pkg/data/convert"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

var (
	rancherSchema = "management.cattle.io.preference"
)

type rancherPrefStore struct {
	empty.Store
	cg proxy.ClientGetter
}

func (e *rancherPrefStore) getClient(apiOp *types.APIRequest) (dynamic.ResourceInterface, error) {
	u := getUser(apiOp).GetName()
	cmSchema := apiOp.Schemas.LookupSchema(rancherSchema)
	if cmSchema == nil {
		return nil, validation.NotFound
	}

	return e.cg.AdminClient(apiOp, cmSchema, u)
}

func (e *rancherPrefStore) ByID(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
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

	objs, err := client.List(apiOp.Context(), metav1.ListOptions{})
	if err != nil {
		return result, err
	}

	for _, obj := range objs.Items {
		pref.Data[obj.GetName()] = convert.ToString(obj.Object["value"])
	}

	return result, nil
}

func (e *rancherPrefStore) List(apiOp *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
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

func (e *rancherPrefStore) createNamespace(apiOp *types.APIRequest, ns string) error {
	client, err := e.cg.AdminClient(apiOp, apiOp.Schemas.LookupSchema("namespace"), "")
	if err != nil {
		return err
	}
	_, err = client.Get(apiOp.Context(), ns, metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		return err
	}
	_, err = client.Create(apiOp.Context(), &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": ns,
			},
		},
	}, metav1.CreateOptions{})
	return err
}

func (e *rancherPrefStore) Update(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject, id string) (types.APIObject, error) {
	client, err := e.getClient(apiOp)
	if err != nil {
		return types.APIObject{}, err
	}

	gvk := attributes.GVK(apiOp.Schemas.LookupSchema(rancherSchema))

	newValues := map[string]string{}
	for k, v := range data.Data().Map("data") {
		newValues[k] = convert.ToString(v)
	}

	prefs, err := client.List(apiOp.Context(), metav1.ListOptions{})
	if err != nil {
		return types.APIObject{}, err
	}

	for _, pref := range prefs.Items {
		key := pref.GetName()
		newValue, ok := newValues[key]
		delete(newValues, key)
		if ok && newValue != pref.Object["value"] {
			pref.Object["value"] = newValue
			_, err := client.Update(apiOp.Context(), &pref, metav1.UpdateOptions{})
			if err != nil {
				return types.APIObject{}, err
			}
		} else if !ok {
			err := client.Delete(apiOp.Context(), key, metav1.DeleteOptions{})
			if err != nil {
				return types.APIObject{}, err
			}
		}
	}

	nsExists := false
	for k, v := range newValues {
		if !nsExists {
			if err := e.createNamespace(apiOp, getUser(apiOp).GetName()); err != nil {
				return types.APIObject{}, err
			}
			nsExists = true
		}

		_, err = client.Create(apiOp.Context(), &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": gvk.GroupVersion().String(),
				"kind":       gvk.Kind,
				"metadata": map[string]interface{}{
					"name": k,
				},
				"value": v,
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return types.APIObject{}, err
		}
	}

	return e.ByID(apiOp, schema, "")
}

func (e *rancherPrefStore) Delete(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	client, err := e.getClient(apiOp)
	if err != nil {
		return types.APIObject{}, err
	}

	return types.APIObject{}, client.DeleteCollection(apiOp.Context(), metav1.DeleteOptions{}, metav1.ListOptions{})
}
