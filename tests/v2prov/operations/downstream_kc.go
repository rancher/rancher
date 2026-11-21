package operations

import (
	"context"
	"fmt"
	"sync"

	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/dashboardapi/settings"
	"github.com/rancher/rancher/pkg/provisioningv2/kubeconfig"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

var (
	settingsRegisterLock sync.Mutex
	settingsRegistered   bool
)

// registerSettings sets the global settings provider, which the kubeconfig manager reads the internal server URL + CA from.
// settings.SetProvider writes an unsynchronized global, so this only registers once per process. Parallel tests and
// failure hooks would otherwise race on it.
func registerSettings(wContext *wrangler.Context) error {
	settingsRegisterLock.Lock()
	defer settingsRegisterLock.Unlock()

	if settingsRegistered {
		return nil
	}
	if err := settings.Register(wContext.Mgmt.Setting()); err != nil {
		return err
	}
	settingsRegistered = true
	return nil
}

func getDownstreamClientset(clients *clients.Clients, c *v1.Cluster) (*kubernetes.Clientset, error) {
	wContext, err := wrangler.NewContext(context.TODO(), clients.ClientConfig, clients.RESTConfig)
	if err != nil {
		return nil, err
	}
	if err := registerSettings(wContext); err != nil {
		return nil, err
	}
	kcManager := kubeconfig.New(wContext)

	// Get kubeconfig for the downstream cluster to create test resources
	restConfig, err := kcManager.GetRESTConfig(c, c.Status)
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(restConfig)
}

func GetAndVerifyDownstreamClientset(clients *clients.Clients, c *v1.Cluster) (*kubernetes.Clientset, error) {
	clientset, err := getDownstreamClientset(clients, c)
	if err != nil {
		return nil, err
	}

	// Try to continuously get the default kubernetes service
	err = retry.OnError(defaults.DownstreamClientsetRetry, func(err error) bool {
		logrus.Warnf("[GetAndVerifyDownstreamClientset] failed to get the default kubernetes service, will retry: %v", err)
		return true
	}, func() error {
		_, err = clientset.CoreV1().Services(corev1.NamespaceDefault).Get(context.TODO(), "kubernetes", metav1.GetOptions{})
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to verify downstream clientset: %w", err)
	}

	return clientset, nil
}
