package deferRegistration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/rancher/tests/controllers/common"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// Note: it is not currently possible to test the DeferRegistration function directly.
// This is because it will attempt to start the provided wrangler context factory,
// which references CRDs that do not exist in this test. The only difference between
// DeferFunc and DeferRegistration is the additional call to the wrangler factory,
// so only testing DeferFunc will cover the core behavior of defer.go that we're
// interested in.

type DeferredRegistrationSuite struct {
	suite.Suite

	testEnv *envtest.Environment
	rest    *rest.Config
}

func (d *DeferredRegistrationSuite) SetupSuite() {
	d.testEnv = &envtest.Environment{}

	restCfg, err := d.testEnv.Start()
	require.NoError(d.T(), err)
	require.NotNil(d.T(), restCfg)

	d.rest = restCfg
}

func (d *DeferredRegistrationSuite) TearDownSuite() {
	err := d.testEnv.Stop()
	require.NoError(d.T(), err)
}

func (d *DeferredRegistrationSuite) createClients(ctx context.Context) *wrangler.Context {
	wranglerContext, err := wrangler.NewContext(ctx, nil, d.rest)
	require.NoError(d.T(), err)
	return wranglerContext
}

func (d *DeferredRegistrationSuite) startClients(w *wrangler.Context) {
	namespaceGVK := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "namespace",
	}

	common.StartWranglerControllers(d.T().Context(), d.T(), w, namespaceGVK)
	common.StartWranglerCaches(d.T().Context(), d.T(), w, namespaceGVK)
}

func Test_DeferFunc(t *testing.T) {
	suite.Run(t, new(DeferredRegistrationSuite))
}

const (
	triggerNamespace = "trigger"
)

func newTestNamespaces(test string) (string, string) {
	return triggerNamespace + test, "someothernamespace" + test
}

type testDeferContext struct {
	desiredNamespace string
}

type testDeferInitializer struct {
	*wrangler.BaseInitializer[*testDeferContext]
}

func (t testDeferInitializer) OnChange(ctx context.Context, client *wrangler.Context) {
	client.Core.Namespace().OnChange(ctx, "deferred-namespace-test", func(_ string, namespace *corev1.Namespace) (*corev1.Namespace, error) {
		if strings.Contains(namespace.Name, triggerNamespace) && namespace.ObjectMeta.DeletionTimestamp == nil {
			t.SetClientContext(&testDeferContext{
				desiredNamespace: namespace.ObjectMeta.Name,
			})
		}
		return namespace, nil
	})
}

func newTestDeferInitializer() *testDeferInitializer {
	return &testDeferInitializer{
		BaseInitializer: wrangler.NewBaseInitializer[*testDeferContext](),
	}
}

func (d *DeferredRegistrationSuite) TestDeferFunc() {
	testCtx, testCtxCancel := context.WithCancel(d.T().Context())
	defer testCtxCancel()

	clients := d.createClients(testCtx)
	triggerNS, otherNS := newTestNamespaces("testdeferfunc")
	defer func() {
		require.NoError(d.T(), clients.Core.Namespace().Delete(triggerNS, &metav1.DeleteOptions{}))
		require.NoError(d.T(), clients.Core.Namespace().Delete(otherNS, &metav1.DeleteOptions{}))
	}()

	testDefer := wrangler.NewDeferredRegistration[*testDeferContext, *testDeferInitializer](clients, newTestDeferInitializer(), "test-deferred")
	testDefer.Manage(d.T().Context())

	// need to start the factory after the deferred initializer's
	// onChange handler has been registered in order for it to be picked up
	d.startClients(clients)

	// queue up some deferred functions
	count := 0
	testDefer.DeferFunc(func(clients *testDeferContext) {
		require.Equal(d.T(), triggerNS, clients.desiredNamespace)
		count++
	})
	testDefer.DeferFunc(func(clients *testDeferContext) {
		require.Equal(d.T(), triggerNS, clients.desiredNamespace)
		count++
	})
	testDefer.DeferFunc(func(clients *testDeferContext) {
		require.Equal(d.T(), triggerNS, clients.desiredNamespace)
		count++
	})

	// ensure unrelated namespace creation does not trigger functions
	_, err := clients.Core.Namespace().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: otherNS,
		},
	})
	require.NoError(d.T(), err)
	time.Sleep(time.Millisecond * 100)
	require.Equal(d.T(), 0, count)

	// trigger execution of deferred functions
	_, err = clients.Core.Namespace().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: triggerNS,
		},
	})
	require.NoError(d.T(), err)

	// wait a bit to allow deferred functions to execute
	time.Sleep(time.Millisecond * 100)
	require.Equal(d.T(), 3, count)
}

func (d *DeferredRegistrationSuite) TestDeferFuncWithError() {
	testCtx, testCtxCancel := context.WithCancel(d.T().Context())
	defer testCtxCancel()

	clients := d.createClients(testCtx)
	triggerNS, otherNS := newTestNamespaces("testdeferwitherror")
	defer func() {
		require.NoError(d.T(), clients.Core.Namespace().Delete(triggerNS, &metav1.DeleteOptions{}))
		require.NoError(d.T(), clients.Core.Namespace().Delete(otherNS, &metav1.DeleteOptions{}))
	}()

	testDefer := wrangler.NewDeferredRegistration[*testDeferContext, *testDeferInitializer](clients, newTestDeferInitializer(), "test-deferred")
	testDefer.Manage(d.T().Context())

	// need to start the factory after the deferred initializer's
	// onChange handler has been registered in order for it to be picked up
	d.startClients(clients)

	// queue up some deferred functions
	count := 0
	errCount := 0
	err1 := testDefer.DeferFuncWithError(func(clients *testDeferContext) error {
		require.Equal(d.T(), triggerNS, clients.desiredNamespace)
		count++
		return nil
	})

	err2 := testDefer.DeferFuncWithError(func(clients *testDeferContext) error {
		require.Equal(d.T(), triggerNS, clients.desiredNamespace)
		errCount++
		return fmt.Errorf("fake error")
	})

	err3 := testDefer.DeferFuncWithError(func(clients *testDeferContext) error {
		require.Equal(d.T(), triggerNS, clients.desiredNamespace)
		count++
		return nil
	})

	// ensure unrelated namespace creation does not trigger functions
	_, err := clients.Core.Namespace().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: otherNS,
		},
	})
	require.NoError(d.T(), err)
	time.Sleep(time.Millisecond * 100)
	require.Equal(d.T(), 0, count)
	require.Equal(d.T(), 0, errCount)

	// trigger execution of deferred functions
	_, err = clients.Core.Namespace().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: triggerNS,
		},
	})
	require.NoError(d.T(), err)

	errOut1, timedOut := getFromChanOrTimeout(err1)
	require.False(d.T(), timedOut)
	require.NoError(d.T(), errOut1)

	errOut2, timedOut := getFromChanOrTimeout(err2)
	require.False(d.T(), timedOut)
	require.Error(d.T(), errOut2)

	errOut3, timedOut := getFromChanOrTimeout(err3)
	require.False(d.T(), timedOut)
	require.NoError(d.T(), errOut3)

	require.Equal(d.T(), 2, count)
	require.Equal(d.T(), 1, errCount)
}

func getFromChanOrTimeout(errs chan error) (error, bool) {
	timeout := time.NewTicker(time.Second * 10)
	defer timeout.Stop()
	select {
	case <-timeout.C:
		return nil, true
	case err := <-errs:
		return err, false
	}
}
