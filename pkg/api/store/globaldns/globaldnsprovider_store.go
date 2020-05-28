package globaldns

import (
	"strings"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	managementv3 "github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/namespace"
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
	var globalDNSs []managementv3.GlobalDns

	conditions := []*types.QueryCondition{
		types.NewConditionFromString(managementv3.GlobalDnsFieldProviderID, types.ModifierEQ, []string{id}...),
	}

	if err := access.List(apiContext, apiContext.Version, managementv3.GlobalDnsType, &types.QueryOptions{Conditions: conditions}, &globalDNSs); err != nil {
		return err
	}

	if len(globalDNSs) > 0 {
		return httperror.NewAPIError(httperror.PermissionDenied, "Cannot delete the provider until GlobalDNS entries referring it are removed")
	}

	return nil
}

func ProviderPwdWrap(store types.Store) types.Store {
	storeWrapped := &ProviderPwdStore{
		Store: store,
	}
	return storeWrapped
}

type ProviderPwdStore struct {
	types.Store
}

func (p *ProviderPwdStore) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {

	providerFound, keys := findProviderPasswordKeys(data)
	if !providerFound {
		return p.Store.Update(apiContext, schema, data, id)
	}

	_, passwordFound := values.GetValue(data, keys...)

	if !passwordFound {
		existingProvider, err := p.ByID(apiContext, schema, id)
		if err != nil {
			return nil, err
		}
		existingPassword, _ := values.GetValue(existingProvider, keys...)
		if !strings.HasPrefix(convert.ToString(existingPassword), namespace.GlobalNamespace) {
			values.PutValue(data, existingPassword, keys...)
		}
	}

	return p.Store.Update(apiContext, schema, data, id)
}

func findProviderPasswordKeys(data map[string]interface{}) (bool, []string) {
	var keys []string

	_, route53 := values.GetValue(data, "route53ProviderConfig")
	if route53 {
		return true, []string{"route53ProviderConfig", "secretKey"}
	}

	_, cloudFlare := values.GetValue(data, "cloudflareProviderConfig")
	if cloudFlare {
		return true, []string{"cloudflareProviderConfig", "apiKey"}
	}

	_, aliDNS := values.GetValue(data, "alidnsProviderConfig")
	if aliDNS {
		return true, []string{"alidnsProviderConfig", "secretKey"}
	}

	return false, keys
}
