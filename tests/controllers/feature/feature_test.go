package feature_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("harvester-baremetal-container-workload Feature", func() {
	feature := v3.Feature{
		ObjectMeta: v1.ObjectMeta{
			Name: "harvester-baremetal-container-workload",
		},
	}
	When("harvester feature is created", func() {
		BeforeEach(func() {
			f1, err := wranglerContext.Mgmt.Feature().Create(&feature)
			Expect(f1).ToNot(BeNil())
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := wranglerContext.Mgmt.Feature().Delete("harvester-baremetal-container-workload", &v1.DeleteOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("adds experimental feature annotation", func() {
			Eventually(func() string {
				f, err := wranglerContext.Mgmt.Feature().Get("harvester-baremetal-container-workload", v1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return f.Annotations[v3.ExperimentalFeatureKey]
			}).Should(Equal(v3.ExperimentalFeatureValue))
		})
	})
})
