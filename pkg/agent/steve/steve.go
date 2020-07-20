package steve

import (
	"context"
	"sync"

	server2 "github.com/rancher/dynamiclistener/server"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/steve/pkg/auth"
	"github.com/rancher/steve/pkg/server"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

var (
	running bool
	runLock sync.Mutex
)

func Run(ctx context.Context, namespace string) error {
	if !features.Steve.Enabled() {
		return nil
	}

	runLock.Lock()
	defer runLock.Unlock()

	if running {
		return nil
	}

	logrus.Info("Starting steve")
	c, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	s := server.Server{
		RESTConfig:     c,
		AuthMiddleware: auth.ToMiddleware(auth.AuthenticatorFunc(auth.Impersonation)),
	}

	go func() {
		err := s.ListenAndServe(ctx, 6443, 6080, &server2.ListenOpts{
			BindHost: "127.0.0.1",
		})
		logrus.Fatalf("steve exited: %v", err)
	}()

	running = true
	return nil
}
