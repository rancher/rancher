package accesscontrol

import (
	"github.com/rancher/apiserver/pkg/server"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/attributes"
)

type AccessControl struct {
	server.SchemaBasedAccess
}

func NewAccessControl() *AccessControl {
	return &AccessControl{}
}

func (a *AccessControl) CanWatch(apiOp *types.APIRequest, schema *types.APISchema) error {
	if attributes.GVK(schema).Kind != "" {
		access := GetAccessListMap(schema)
		if _, ok := access["watch"]; ok {
			return nil
		}
	}
	return a.SchemaBasedAccess.CanWatch(apiOp, schema)
}
