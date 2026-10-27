package common

import (
	"fmt"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
)

// localConfigName is the name of the local AuthConfig, and of its provider.
const localConfigName = "local"

// ConfigRef identifies an AuthConfig and the provider that implements it.
type ConfigRef struct {
	ConfigName   string
	ProviderName string
}

// LookupProviderNameForConfig returns the name of the provider that
// implements the named AuthConfig, e.g. "github" for the AuthConfig
// "github-eu", by looking the AuthConfig up in the lister. The local config
// always maps to the local provider without a lookup.
//
// It returns an error if the AuthConfig can't be found.
func LookupProviderNameForConfig(lister v3.AuthConfigLister, configName string) (string, error) {
	if configName == localConfigName {
		return localConfigName, nil
	}

	authConfig, err := lister.Get("", configName)
	if err != nil {
		return "", fmt.Errorf("getting auth config '%s': %w", configName, err)
	}

	return ProviderNameFromType(authConfig.Type), nil
}

// ConfigRefs returns every AuthConfig with the provider that implements it.
func ConfigRefs(lister v3.AuthConfigLister) ([]ConfigRef, error) {
	authConfigs, err := lister.List("", labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("listing auth configs: %w", err)
	}

	refs := make([]ConfigRef, len(authConfigs))
	for i, authConfig := range authConfigs {
		refs[i] = ConfigRef{ConfigName: authConfig.Name, ProviderName: ProviderNameFromType(authConfig.Type)}
	}

	return refs, nil
}

// UpdateAuthConfig updates the AuthConfig with obj, where meta is the metadata
// of obj, and is expected to be the metadata of the stored AuthConfig.
//
// The AuthConfig can be written by something else after it was read, so if
// the update conflicts, meta is replaced with the latest metadata from
// getStoredMeta and the update is retried. Anything in obj derived from the
// stored AuthConfig other than its metadata is not refreshed.
func UpdateAuthConfig(authConfigs v3.AuthConfigInterface, obj runtime.Object, meta *metav1.ObjectMeta, getStoredMeta func() (metav1.ObjectMeta, error)) error {
	refresh := false
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if refresh {
			storedMeta, err := getStoredMeta()
			if err != nil {
				return err
			}
			*meta = storedMeta
		}
		refresh = true

		_, err := authConfigs.ObjectClient().Update(meta.Name, obj)
		return err
	})
}
