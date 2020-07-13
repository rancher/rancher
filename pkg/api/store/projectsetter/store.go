package projectsetter

import (
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/project"
	v1 "github.com/rancher/rancher/pkg/types/apis/core/v1"
	client "github.com/rancher/rancher/pkg/types/client/cluster/v3"
	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func New(store types.Store, manager *clustermanager.Manager) types.Store {
	t := &transformer{
		ClusterManager: manager,
	}
	return &transform.Store{
		Store: projectSetter{
			store,
			manager,
		},
		Transformer:       t.object,
		ListTransformer:   t.list,
		StreamTransformer: t.stream,
	}
}

type projectSetter struct {
	types.Store

	ClusterManager *clustermanager.Manager
}

type transformer struct {
	ClusterManager *clustermanager.Manager
}

func (p projectSetter) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	options := *opt
	if err := p.setOptionsNamespaces(apiContext, &options); err != nil {
		return nil, err
	}

	return p.Store.List(apiContext, schema, &options)
}

func (p projectSetter) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	if _, ok := data[client.NamespaceFieldProjectID]; ok {
		delete(data, client.NamespaceFieldProjectID)
	}
	return p.Store.Create(apiContext, schema, data)
}

func (p projectSetter) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	if _, ok := data[client.NamespaceFieldProjectID]; ok {
		delete(data, client.NamespaceFieldProjectID)
	}
	return p.Store.Update(apiContext, schema, data, id)
}

func (t *transformer) object(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
	t.lookupAndSetProjectID(apiContext, schema, data)
	return data, nil
}

func (t *transformer) list(apiContext *types.APIContext, schema *types.Schema, data []map[string]interface{}, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	namespaceLister := t.lister(apiContext, schema)
	if namespaceLister == nil {
		return data, nil
	}

	for _, item := range data {
		setProjectID(namespaceLister, item)
	}

	return data, nil
}

// Retrieves project from api sub-context, then finds associated namespaces. If found, assigns to options.
func (p projectSetter) setOptionsNamespaces(apiContext *types.APIContext, opt *types.QueryOptions) error {
	clusterName := p.ClusterManager.ClusterName(apiContext)
	if clusterName == "" {
		return nil
	}

	clusterContext, err := p.ClusterManager.UserContext(clusterName)
	if err != nil {
		return err
	}

	namespaces, err := clusterContext.Core.Namespaces("").Controller().Lister().List("", labels.NewSelector())
	if err != nil {
		return err
	}

	matchingNamespaces := getMatchingNamespaces(*apiContext, namespaces)

	if opt == nil {
		opt = &types.QueryOptions{}
	}

	// It is important this field is set to not nil even if there are no namespaces, so that no namespaces are queried instead of all namespaces
	if opt.Namespaces == nil {
		opt.Namespaces = make([]string, 0)
	}

	// Not using namespaces in opt.Conditions to avoid conflicts
	opt.Namespaces = append(opt.Namespaces, matchingNamespaces...)

	return nil
}

func getMatchingNamespaces(apiContext types.APIContext, namespaces []*k8sv1.Namespace) []string {
	var matchingNamespaces []string

	projectID := apiContext.SubContext["/v3/schemas/project"]
	if projectID == "" {
		return matchingNamespaces
	}

	for _, ns := range namespaces {
		if ns.Annotations[project.ProjectIDAnn] == projectID {
			matchingNamespaces = append(matchingNamespaces, ns.Name)
		}
	}
	return matchingNamespaces
}

func (t *transformer) stream(apiContext *types.APIContext, schema *types.Schema, data chan map[string]interface{}, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	namespaceLister := t.lister(apiContext, schema)
	if namespaceLister == nil {
		return data, nil
	}

	return convert.Chan(data, func(data map[string]interface{}) map[string]interface{} {
		setProjectID(namespaceLister, data)
		return data
	}), nil
}

func (t *transformer) lister(apiContext *types.APIContext, schema *types.Schema) v1.NamespaceLister {
	if _, ok := schema.ResourceFields[client.NamespaceFieldProjectID]; !ok || schema.ID == client.NamespaceType {
		return nil
	}

	clusterName := t.ClusterManager.ClusterName(apiContext)
	if clusterName == "" {
		return nil
	}

	clusterContext, err := t.ClusterManager.UserContext(clusterName)
	if err != nil {
		return nil
	}

	return clusterContext.Core.Namespaces("").Controller().Lister()
}

func (t *transformer) lookupAndSetProjectID(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) {
	namespaceLister := t.lister(apiContext, schema)
	if namespaceLister == nil {
		return
	}

	setProjectID(namespaceLister, data)
}

func setProjectID(namespaceLister v1.NamespaceLister, data map[string]interface{}) {
	if data == nil {
		return
	}

	ns, _ := data["namespaceId"].(string)
	projectID, _ := data[client.NamespaceFieldProjectID].(string)
	if projectID != "" {
		return
	}

	nsObj, err := namespaceLister.Get("", ns)
	if err != nil {
		return
	}

	data[client.NamespaceFieldProjectID] = nsObj.Annotations["field.cattle.io/projectId"]
}
