package steve

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/rancher/rancher/pkg/rancher"

	"github.com/rancher/rancher/pkg/features"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

var (
	running bool
	runLock sync.Mutex
)

func Run() error {
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

	go func() {
		for {
			ctx, cancel := context.WithCancel(context.Background())
			r, err := rancher.New(ctx, c, &rancher.Options{
				AddLocal: "true",
				Agent:    true,
			})
			if err != nil {
				cancel()
				logrus.Errorf("failed to initialize Rancher: %v", err)
				time.Sleep(10 * time.Second)
				continue
			}

			if err := r.Start(ctx); err != nil {
				cancel()
				logrus.Errorf("failed to start Rancher: %v", err)
				time.Sleep(10 * time.Second)
				continue
			}

			if err := http.ListenAndServe("127.0.0.1:6080", r.Handler); err != nil {
				logrus.Fatalf("steve exited: %v", err)
			}
			cancel()
		}
	}()

	running = true
	return nil
}
