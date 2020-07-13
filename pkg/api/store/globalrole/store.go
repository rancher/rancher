package globalrole

import (
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
)

type store struct {
	types.Store

	grLister v3.GlobalRoleLister
}

func Wrap(s types.Store, grLister v3.GlobalRoleLister) types.Store {
	return &store{
		Store:    s,
		grLister: grLister,
	}
}

func (s *store) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	globalRole, err := s.grLister.Get("", id)
	if err != nil {
		return nil, err
	}

	if globalRole.Builtin {
		return nil, httperror.NewAPIError(httperror.PermissionDenied, "cannot delete builtin global roles")
	}
	return s.Store.Delete(apiContext, schema, id)
}
