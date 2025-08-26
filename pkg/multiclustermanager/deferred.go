package multiclustermanager

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
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

func (s *DeferredServer) Wait(ctx context.Context) {
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

func (s *DeferredServer) NormanSchemas() *types.Schemas {
	mcm := s.getMCM()
	if mcm == nil {
		return nil
	}
	return mcm.NormanSchemas()
}

func (s *DeferredServer) Start(ctx context.Context) error {
	logrus.Info("HITHERE Locking")
	s.Lock()
	logrus.Info("HITHERE Locked")
	defer s.Unlock()

	if s.mcm != nil {
		logrus.Info("HITHERE bye")
		return nil
	}

	var (
		mcm *mcm
		err error
	)

	logrus.Info("HITHERE StartWithTransaction")
	err = s.wrangler.StartWithTransaction(ctx, func(ctx context.Context) error {
		logrus.Info("HITHERE calling newMCM")
		mcm, err = newMCM(ctx, s.wrangler, s.opts)
		if err != nil {
			return err
		}

		return mcm.Start(ctx)
	})
	if mcm != nil {
		// always start, even on error
		mcm.started(ctx)
	}
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
		dialer, err := mcm.ScaledContext.Dialer.ClusterDialer(clusterID, true)
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
	clusterContext, err := mcm.clusterManager.UserContextNoControllers(clusterName)
	if err != nil {
		return nil, err
	}
	return clusterContext.K8sClient, nil
}
