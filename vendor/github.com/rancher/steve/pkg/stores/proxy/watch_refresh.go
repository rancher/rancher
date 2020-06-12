package proxy

import (
	"context"
	"time"

	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/accesscontrol"
	"k8s.io/apiserver/pkg/endpoints/request"
)

type WatchRefresh struct {
	types.Store
	asl accesscontrol.AccessSetLookup
}

func (w *WatchRefresh) Watch(apiOp *types.APIRequest, schema *types.APISchema, wr types.WatchRequest) (chan types.APIEvent, error) {
	user, ok := request.UserFrom(apiOp.Context())
	if !ok {
		return w.Store.Watch(apiOp, schema, wr)
	}

	as := w.asl.AccessFor(user)
	ctx, cancel := context.WithCancel(apiOp.Context())
	apiOp = apiOp.WithContext(ctx)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Second):
			}

			newAs := w.asl.AccessFor(user)
			if as.ID != newAs.ID {
				// RBAC changed
				cancel()
				return
			}
		}
	}()

	return w.Store.Watch(apiOp, schema, wr)
}
