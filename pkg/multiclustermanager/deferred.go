package multiclustermanager

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/rancher/rancher/pkg/wrangler"
)

type DeferredServer struct {
	sync.RWMutex

	wrangler *wrangler.Context
	opts     *Options
	mcm      *mcm
}

func NewDeferredServer(wrangler *wrangler.Context, opts *Options) *DeferredServer {
	return &DeferredServer{
		wrangler: wrangler,
		opts:     opts,
	}
}

func (s *DeferredServer) Start(ctx context.Context) error {
	s.Lock()
	defer s.Unlock()

	if s.mcm != nil {
		return nil
	}

	var (
		mcm *mcm
		err error
	)

	err = s.wrangler.StartWithTransaction(ctx, func(ctx context.Context) error {
		mcm, err = newMCM(ctx, s.wrangler, s.opts)
		if err != nil {
			return err
		}

		return mcm.Start(ctx)
	})
	if err != nil {
		return err
	}

	s.mcm = mcm
	go func() {
		<-ctx.Done()
		s.Lock()
		defer s.Unlock()
		s.mcm = nil
	}()
	return nil
}

func (s *DeferredServer) Middleware(next http.Handler) http.Handler {
	s.RLock()
	defer s.RUnlock()
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if s.mcm == nil {
			next.ServeHTTP(rw, req)
			return
		}
		s.mcm.Middleware(next).ServeHTTP(rw, req)
	})
}

func (s *DeferredServer) ClusterDialer(clusterID string) func(ctx context.Context, network, address string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		s.RLock()
		if s.mcm == nil {
			s.RUnlock()
			return nil, fmt.Errorf("failed to find cluster %s", clusterID)
		}
		dialer, err := s.mcm.ScaledContext.Dialer.ClusterDialer(clusterID)
		s.RUnlock()
		if err != nil {
			return nil, err
		}
		return dialer(ctx, network, address)
	}
}
