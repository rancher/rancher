package navlinks

import (
	"context"

	"github.com/rancher/apiserver/pkg/types"
	schema2 "github.com/rancher/steve/pkg/schema"
	steve "github.com/rancher/steve/pkg/server"
)

func Register(ctx context.Context, server *steve.Server) {
	server.SchemaFactory.AddTemplate(schema2.Template{
		Group: "ui.cattle.io",
		Kind:  "NavLink",
		StoreFactory: func(innerStore types.Store) types.Store {
			return &store{
				Store: innerStore,
			}
		},
	})
}
