package feature

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("harvester-baremetal-container-workload Feature", func() {
	feature := v3.Feature{
		TypeMeta: v1.TypeMeta{
			Kind:       "features.management.cattle.io",
			APIVersion: "v3",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "harvester-baremetal-container-workload",
		},
	}
	When("test1", func() {
		BeforeEach(func() {
			//f, err := wranglerContext.Mgmt.Feature().Create(&feature)
			f := v3.Feature{}
			err := sharedController.Client().Create(ctx, "", &feature, &f, v1.CreateOptions{})
			Expect(f).ToNot(BeNil())
			fmt.Printf("%v", f)
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() {
				err := wranglerContext.Mgmt.Feature().Delete("harvester-baremetal-container-workload", &v1.DeleteOptions{})
				Expect(err).ToNot(HaveOccurred())
			})
		})

		It("adds annotation", func() {
			f := &v3.Feature{}
			//Eventually(func() bool {
			//err := k8sClient.Get(ctx, types.NamespacedName{Namespace: "test-namespace", Name: "harvester-baremetal-container-workload"}, &f)
			//var err error
			f, _ = wranglerContext.Mgmt.Feature().Get("harvester-baremetal-container-workload", v1.GetOptions{})
			//return err == nil
			//}).Should(BeTrue())

			Expect(f.Annotations[v3.ExperimentalFeatureKey]).To(Equal(v3.ExperimentalFeatureValue))
		})
	})
})
