package samlconfig

import (
	"context"
	"fmt"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/saml"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type authProvider struct {
	authConfigs v3.AuthConfigInterface
	secrets     corev1.SecretInterface
}

func Register(ctx context.Context, apiContext *config.ScaledContext) {
	a := newAuthProvider(apiContext)
	apiContext.Management.AuthConfigs("").AddHandler(ctx, "authConfigController", a.sync)
}

func newAuthProvider(apiContext *config.ScaledContext) *authProvider {
	a := &authProvider{
		authConfigs: apiContext.Management.AuthConfigs(""),
		secrets:     apiContext.Core.Secrets(""),
	}
	return a
}

func (a *authProvider) sync(key string, config *v3.AuthConfig) (runtime.Object, error) {
	samlConfig := &v32.SamlConfig{}
	if key == "" || config == nil {
		return nil, nil
	}

	if config.Name != saml.PingName && config.Name != saml.ADFSName && config.Name != saml.KeyCloakName &&
		config.Name != saml.OKTAName && config.Name != saml.ShibbolethName {
		return nil, nil
	}

	if !config.Enabled {
		return nil, nil
	}

	authConfigObj, err := a.authConfigs.ObjectClient().UnstructuredClient().Get(config.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve SamlConfig, error: %v", err)
	}

	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("failed to retrieve SamlConfig, cannot read k8s Unstructured data")
	}
	storedSamlConfigMap := u.UnstructuredContent()
	err = common.Decode(storedSamlConfigMap, samlConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to decode Azure Config: %w", err)
	}

	if samlConfig.SpKey != "" {
		value, err := common.ReadFromSecret(a.secrets, samlConfig.SpKey, "spkey")

		if err != nil {
			return nil, err
		}
		samlConfig.SpKey = value
	}

	return nil, saml.InitializeSamlServiceProvider(samlConfig, config.Name)
}
