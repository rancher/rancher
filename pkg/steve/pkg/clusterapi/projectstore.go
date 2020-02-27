package clusterapi

import (
	"net/http"

	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/steve/pkg/server/store/proxy"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/endpoints/request"
)

var (
	creatorPath = []string{
		"metadata", "annotations", "field.cattle.io/creatorId",
	}
)

type projectStore struct {
	types.Store
	clients proxy.ClientGetter
}

func (p *projectStore) setCreator(apiOp *types.APIRequest, data types.APIObject) (types.APIObject, error) {
	user, ok := request.UserFrom(apiOp.Context())
	if !ok {
		return types.APIObject{}, validation.Unauthorized
	}
	return p.setCreatorWithUser(apiOp, user.GetName(), data)
}

func (p *projectStore) setCreatorWithUser(apiOp *types.APIRequest, user string, data types.APIObject) (types.APIObject, error) {
	newData := data.Data()
	newData.SetNested(user, creatorPath...)
	data.Object = newData
	return data, nil
}

func (p *projectStore) Create(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject) (types.APIObject, error) {
	data, err := p.setCreator(apiOp, data)
	if err != nil {
		return types.APIObject{}, err
	}
	return p.Store.Create(apiOp, schema, data)
}

func (p *projectStore) Update(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject, id string) (types.APIObject, error) {
	if apiOp.Method == http.MethodPatch {
		return types.APIObject{}, validation.MethodNotAllowed
	}

	obj, err := p.Store.ByID(apiOp, schema, id)
	if err != nil {
		return types.APIObject{}, err
	}

	data, err = p.setCreatorWithUser(apiOp, obj.Data().String(creatorPath...), data)
	if err != nil {
		return types.APIObject{}, err
	}

	return p.Update(apiOp, schema, data, id)
}

func (p *projectStore) getProjects(apiOp *types.APIRequest, schema *types.APISchema, namespace string) ([]string, error) {
	client, err := p.clients.Client(apiOp, schema, namespace)
	if err != nil {
		return nil, err
	}

	objs, err := client.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var result []string
	for _, obj := range objs.Items {
		result = append(result, obj.GetName())
	}

	return result, nil
}

func (p *projectStore) newRequest(apiOp *types.APIRequest) (*types.APIRequest, error) {
	namespaces, err := p.getProjects(apiOp, apiOp.Schemas.LookupSchema("project"), apiOp.Namespace)
	if err != nil {
		return nil, err
	}

	req := proxy.AddNamespaceConstraint(apiOp.Request, namespaces...)
	newAPIOp := *apiOp
	newAPIOp.Namespace = ""
	newAPIOp.Request = req
	return &newAPIOp, nil
}

func (p *projectStore) List(apiOp *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
	newReq, err := p.newRequest(apiOp)
	if err != nil {
		return types.APIObjectList{}, err
	}

	return p.Store.List(newReq, schema)
}

func (p *projectStore) Watch(apiOp *types.APIRequest, schema *types.APISchema, wr types.WatchRequest) (chan types.APIEvent, error) {
	newReq, err := p.newRequest(apiOp)
	if err != nil {
		return nil, err
	}

	return p.Store.Watch(newReq, schema, wr)
}
