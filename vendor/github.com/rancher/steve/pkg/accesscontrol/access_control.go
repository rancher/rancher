package accesscontrol

import (
	"fmt"

	"github.com/rancher/steve/pkg/schemaserver/server"
	"github.com/rancher/steve/pkg/schemaserver/types"
)

type AccessControl struct {
	server.SchemaBasedAccess
}

func NewAccessControl() *AccessControl {
	return &AccessControl{}
}

func (a *AccessControl) CanWatch(apiOp *types.APIRequest, schema *types.APISchema) error {
	access := GetAccessListMap(schema)
	if _, ok := access["watch"]; ok {
		return nil
	}
	return fmt.Errorf("watch not allowed")
}
