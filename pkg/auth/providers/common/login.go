package common

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rancher/norman/httperror"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
)

// LoginConfigName returns the name of the AuthConfig that a login for the
// named provider is for.
//
// Logins name the AuthConfig in configName. The login is rejected if that
// AuthConfig does not exist, is disabled, or is not for the provider.
//
// Clients that only send the provider, e.g. older CLIs, have no configName.
// Their logins use the only enabled AuthConfig of the provider. If there is
// none, or more than one, the login is rejected with an error that asks the
// client to send configName. If there are several, the error lists their
// names. Only enabled AuthConfigs are listed, which unauthenticated clients
// can already see in /v3-public/authProviders.
//
// AuthConfigs are read from the cache, so a login can see an AuthConfig as
// enabled or disabled for a few seconds after it changes.
//
// Rejections are returned unwrapped so that they reach the client as a 4xx.
func LoginConfigName(authConfigs v3.AuthConfigLister, configName, providerName string) (string, error) {
	if configName != "" {
		if err := checkLoginConfig(authConfigs, configName, providerName); err != nil {
			return "", err
		}
		return configName, nil
	}

	list, err := authConfigs.List("", labels.Everything())
	if err != nil {
		return "", fmt.Errorf("listing auth configs: %w", err)
	}

	var names []string
	for _, authConfig := range list {
		if authConfig.Enabled && ProviderNameFromType(authConfig.Type) == providerName {
			names = append(names, authConfig.Name)
		}
	}

	switch len(names) {
	case 1:
		return names[0], nil
	case 0:
		return "", httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("no auth config is enabled for provider %s, specify configName", providerName))
	}

	slices.Sort(names)
	return "", httperror.NewAPIError(httperror.InvalidBodyContent,
		fmt.Sprintf("several auth configs are enabled for provider %s, specify configName as one of: %s",
			providerName, strings.Join(names, ", ")))
}

// checkLoginConfig returns an error if the named AuthConfig can't be used for
// a login with the named provider.
//
// A missing, disabled or other provider's AuthConfig gets the same error, so
// that unauthenticated clients can't use logins to find out which AuthConfigs
// exist.
func checkLoginConfig(authConfigs v3.AuthConfigLister, configName, providerName string) error {
	authConfig, err := authConfigs.Get("", configName)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("getting auth config %s: %w", configName, err)
	}

	if err != nil || !authConfig.Enabled || ProviderNameFromType(authConfig.Type) != providerName {
		return httperror.NewAPIError(httperror.Unauthorized, "authentication failed")
	}

	return nil
}
