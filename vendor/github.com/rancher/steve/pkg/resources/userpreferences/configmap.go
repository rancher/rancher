package userpreferences

import (
	"github.com/rancher/apiserver/pkg/store/empty"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/steve/pkg/stores/proxy"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type configMapStore struct {
	empty.Store
	cg proxy.ClientGetter
}

func (e *configMapStore) getClient(apiOp *types.APIRequest) (v1.ConfigMapInterface, error) {
	c, err := e.cg.AdminK8sInterface()
	if err != nil {
		return nil, err
	}
	return c.CoreV1().ConfigMaps("kube-system"), nil
}

func newPref(u user.Info) (types.APIObject, *UserPreference) {
	pref := &UserPreference{
		Data: map[string]string{},
	}
	return types.APIObject{
		Type:   "userpreference",
		ID:     u.GetName(),
		Object: pref,
	}, pref
}

func (e *configMapStore) ByID(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	u := getUser(apiOp)
	result, pref := newPref(u)

	client, err := e.getClient(apiOp)
	if err == validation.NotFound {
		return result, nil
	} else if err != nil {
		return types.APIObject{}, err
	}

	obj, err := client.Get(apiOp.Context(), prefName(u), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return result, nil
	}

	pref.Data = obj.Data
	return result, nil
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

	values := map[string]string{}
	for k, v := range data.Data().Map("data") {
		values[k] = convert.ToString(v)
	}

	obj, err := client.Get(apiOp.Context(), prefName(u), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = client.Create(apiOp.Context(), &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: prefName(u),
			},
			Data: values,
		}, metav1.CreateOptions{})
	} else if err == nil {
		obj.Data = values
		_, err = client.Update(apiOp.Context(), obj, metav1.UpdateOptions{})
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

	return types.APIObject{}, client.Delete(apiOp.Context(), prefName(u), metav1.DeleteOptions{})
}
