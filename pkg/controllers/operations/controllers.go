package operations

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/operations/encryptionkeyrotation"
	"github.com/rancher/rancher/pkg/controllers/operations/etcdsnapshotrestore"
	"github.com/rancher/rancher/pkg/controllers/operations/etcdsnapshotsave"
	"github.com/rancher/rancher/pkg/wrangler"
)

func Register(ctx context.Context, clients *wrangler.CAPIContext) {
	encryptionkeyrotation.Register(ctx, clients)
	etcdsnapshotsave.Register(ctx, clients)
	etcdsnapshotrestore.Register(ctx, clients)
}
