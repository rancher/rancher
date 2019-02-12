package globaldns

import (
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	managementv3 "github.com/rancher/types/client/management/v3"
)

func ProviderWrap(store types.Store) types.Store {
	storeWrapped := &ProviderStore{
		Store: store,
	}
	return storeWrapped
}

type ProviderStore struct {
	types.Store
}

func (p *ProviderStore) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {

	if err := canDeleteProvider(apiContext, id); err != nil {
		return nil, err
	}

	return p.Store.Delete(apiContext, schema, id)
}

func canDeleteProvider(apiContext *types.APIContext, id string) error {

	//check if there are any globalDNS entries referencing this provider
	var globalDNSs []managementv3.GlobalDNS

	conditions := []*types.QueryCondition{
		types.NewConditionFromString(managementv3.GlobalDNSFieldProviderID, types.ModifierEQ, []string{id}...),
	}

	if err := access.List(apiContext, apiContext.Version, managementv3.GlobalDNSType, &types.QueryOptions{Conditions: conditions}, &globalDNSs); err != nil {
		return err
	}

	if len(globalDNSs) > 0 {
		return httperror.NewAPIError(httperror.PermissionDenied, "Cannot delete the provider until GlobalDNS entries referring it are removed")
	}

	return nil
}
