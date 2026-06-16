package clusterauthtoken

import (
	"testing"

	lassocache "github.com/rancher/lasso/pkg/cache"
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
)

// TestRegisterFactoryDuplicate verifies that calling RegisterFactory twice on
// the same UserContext returns a duplicate-factory error on the second call.
// This confirms the factory is registered in UserContext.extraControllerFactories
// on the first call, enabling UserContext.Start() to start it alongside other
// factories in doStart().
func TestRegisterFactoryDuplicate(t *testing.T) {
	t.Parallel()

	cluster := buildMinimalUserContext(t)
	cache, err := RegisterFactory(cluster)
	require.NoError(t, err)
	assert.NotNil(t, cache)

	_, err = RegisterFactory(cluster)
	require.Error(t, err)
	assert.ErrorContains(t, err, "duplicate")
}

// buildMinimalUserContext constructs the minimum UserContext needed to test
// RegisterFactory. It uses a real lasso controller factory backed by a
// no-network-contact client factory (startContext left nil so
// RegisterExtraControllerFactory does not call factory.Start()).
func buildMinimalUserContext(t *testing.T) *config.UserContext {
	t.Helper()

	cfg := &rest.Config{Host: "http://127.0.0.1:0"}
	clientFactory, err := client.NewSharedClientFactory(cfg, nil)
	require.NoError(t, err)
	cacheFactory := lassocache.NewSharedCachedFactory(clientFactory, nil)
	controllerFactory := controller.NewSharedControllerFactory(cacheFactory, nil)

	return &config.UserContext{
		ClusterName:       "test-cluster",
		ControllerFactory: controllerFactory,
	}
}
