package managementstored

import (
	"context"
	"sync"

	"github.com/rancher/norman/store/crd"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/api/store/sharewatch"
	"github.com/rancher/types/config"
)

func createCrd(ctx context.Context, wg *sync.WaitGroup, factory *crd.Factory, schemas *types.Schemas, version *types.APIVersion, schemaIDs ...string) {
	wg.Add(1)
	go func() {
		defer wg.Done()

		var schemasToCreate []*types.Schema

		for _, schemaID := range schemaIDs {
			s := schemas.Schema(version, schemaID)
			if s == nil {
				panic("can not find schema " + schemaID)
			}
			schemasToCreate = append(schemasToCreate, s)
		}

		err := factory.AssignStores(ctx, config.ManagementStorageContext, schemasToCreate...)
		if err != nil {
			panic("creating CRD store " + err.Error())
		}

		for _, schema := range schemasToCreate {
			schema.Store = &sharewatch.WatchShare{
				Store: schema.Store,
				Close: ctx,
			}
		}
	}()
}
