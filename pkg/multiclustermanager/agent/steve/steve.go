package steve

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/rancher/rancher/pkg/rancher"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var (
	running bool
	runLock sync.Mutex
)

func Run(ctx context.Context) error {
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
		err = http.ListenAndServe(":8080", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			resp, err := http.Get("http://localhost:6080/healthz")
			if err != nil {
				http.Error(rw, err.Error(), http.StatusInternalServerError)
				return
			}
			defer resp.Body.Close()
			rw.WriteHeader(resp.StatusCode)
			_, _ = io.Copy(rw, resp.Body)
		}))
		panic("health check server failed: " + err.Error())
	}()

	go func() {
		for {
			ctx, cancel := context.WithCancel(ctx)
			r, err := rancher.New(ctx, reverseClientConfig{cfg: c}, &rancher.Options{
				BindHost:        "127.0.0.1",
				HTTPListenPort:  6080,
				HTTPSListenPort: 0,
				AddLocal:        "true",
				Agent:           true,
			})
			if err != nil {
				cancel()
				logrus.Errorf("failed to initialize Rancher: %v", err)
				time.Sleep(10 * time.Second)
				continue
			}

			if err := r.ListenAndServe(ctx); err != nil {
				cancel()
				logrus.Errorf("failed to start Rancher: %v", err)
				time.Sleep(10 * time.Second)
				continue
			}

			cancel()
		}
	}()

	running = true
	return nil
}

type reverseClientConfig struct {
	cfg *rest.Config
}

func (r reverseClientConfig) RawConfig() (clientcmdapi.Config, error) {
	panic("not implemented")
}

func (r reverseClientConfig) ClientConfig() (*rest.Config, error) {
	return r.cfg, nil
}

func (r reverseClientConfig) Namespace() (string, bool, error) {
	return "", false, nil
}

func (r reverseClientConfig) ConfigAccess() clientcmd.ConfigAccess {
	panic("not implemented ")
}
