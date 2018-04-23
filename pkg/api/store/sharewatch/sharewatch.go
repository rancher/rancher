package sharewatch

import (
	"context"
	"sync"

	"github.com/rancher/norman/store/proxy"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/broadcast"
	"k8s.io/client-go/rest"
)

type WatchShare struct {
	sync.Mutex
	types.Store
	close        context.Context
	clientGetter proxy.ClientGetter
	broadcasters map[rest.Interface]*broadcast.Broadcaster
}

func NewWatchShare(ctx context.Context, getter proxy.ClientGetter, store types.Store) *WatchShare {
	return &WatchShare{
		Store:        store,
		close:        ctx,
		clientGetter: getter,
		broadcasters: map[rest.Interface]*broadcast.Broadcaster{},
	}
}

func (w *WatchShare) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	client, err := w.clientGetter.UnversionedClient(apiContext, w.Context())
	if err != nil {
		return nil, err
	}

	var b *broadcast.Broadcaster
	w.Lock()
	b, ok := w.broadcasters[client]
	if !ok {
		b = &broadcast.Broadcaster{}
		w.broadcasters[client] = b
	}
	w.Unlock()

	return b.Subscribe(apiContext.Request.Context(), func() (chan map[string]interface{}, error) {
		newAPIContext := *apiContext
		newAPIContext.Request = apiContext.Request.WithContext(w.close)
		return w.Store.Watch(&newAPIContext, schema, &types.QueryOptions{})
	})
}
