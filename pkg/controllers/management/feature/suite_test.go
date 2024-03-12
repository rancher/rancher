package feature

import (
	"context"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/rancher/pkg/wrangler"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	ctx              context.Context
	cancel           context.CancelFunc
	testEnv          *envtest.Environment
	cfg              *rest.Config
	k8sClient        client.Client
	wranglerContext  *wrangler.Context
	sharedController controller.SharedController
)

func TestFeatures(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Feature Suite")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.TODO())

	var err error
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "crds", "yaml", "generated", "management.cattle.io_features.yaml")},
		ErrorIfCRDPathMissing: true,
	}
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sClient, err = client.New(cfg, client.Options{})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// Create the clientConfig for the wrangler context
	config := clientcmdapi.NewConfig()
	clientCfg := clientcmd.NewDefaultClientConfig(*config, nil)

	// wrangler context needs to provide:
	//   featuresClient:   		wContext.Mgmt.Feature()
	//   tokensLister:     		wContext.Mgmt.Token().Cache()
	//   tokenEnqueue:			wContext.Mgmt.Token().EnqueueAfter
	//   nodeDriverController:	wContext.Mgmt.NodeDriver()
	wranglerContext, err = wrangler.NewContext(ctx, clientCfg, cfg)
	Expect(err).NotTo(HaveOccurred())

	/*
		d, err := discovery.NewDiscoveryClientForConfig(cfg)
		Expect(err).ToNot(HaveOccurred())
		disc := memory.NewMemCacheClient(d)
		mapper := restmapper.NewDeferredDiscoveryRESTMapper(disc)
		cf, err := lassoclient.NewSharedClientFactory(cfg, &lassoclient.SharedClientFactoryOptions{
			Mapper: mapper,
		})
		Expect(err).ToNot(HaveOccurred())

		sharedFactory := controller.NewSharedControllerFactory(cache.NewSharedCachedFactory(cf, &cache.SharedCacheFactoryOptions{
			DefaultResync: time.Minute * 30,
		}), nil)

		opts := &generic.FactoryOptions{
			SharedControllerFactory: sharedFactory,
		}

		mgmt, err := management.NewFactoryFromConfigWithOptions(cfg, opts)
		Expect(err).ToNot(HaveOccurred())

		wranglerContext = &wrangler.Context{
			Mgmt: mgmt.Management().V3(),
		}
	*/

	Register(ctx, wranglerContext)
	sharedController, err = wranglerContext.ControllerFactory.ForKind(schema.GroupVersionKind{
		Group:   "management.cattle.io",
		Version: "v3",
		Kind:    "Feature",
	})
	Expect(err).ToNot(HaveOccurred())
	sharedController.Start(ctx, 1)
	//err = wranglerContext.Start(ctx)
	//Expect(err).ToNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	cancel()
	Expect(testEnv.Stop()).ToNot(HaveOccurred())
})
