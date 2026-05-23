package operations

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/operations/etcdsnapshotsave"
	"github.com/rancher/rancher/pkg/wrangler"
)

func Register(ctx context.Context, clients *wrangler.CAPIContext) {
	etcdsnapshotsave.Register(ctx, clients)
}
