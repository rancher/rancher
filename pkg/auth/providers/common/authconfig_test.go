package common

import (
	"testing"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/authconfigtest"
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
		&apiv3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "localsaml"}, Type: client.OKTAConfigType},
	)

	for configName, want := range map[string]string{
		"github-eu": "github",
		"corp-ad":   "activedirectory",
		// A config named like local is looked up.
		"localsaml": "okta",
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

func TestUpdateAuthConfig(t *testing.T) {
	newConfig := func(resourceVersion string) *apiv3.AuthConfig {
		return &apiv3.AuthConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "github", ResourceVersion: resourceVersion},
			Type:       client.GithubConfigType,
		}
	}

	t.Run("updates without reading the AuthConfig again", func(t *testing.T) {
		srv := authconfigtest.NewUpdateServer(t, map[string]any{"metadata": map[string]any{"name": "github"}})
		config := newConfig("1")

		err := UpdateAuthConfig(srv.Client(), config, &config.ObjectMeta, func() (metav1.ObjectMeta, error) {
			t.Fatal("the AuthConfig should not be read again without a conflict")
			return metav1.ObjectMeta{}, nil
		})
		require.NoError(t, err)

		assert.Equal(t, 1, srv.Updates())
		assert.Equal(t, client.GithubConfigType, srv.Stored()["type"])
	})

	t.Run("retries with the latest metadata on conflict", func(t *testing.T) {
		srv := authconfigtest.NewUpdateServer(t, map[string]any{"metadata": map[string]any{"name": "github"}})
		srv.WriteAfterGets(1)
		storedConfig, err := srv.Client().Get("github", metav1.GetOptions{})
		require.NoError(t, err)
		config := newConfig(storedConfig.ResourceVersion)

		err = UpdateAuthConfig(srv.Client(), config, &config.ObjectMeta, func() (metav1.ObjectMeta, error) {
			storedConfig, err := srv.Client().Get("github", metav1.GetOptions{})
			if err != nil {
				return metav1.ObjectMeta{}, err
			}
			return storedConfig.ObjectMeta, nil
		})
		require.NoError(t, err)

		authconfigtest.RequireUpdatedAfterConflict(t, srv)
		assert.Equal(t, client.GithubConfigType, srv.Stored()["type"])
	})

	t.Run("returns an error reading the AuthConfig", func(t *testing.T) {
		srv := authconfigtest.NewUpdateServer(t, map[string]any{"metadata": map[string]any{"name": "github"}})
		config := newConfig("0")

		err := UpdateAuthConfig(srv.Client(), config, &config.ObjectMeta, func() (metav1.ObjectMeta, error) {
			return metav1.ObjectMeta{}, assert.AnError
		})

		require.ErrorIs(t, err, assert.AnError)
		assert.Equal(t, 1, srv.Updates())
	})

	t.Run("does not retry other errors", func(t *testing.T) {
		srv := authconfigtest.NewUpdateServer(t, map[string]any{"metadata": map[string]any{"name": "github"}})
		config := newConfig("1")
		config.Name = "missing"

		err := UpdateAuthConfig(srv.Client(), config, &config.ObjectMeta, func() (metav1.ObjectMeta, error) {
			t.Fatal("the AuthConfig should not be read again without a conflict")
			return metav1.ObjectMeta{}, nil
		})

		assert.True(t, apierrors.IsNotFound(err), "expected a NotFound error, got %v", err)
		assert.Equal(t, 0, srv.Updates())
	})
}
