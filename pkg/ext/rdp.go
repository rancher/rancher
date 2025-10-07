package ext

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/remotedialer-proxy/forward"
	"github.com/rancher/remotedialer-proxy/proxyclient"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/randomtoken"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
)

const (
	apiExtSecretName = "api-extension"
	rdpEnabledEnvVar = "IMPERATIVE_API_DIRECT"
	certSecretName   = "api-extension-ca-name"
	certServerName   = "api-extension-tls-name"
)

var rdpSecretBackoff = wait.Backoff{
	Steps:    7,
	Duration: 15 * time.Second,
	Factor:   2.0,
	Jitter:   0.1,
}

func RDPStart(ctx context.Context, restConfig *rest.Config, wranglerContext *wrangler.Context) error {
	if features.MCMAgent.Enabled() {
		return nil
	}

	portForwarder, err := forward.New(
		restConfig,
		wranglerContext.Core.Pod(),
		namespace.System,
		"app=api-extension",
		[]string{"5555:5555"})

	if err != nil {
		return err
	}

	var connectSecret string
	var retryErr error
	retryCount := 0

	// Retry to get or create the connect secret for approx 15 minutes
	retry.OnError(rdpSecretBackoff, func(err error) bool {
		retryCount++
		logrus.Errorf("RDPClient: error getting connect secret (retry #%d), will retry: %s", retryCount, err.Error())
		return true
	}, func() error {
		connectSecret, retryErr = GetOrCreateRDPConnectSecret(wranglerContext.Core.Secret())
		return retryErr
	})

	if retryErr != nil {
		return retryErr
	}

	remoteDialerProxyClient, err := proxyclient.New(
		ctx,
		connectSecret,
		namespace.System,
		certSecretName,
		certServerName,
		wranglerContext.Core.Secret(),
		portForwarder)

	if err != nil {
		return err
	}

	remoteDialerProxyClient.Run(ctx)

	return nil
}

func RDPEnabled() bool {
	return os.Getenv(rdpEnabledEnvVar) != "true"
}

func GetOrCreateRDPConnectSecret(secretController corecontrollers.SecretController) (string, error) {
	secret, err := secretController.Get(namespace.System, apiExtSecretName, v1.GetOptions{})
	if secret != nil && err == nil {
		return string(secret.Data["data"]), nil
	}

	if secret == nil {
		logrus.Warnf("RDPClient: couldn't read connect secret, will attempt to create new one...")
	}

	if err != nil {
		logrus.Errorf("RDPClient: error reading connect secret: %s, will attempt to create new one...", err.Error())
	}

	secretValue, err := randomtoken.Generate()
	if err != nil {
		return "", fmt.Errorf("RDPClient: connect secret generation failed: %s", err.Error())
	}

	if _, err := secretController.Create(&corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      apiExtSecretName,
			Namespace: namespace.System,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"data": secretValue,
		},
	}); err != nil {
		logrus.Errorf("RDPClient: error creating connect secret: %s", err.Error())
		return "", err
	}

	return secretValue, nil
}
