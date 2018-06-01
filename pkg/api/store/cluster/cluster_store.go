package cluster

import (
	"sync"

	"strings"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	managementv3 "github.com/rancher/types/client/management/v3"
)

type Store struct {
	types.Store
	ShellHandler types.RequestHandler
	mu           sync.Mutex
}

func (r *Store) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	// Really we want a link handler but the URL parse makes it impossible to add links to clusters for now.  So this
	// is basically a hack
	if apiContext.Query.Get("shell") == "true" {
		return nil, r.ShellHandler(apiContext, nil)
	}
	return r.Store.ByID(apiContext, schema, id)
}

func (r *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {

	name := convert.ToString(data["name"])
	if name == "" {
		return nil, httperror.NewFieldAPIError(httperror.MissingRequired, "Cluster name", "")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if err := canUseClusterName(apiContext, name); err != nil {
		return nil, err
	}

	return r.Store.Create(apiContext, schema, data)
}

func (r *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {

	updatedName := convert.ToString(data["name"])
	if updatedName == "" {
		return nil, httperror.NewFieldAPIError(httperror.MissingRequired, "Cluster name", "")
	}

	existingCluster, err := r.ByID(apiContext, schema, id)
	if err != nil {
		return nil, err
	}

	clusterName, ok := existingCluster["name"].(string)
	if !ok {
		clusterName = ""
	}

	if !strings.EqualFold(updatedName, clusterName) {
		r.mu.Lock()
		defer r.mu.Unlock()

		if err := canUseClusterName(apiContext, updatedName); err != nil {
			return nil, err
		}
	}
	return r.Store.Update(apiContext, schema, data, id)
}

func canUseClusterName(apiContext *types.APIContext, requestedName string) error {
	var clusters []managementv3.Cluster

	if err := access.List(apiContext, apiContext.Version, managementv3.ClusterType, &types.QueryOptions{}, &clusters); err != nil {
		return err
	}

	for _, c := range clusters {
		if c.Removed == "" && strings.EqualFold(c.Name, requestedName) {
			//cluster exists by this name
			return httperror.NewFieldAPIError(httperror.NotUnique, "Cluster name", "")
		}
	}

	return nil
}
