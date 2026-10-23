package common

import (
	"testing"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func newAuthConfigLister(authConfigs ...*apiv3.AuthConfig) *fakes.AuthConfigListerMock {
	return &fakes.AuthConfigListerMock{
		GetFunc: func(_ string, name string) (*apiv3.AuthConfig, error) {
			for _, authConfig := range authConfigs {
				if authConfig.Name == name {
					return authConfig, nil
				}
			}
			return nil, apierrors.NewNotFound(apiv3.Resource("authconfig"), name)
		},
		ListFunc: func(_ string, _ labels.Selector) ([]*apiv3.AuthConfig, error) {
			return authConfigs, nil
		},
	}
}

func TestLookupProviderNameForConfig(t *testing.T) {
	lister := newAuthConfigLister(
		&apiv3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "github-eu"}, Type: client.GithubConfigType},
		&apiv3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "corp-ad"}, Type: client.ActiveDirectoryConfigType},
	)

	for configName, want := range map[string]string{
		"github-eu": "github",
		"corp-ad":   "activedirectory",
		// The local config is never looked up.
		"local": "local",
	} {
		got, err := LookupProviderNameForConfig(lister, configName)
		require.NoError(t, err, configName)
		assert.Equal(t, want, got, configName)
	}

	_, err := LookupProviderNameForConfig(lister, "missing")
	assert.True(t, apierrors.IsNotFound(err), "expected a not found error, got %v", err)
}

func TestConfigRefs(t *testing.T) {
	lister := newAuthConfigLister(
		&apiv3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "local"}, Type: client.LocalConfigType},
		&apiv3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "github-eu"}, Type: client.GithubConfigType},
	)

	refs, err := ConfigRefs(lister)
	require.NoError(t, err)
	assert.Equal(t, []ConfigRef{
		{ConfigName: "local", ProviderName: "local"},
		{ConfigName: "github-eu", ProviderName: "github"},
	}, refs)
}
