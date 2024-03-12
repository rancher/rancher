package globalroles

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	ctx     context.Context
	cancel  context.CancelFunc
	testEnv *envtest.Environment
	cfg     *rest.Config
)

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.TODO())

	var err error
	testEnv = &envtest.Environment{}
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sClient, err := client.New(cfg, client.Options{})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

})

var _ = AfterSuite(func() {
	cancel()
	Expect(testEnv.Stop()).ToNot(HaveOccurred())
})
