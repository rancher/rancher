package userstored

import (
	"context"
	"strings"

	"github.com/rancher/norman/store/proxy"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/api/store/sharewatch"
	clusterSchema "github.com/rancher/types/apis/cluster.cattle.io/v3/schema"
	"github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/rancher/types/config"
)

type storeWrapperFunc func(types.Store) types.Store

func addProxyStore(ctx context.Context, schemas *types.Schemas, context *config.ScaledContext, schemaType, apiVersion string, storeWrapper storeWrapperFunc) *types.Schema {
	s := schemas.Schema(&schema.Version, schemaType)
	if s == nil {
		s = schemas.Schema(&clusterSchema.Version, schemaType)
	}

	if s == nil {
		panic("Failed to find schema " + schemaType)
	}

	prefix := []string{"api"}
	group := ""
	version := "v1"
	kind := s.CodeName
	plural := s.PluralName

	parts := strings.SplitN(apiVersion, "/", 2)
	if len(parts) == 1 {
		version = parts[0]
	} else {
		group = parts[0]
		version = parts[1]
		prefix = []string{"apis"}
	}

	s.Store = proxy.NewProxyStore(context.ClientGetter,
		config.UserStorageContext,
		prefix,
		group,
		version,
		kind,
		plural)

	s.Store = &sharewatch.WatchShare{
		Store: s.Store,
		Close: ctx,
	}

	if storeWrapper != nil {
		s.Store = storeWrapper(s.Store)
	}

	return s
}
