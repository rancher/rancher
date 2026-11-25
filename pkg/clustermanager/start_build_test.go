package clustermanager

import (
	"context"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rancher/lasso/pkg/controller"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	corefakes "github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	mgmtfakes "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/rancher/rancher/pkg/types/config"
	dialermocks "github.com/rancher/rancher/pkg/types/config/dialer/mocks"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"golang.org/x/sync/semaphore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

func TestStartBuildFailureKeepsTheReplacement(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(http.NotFound))
	defer server.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	cluster := newMovedTestCluster()
	cluster.Status.APIEndpoint = server.URL
	cluster.Status.CACert = base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{
		Type: "CERTIFICATE", Bytes: server.Certificate().Raw,
	}))
	cluster.Status.ServiceAccountTokenSecret = "cluster-token"
	apimgmtv3.ClusterConditionServiceAccountSecretsMigrated.True(cluster)

	managementConfig := rest.Config{Host: server.URL}
	factory, err := controller.NewSharedControllerFactoryFromConfig(&managementConfig, wrangler.Scheme)
	require.NoError(t, err)
	building := make(chan struct{})
	resume := make(chan struct{})
	resumeBuild := sync.OnceFunc(func() { close(resume) })
	defer resumeBuild()

	secretErr := errors.New("cluster token secret lookup failed")
	dialFactory := dialermocks.NewMockFactory(gomock.NewController(t))
	dialFactory.EXPECT().ClusterDialer(cluster.Name, true).Return(nil, nil).MinTimes(2)
	var lookupStarted atomic.Bool
	m := &Manager{
		ScaledContext: &config.ScaledContext{
			RESTConfig:        managementConfig,
			ControllerFactory: factory,
			Dialer:            dialFactory,
			RunContext:        ctx,
		},
		clusterLister: &mgmtfakes.ClusterListerMock{
			GetFunc: func(string, string) (*apimgmtv3.Cluster, error) { return cluster.DeepCopy(), nil },
		},
		secretLister: &corefakes.SecretListerMock{
			GetFunc: func(string, string) (*corev1.Secret, error) {
				if lookupStarted.CompareAndSwap(false, true) {
					close(building)
					<-resume
					return nil, secretErr
				}
				return &corev1.Secret{}, nil
			},
		},
		clusters: newTestClusters(cluster),
		// Hold the replacement in Starting until the assertions are done.
		startSem: semaphore.NewWeighted(0),
	}
	defer m.Stop(cluster)

	startErr := make(chan error, 1)
	go func() { startErr <- m.Start(ctx, cluster, true) }()
	select {
	case <-building:
	case <-ctx.Done():
		t.Fatal("start did not reach the secret lookup")
	}

	// A client request installs a record while the first controller start is still building one.
	userContext, err := m.UserContext(cluster.Name)
	require.NoError(t, err)
	require.NoError(t, m.Start(ctx, cluster, true))
	starting := ControllersState{Phase: ControllersStarting, Owner: true}
	require.Equal(t, starting, m.ControllersState(cluster))

	resumeBuild()
	select {
	case err := <-startErr:
		require.ErrorIs(t, err, secretErr)
	case <-ctx.Done():
		t.Fatal("start did not return the secret lookup error")
	}

	assert.Equal(t, starting, m.ControllersState(cluster), "the failed build must leave the replacement starting")
	current, err := m.UserContext(cluster.Name)
	require.NoError(t, err)
	assert.Same(t, userContext, current, "client requests must still use the replacement record")
}
