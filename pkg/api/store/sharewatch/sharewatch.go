package sharewatch

import (
	"context"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/broadcast"
)

type WatchShare struct {
	Close context.Context
	types.Store
	broadcaster broadcast.Broadcaster
}

func (w *WatchShare) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	return w.broadcaster.Subscribe(apiContext.Request.Context(), func() (chan map[string]interface{}, error) {
		newAPIContext := *apiContext
		newAPIContext.Request = apiContext.Request.WithContext(w.Close)
		return w.Store.Watch(&newAPIContext, schema, &types.QueryOptions{})
	})
}
