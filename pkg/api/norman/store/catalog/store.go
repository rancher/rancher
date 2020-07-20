package catalog

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	c "github.com/rancher/rancher/pkg/api/norman/customization/catalog"
	"github.com/rancher/rancher/pkg/settings"
)

type Store struct {
	types.Store
}

func Wrap(store types.Store) types.Store {
	return &Store{
		store,
	}
}

func (s *Store) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	isSystemCatalog, err := s.isSystemCatalog(apiContext, schema, id)
	if err != nil {
		return nil, err
	}
	if isSystemCatalog {
		return nil, httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprint("not allowed to delete system-library catalog"))
	}
	return s.Store.Delete(apiContext, schema, id)
}

func (s *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	isSystemCatalog, err := s.isSystemCatalog(apiContext, schema, id)
	if err != nil {
		return nil, err
	}
	if isSystemCatalog {
		if strings.ToLower(settings.SystemCatalog.Get()) == "bundled" {
			return nil, httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprint("not allowed to edit system-library catalog"))
		}
	}
	return s.Store.Update(apiContext, schema, data, id)
}

// isSystemCatalog checks whether the catalog is the the system catalog maintained by rancher
func (s *Store) isSystemCatalog(apiContext *types.APIContext, schema *types.Schema, id string) (bool, error) {
	catalog, err := s.ByID(apiContext, schema, id)
	if err != nil {
		return false, err
	}
	if catalog["url"] == c.SystemLibraryURL && catalog["name"] == c.SystemCatalogName {
		return true, nil
	}
	return false, nil
}
