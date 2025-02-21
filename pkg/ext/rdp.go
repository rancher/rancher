package ext

import (
	"context"
	"fmt"
	"os"

	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/remotedialer/forward"
	"github.com/rancher/remotedialer/proxyclient"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/randomtoken"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	apiExtSecretName = "api-extension"
	rdpEnabledEnvVar = "IMPERATIVE_API_DIRECT"
	certSecretName   = "api-extension-ca-name"
	certServerName   = "api-extension-tls-name"
)

func RDPStart(ctx context.Context, restConfig *rest.Config, wranglerContext *wrangler.Context) error {
	if RDPEnabled() {
		portForwarder, err := forward.New(
			restConfig,
			wranglerContext.Core.Pod(),
			namespace.System,
			"app=api-extension",
			[]string{"5555:5555"})

		if err != nil {
			return err
		}

		connectSecret, err := GetOrCreateRDPConnectSecret(wranglerContext.Core.Secret())
		if err != nil {
			return err
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

	return DeleteRDPConnectSecret(wranglerContext.Core.Secret())
}

func RDPEnabled() bool {
	switch os.Getenv(rdpEnabledEnvVar) {
	case "true":
		return false
	default:
		return true
	}
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
		logrus.Errorf("RDPClient: error reading connect secret: %s, will attempt to create new one...")
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

func DeleteRDPConnectSecret(secretController corecontrollers.SecretController) error {
	err := secretController.Delete(namespace.System, apiExtSecretName, &v1.DeleteOptions{})
	return client.IgnoreNotFound(err)
}
