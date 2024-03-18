package feature_test

import (
	"fmt"

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
	When("test1", func() {
		BeforeEach(func() {
			f := v3.Feature{}
			err := sharedController.Client().Create(ctx, "", &feature, &f, v1.CreateOptions{})
			Expect(f).ToNot(BeNil())
			fmt.Printf("%v\n", f)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := sharedController.Client().Delete(ctx, "", "harvester-baremetal-container-workload", v1.DeleteOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("adds annotation", func() {
			f := v3.Feature{}
			// test currently fails because the feature handler's featureClient isn't updating the feature correctly
			// TODO figure out why
			err := sharedController.Client().Get(ctx, "", "harvester-baremetal-container-workload", &f, v1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			fmt.Printf("%v\n", f)

			Expect(f.Annotations[v3.ExperimentalFeatureKey]).To(Equal(v3.ExperimentalFeatureValue))
		})
	})
})
