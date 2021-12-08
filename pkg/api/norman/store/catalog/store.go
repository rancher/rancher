package catalog

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	c "github.com/rancher/rancher/pkg/api/norman/customization/catalog"
	gaccess "github.com/rancher/rancher/pkg/api/norman/customization/globalnamespaceaccess"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
)

type Store struct {
	types.Store
	Users     v3.UserInterface
	GrbLister v3.GlobalRoleBindingLister
	GrLister  v3.GlobalRoleLister
}

func Wrap(store types.Store, users v3.UserInterface, grbLister v3.GlobalRoleBindingLister, grLister v3.GlobalRoleLister) types.Store {
	return &Store{
		Store:     store,
		Users:     users,
		GrbLister: grbLister,
		GrLister:  grLister,
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
		isRestrictedAdmin, err := s.isRestrictedAdmin(apiContext)
		if err != nil {
			return nil, err
		}
		if strings.ToLower(settings.SystemCatalog.Get()) == "bundled" || isRestrictedAdmin {
			return nil, httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprint("not allowed to edit system-library catalog"))
		}
	}
	return s.Store.Update(apiContext, schema, data, id)
}

func (s *Store) isRestrictedAdmin(apiContext *types.APIContext) (bool, error) {
	ma := gaccess.MemberAccess{
		Users:     s.Users,
		GrLister:  s.GrLister,
		GrbLister: s.GrbLister,
	}
	callerID := apiContext.Request.Header.Get(gaccess.ImpersonateUserHeader)

	return ma.IsRestrictedAdmin(callerID)
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
