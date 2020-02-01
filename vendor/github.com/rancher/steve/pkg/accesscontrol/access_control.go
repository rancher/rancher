package accesscontrol

import (
	"fmt"

	"github.com/rancher/steve/pkg/schemaserver/server"
	"github.com/rancher/steve/pkg/schemaserver/types"
)

type AccessControl struct {
	server.AllAccess
}

func NewAccessControl() *AccessControl {
	return &AccessControl{}
}

func (a *AccessControl) CanWatch(apiOp *types.APIRequest, schema *types.APISchema) error {
	access := GetAccessListMap(schema)
	if !access.Grants("watch", "*", "*") {
		return fmt.Errorf("watch not allowed")
	}
	return nil
}
