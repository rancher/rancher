package deferred

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/multiclustermanager/options"
	"github.com/rancher/rancher/pkg/wrangler"
	"k8s.io/client-go/kubernetes"
)

type Factory func(ctx context.Context, wranglerContext *wrangler.Context, cfg *options.Options) (wrangler.MultiClusterManager, error)

type Server struct {
	sync.RWMutex

	wrangler *wrangler.Context
	opts     *options.Options
	mcm      wrangler.MultiClusterManager
	factory  Factory
}

func NewDeferredServer(wrangler *wrangler.Context, factory Factory, opts *options.Options) *Server {
	return &Server{
		wrangler: wrangler,
		factory:  factory,
		opts:     opts,
	}
}

func (s *Server) Wait(ctx context.Context) {
	if !features.MCM.Enabled() {
		return
	}
	for {
		s.Lock()
		if s.mcm == nil {
			s.Unlock()
			select {
			case <-time.After(500 * time.Millisecond):
				continue
			case <-ctx.Done():
				return
			}
		}
		s.Unlock()
		s.mcm.Wait(ctx)
		break
	}
}

func (s *Server) Start(ctx context.Context) error {
	s.Lock()
	defer s.Unlock()

	if s.mcm != nil {
		return nil
	}

	var (
		mcm wrangler.MultiClusterManager
		err error
	)

	err = s.wrangler.StartWithTransaction(ctx, func(ctx context.Context) error {
		mcm, err = s.factory(ctx, s.wrangler, s.opts)
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

func (s *Server) getMCM() wrangler.MultiClusterManager {
	s.RLock()
	defer s.RUnlock()
	return s.mcm
}

func (s *Server) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		mcm := s.getMCM()
		if mcm == nil {
			next.ServeHTTP(rw, req)
			return
		}
		mcm.Middleware(next).ServeHTTP(rw, req)
	})
}

func (s *Server) ClusterDialer(clusterID string) func(ctx context.Context, network, address string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		mcm := s.getMCM()
		if mcm == nil {
			return nil, fmt.Errorf("failed to find cluster %s", clusterID)
		}
		return mcm.ClusterDialer(clusterID)(ctx, network, address)
	}
}

func (s *Server) K8sClient(clusterName string) (kubernetes.Interface, error) {
	mcm := s.getMCM()
	if mcm == nil {
		return nil, nil
	}
	return mcm.K8sClient(clusterName)
}
