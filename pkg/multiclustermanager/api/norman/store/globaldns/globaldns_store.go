package globaldns

import (
	"strings"
	"sync"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	managementv3 "github.com/rancher/rancher/pkg/client/generated/management/v3"
)

func Wrap(store types.Store) types.Store {
	storeWrapped := &Store{
		Store: store,
	}
	storeWrapped.mu = sync.Mutex{}
	return storeWrapped
}

type Store struct {
	types.Store
	mu sync.Mutex
}

func (p *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	fqdn := convert.ToString(data[managementv3.GlobalDnsFieldFQDN])

	p.mu.Lock()
	defer p.mu.Unlock()

	if err := canUseFQDN(apiContext, fqdn); err != nil {
		return nil, err
	}

	return p.Store.Create(apiContext, schema, data)
}

func (p *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	updatedFQDN := convert.ToString(data[managementv3.GlobalDnsFieldFQDN])

	existingGlobalDNS, err := p.ByID(apiContext, schema, id)
	if err != nil {
		return nil, err
	}

	fqdn := convert.ToString(existingGlobalDNS[managementv3.GlobalDnsFieldFQDN])

	if !strings.EqualFold(updatedFQDN, fqdn) {
		p.mu.Lock()
		defer p.mu.Unlock()

		if err := canUseFQDN(apiContext, updatedFQDN); err != nil {
			return nil, err
		}
	}

	return p.Store.Update(apiContext, schema, data, id)
}

func canUseFQDN(apiContext *types.APIContext, fqdnRequested string) error {
	var globalDNSs []managementv3.GlobalDns

	conditions := []*types.QueryCondition{
		types.NewConditionFromString(managementv3.GlobalDnsFieldFQDN, types.ModifierEQ, []string{fqdnRequested}...),
	}

	if err := access.List(apiContext, apiContext.Version, managementv3.GlobalDnsType, &types.QueryOptions{Conditions: conditions}, &globalDNSs); err != nil {
		return err
	}

	if len(globalDNSs) > 0 {
		return httperror.NewFieldAPIError(httperror.NotUnique, managementv3.GlobalDnsFieldFQDN, "")
	}

	return nil
}
