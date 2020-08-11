package multiclustermanager

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/rancher/rancher/pkg/wrangler"
	"k8s.io/client-go/kubernetes"
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

func (s *DeferredServer) getMCM() *mcm {
	s.RLock()
	defer s.RUnlock()
	return s.mcm
}

func (s *DeferredServer) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		mcm := s.getMCM()
		if mcm == nil {
			next.ServeHTTP(rw, req)
			return
		}
		mcm.Middleware(next).ServeHTTP(rw, req)
	})
}

func (s *DeferredServer) ClusterDialer(clusterID string) func(ctx context.Context, network, address string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		mcm := s.getMCM()
		if mcm == nil {
			return nil, fmt.Errorf("failed to find cluster %s", clusterID)
		}
		dialer, err := mcm.ScaledContext.Dialer.ClusterDialer(clusterID)
		if err != nil {
			return nil, err
		}
		return dialer(ctx, network, address)
	}
}

func (s *DeferredServer) K8sClient(clusterName string) (kubernetes.Interface, error) {
	mcm := s.getMCM()
	if mcm == nil {
		return nil, nil
	}
	clusterContext, err := mcm.clusterManager.UserContext(clusterName)
	if err != nil {
		return nil, err
	}
	return clusterContext.K8sClient, nil
}
