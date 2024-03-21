package feature_test

import (
	"context"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/feature"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v2/pkg/crd"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	ctx             context.Context
	cancel          context.CancelFunc
	testEnv         *envtest.Environment
	wranglerContext *wrangler.Context
)

func TestFeatures(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Feature Suite")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.TODO())

	// Load CRD from YAML for REST Client
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "pkg", "crds", "yaml", "generated", "management.cattle.io_features.yaml")},
		ErrorIfCRDPathMissing: true,
	}
	restCfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restCfg).NotTo(BeNil())

	// Create feature CRD
	factory, err := crd.NewFactoryFromClient(restCfg)
	Expect(err).ToNot(HaveOccurred())
	err = factory.BatchCreateCRDs(ctx, crd.CRD{
		SchemaObject: v3.Feature{},
		NonNamespace: true,
	}).BatchWait()
	Expect(err).ToNot(HaveOccurred())

	// Create the clientConfig for the wrangler context
	config := clientcmdapi.NewConfig()
	Expect(err).ToNot(HaveOccurred())

	clientCfg := clientcmd.NewDefaultClientConfig(*config, nil)
	Expect(err).ToNot(HaveOccurred())

	wranglerContext, err = wrangler.NewContext(ctx, clientCfg, restCfg)
	Expect(err).NotTo(HaveOccurred())

	sc := wranglerContext.ControllerFactory.ForResourceKind(schema.GroupVersionResource{
		Group:    "management.cattle.io",
		Version:  "v3",
		Resource: "features",
	}, "Feature", false)
	Expect(err).ToNot(HaveOccurred())

	feature.Register(ctx, wranglerContext)
	err = sc.Start(ctx, 1)
	Expect(err).ToNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	cancel()
	Expect(testEnv.Stop()).ToNot(HaveOccurred())
})
