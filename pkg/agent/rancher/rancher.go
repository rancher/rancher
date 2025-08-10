package rancher

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"sync"

	"github.com/rancher/rancher/pkg/agent/cluster"
	"github.com/rancher/rancher/pkg/controllers/managementuser/cavalidator"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/rancher"
	"github.com/rancher/wrangler/v3/pkg/apply"
	corefactory "github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/kubeconfig"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	started bool
)

func Run(ctx context.Context) error {
	logrus.Infof("DEBUG: rancher.Run() called")

	if err := setupSteveAggregation(ctx); err != nil {
		logrus.Errorf("DEBUG: setupSteveAggregation failed: %v", err)
		return err
	}
	logrus.Infof("DEBUG: setupSteveAggregation completed")

	if started {
		logrus.Infof("DEBUG: rancher already started, returning")
		return nil
	}

	if !features.MCMAgent.Enabled() {
		logrus.Infof("DEBUG: MCMAgent feature not enabled, returning")
		return nil
	}

	cfg, err := kubeconfig.GetNonInteractiveClientConfig("").ClientConfig()
	if err != nil {
		logrus.Errorf("DEBUG: Failed to get client config: %v", err)
		return err
	}
	logrus.Infof("DEBUG: Got client config successfully")

	core, err := corefactory.NewFactoryFromConfig(cfg)
	if err != nil {
		logrus.Errorf("DEBUG: Failed to create core factory: %v", err)
		return err
	}
	logrus.Infof("DEBUG: Created core factory successfully")

	h := handler{
		ctx:          ctx,
		serviceCache: core.Core().V1().Service().Cache(),
	}

	core.Core().V1().Service().OnChange(ctx, "rancher-installed", h.OnChange)
	if err := core.Start(ctx, 1); err != nil {
		logrus.Errorf("DEBUG: Failed to start core factory: %v", err)
		return err
	}
	logrus.Infof("DEBUG: Started core factory successfully")

	started = true
	logrus.Infof("DEBUG: rancher.Run() completed successfully, started=true")
	return nil
}

type handler struct {
	lock            sync.Mutex
	ctx             context.Context
	rancherNotFound *bool
	serviceCache    corecontrollers.ServiceCache
}

func (h *handler) startRancher() {
	logrus.Infof("DEBUG: startRancher() called")

	if features.ProvisioningPreBootstrap.Enabled() {
		logrus.Debugf("not starting embedded rancher due to pre-bootstrap...")
		return
	}

	clientConfig := kubeconfig.GetNonInteractiveClientConfig("")
	logrus.Infof("DEBUG: Creating embedded Rancher server with ports 80 and 443")
	server, err := rancher.New(h.ctx, clientConfig, &rancher.Options{
		HTTPListenPort:  80,
		HTTPSListenPort: 443,
		Features:        os.Getenv("CATTLE_FEATURES"),
		AddLocal:        "true",
		ClusterRegistry: os.Getenv("CATTLE_CLUSTER_REGISTRY"),
	})
	if err != nil {
		logrus.Fatalf("Embedded rancher failed to initialize: %v", err)
	}
	logrus.Infof("DEBUG: Embedded Rancher server created successfully")

	go func() {
		logrus.Infof("DEBUG: Starting embedded Rancher server on ports 80 and 443")

		// Start the server with the original handler
		err = server.ListenAndServe(h.ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				// context cancellation would happen when cancel() corresponding to h.ctx gets called;
				// since h.ctx is a signal context registered for SIGINT and SIGTERM, cancel() would be called upon
				// receiving one of these signals
				logrus.Infof("Embedded rancher exited due to context cancellation: %v", err)
			} else {
				logrus.Fatalf("Embedded rancher failed to start or exited abnormally: %v", err)
			}
		}
	}()
	logrus.Infof("DEBUG: Embedded Rancher server started in background")

	// Log that we're ready to handle HTTP requests
	logrus.Infof("DEBUG: Ready to handle HTTP requests through the embedded server")
}

func (h *handler) OnChange(key string, service *corev1.Service) (*corev1.Service, error) {
	h.lock.Lock()
	defer h.lock.Unlock()
	if h.rancherNotFound == nil {
		_, err := h.serviceCache.Get(namespace.System, "rancher")
		if notFound := apierror.IsNotFound(err); notFound {
			h.rancherNotFound = &notFound
			h.startRancher()
		} else if err != nil {
			return nil, err
		} else {
			h.rancherNotFound = &notFound
		}
	}

	if service == nil {
		if key == namespace.System+"/rancher" {
			logrus.Info("Rancher has been uninstalled, restarting")
			os.Exit(0)
		}
	} else if service.Namespace == namespace.System && service.Name == "rancher" && *h.rancherNotFound {
		logrus.Info("Rancher has been installed, restarting")
		os.Exit(0)
	}

	return service, nil
}

func setupSteveAggregation(ctx context.Context) error {
	logrus.Infof("DEBUG: setupSteveAggregation called")

	cfg, err := kubeconfig.GetNonInteractiveClientConfig("").ClientConfig()
	if err != nil {
		logrus.Errorf("DEBUG: setupSteveAggregation - failed to get client config: %v", err)
		return err
	}
	logrus.Infof("DEBUG: setupSteveAggregation - got client config")

	_, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		logrus.Errorf("DEBUG: setupSteveAggregation - failed to create kubernetes client: %v", err)
		return err
	}
	logrus.Infof("DEBUG: setupSteveAggregation - created kubernetes client")

	apply, err := apply.NewForConfig(cfg)
	if err != nil {
		return err
	}

	token, url, err := cluster.TokenAndURL()
	if err != nil {
		return err
	}

	data := map[string][]byte{
		"CATTLE_SERVER":      []byte(url),
		"CATTLE_TOKEN":       []byte(token),
		"CATTLE_CA_CHECKSUM": []byte(cluster.CAChecksum()),
		"url":                []byte(url + "/v3/connect"),
		"token":              []byte("stv-cluster-" + token),
	}

	ca, err := ioutil.ReadFile("/etc/kubernetes/ssl/certs/serverca")
	if os.IsNotExist(err) {
	} else if err != nil {
		return err
	} else {
		data["ca.crt"] = ca
	}

	if ctx.Value(cavalidator.CacertsValid).(bool) {
		data[cavalidator.CacertsValid] = []byte("true")
	} else {
		data[cavalidator.CacertsValid] = []byte("false")
	}

	logrus.Infof("DEBUG: Applying Steve aggregation secret")
	err = apply.
		WithDynamicLookup().
		WithSetID("rancher-stv-aggregation").
		WithListerNamespace(namespace.System).
		ApplyObjects(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace.System,
				Name:      "stv-aggregation",
			},
			Data: data,
		})
	if err != nil {
		logrus.Errorf("DEBUG: Failed to apply Steve aggregation secret: %v", err)
		return err
	}
	logrus.Infof("DEBUG: Steve aggregation secret applied successfully")

	logrus.Infof("DEBUG: setupSteveAggregation completed successfully")
	return nil
}
