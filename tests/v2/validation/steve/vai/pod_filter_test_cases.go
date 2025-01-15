package vai

import (
	"fmt"
	"net/url"

	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type podFilterTestCase struct {
	name             string
	createPods       func() ([]v1.Pod, []string, []string, []string)
	filter           func(namespaces []string) url.Values
	expectFound      bool
	supportedWithVai bool
}

func (p podFilterTestCase) SupportedWithVai() bool {
	return p.supportedWithVai
}

// Helper function to create a pod
func createPod(name, namespace, image string) v1.Pod {
	return v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{Name: image, Image: image}},
		},
	}
}

var podFilterTestCases = []podFilterTestCase{
	{
		name: "Filter by nginx image",
		createPods: func() ([]v1.Pod, []string, []string, []string) {
			suffix := namegen.RandStringLower(randomStringLength)
			ns1 := fmt.Sprintf("namespace1-%s", suffix)
			ns2 := fmt.Sprintf("namespace2-%s", suffix)
			name1 := fmt.Sprintf("nginx-pod-%s", namegen.RandStringLower(randomStringLength))
			name2 := fmt.Sprintf("busybox-pod-%s", namegen.RandStringLower(randomStringLength))
			name3 := fmt.Sprintf("alpine-pod-%s", namegen.RandStringLower(randomStringLength))

			pods := []v1.Pod{
				createPod(name1, ns1, "nginx"),
				createPod(name2, ns2, "busybox"),
				createPod(name3, ns1, "alpine"),
			}

			expectedNames := []string{name1}
			allNamespaces := []string{ns1, ns2}
			expectedNamespaces := []string{ns1, ns2}

			return pods, expectedNames, allNamespaces, expectedNamespaces
		},
		filter: func(namespaces []string) url.Values {
			return url.Values{
				"filter":               []string{"spec.containers.image=nginx"},
				"projectsornamespaces": namespaces,
			}
		},
		expectFound:      true,
		supportedWithVai: true,
	},
	{
		name: "Filter by busybox image",
		createPods: func() ([]v1.Pod, []string, []string, []string) {
			suffix := namegen.RandStringLower(randomStringLength)
			ns1 := fmt.Sprintf("namespace1-%s", suffix)
			ns2 := fmt.Sprintf("namespace2-%s", suffix)
			name1 := fmt.Sprintf("nginx-pod-%s", namegen.RandStringLower(randomStringLength))
			name2 := fmt.Sprintf("busybox-pod-%s", namegen.RandStringLower(randomStringLength))
			name3 := fmt.Sprintf("alpine-pod-%s", namegen.RandStringLower(randomStringLength))

			pods := []v1.Pod{
				createPod(name1, ns1, "nginx"),
				createPod(name2, ns2, "busybox"),
				createPod(name3, ns1, "alpine"),
			}

			expectedNames := []string{name2}
			allNamespaces := []string{ns1, ns2}
			expectedNamespaces := []string{ns2}

			return pods, expectedNames, allNamespaces, expectedNamespaces
		},
		filter: func(namespaces []string) url.Values {
			return url.Values{
				"filter":               []string{"spec.containers.image=busybox"},
				"projectsornamespaces": namespaces,
			}
		},
		expectFound:      true,
		supportedWithVai: true,
	},
	{
		name: "Filter by non-existent image",
		createPods: func() ([]v1.Pod, []string, []string, []string) {
			suffix := namegen.RandStringLower(randomStringLength)
			ns1 := fmt.Sprintf("namespace1-%s", suffix)
			ns2 := fmt.Sprintf("namespace2-%s", suffix)
			name1 := fmt.Sprintf("nginx-pod-%s", namegen.RandStringLower(randomStringLength))
			name2 := fmt.Sprintf("busybox-pod-%s", namegen.RandStringLower(randomStringLength))
			name3 := fmt.Sprintf("alpine-pod-%s", namegen.RandStringLower(randomStringLength))

			pods := []v1.Pod{
				createPod(name1, ns1, "nginx"),
				createPod(name2, ns2, "busybox"),
				createPod(name3, ns1, "alpine"),
			}

			// Add NodeName to pods
			pods[0].Spec.NodeName = "node1"
			pods[1].Spec.NodeName = "node2"
			pods[2].Spec.NodeName = "node1"

			var expectedNames []string
			allNamespaces := []string{ns1, ns2}
			expectedNamespaces := []string{ns1, ns2}

			return pods, expectedNames, allNamespaces, expectedNamespaces
		},
		filter: func(namespaces []string) url.Values {
			return url.Values{
				"filter":               []string{"spec.containers.image=redis"},
				"projectsornamespaces": namespaces,
			}
		},
		expectFound:      false,
		supportedWithVai: true,
	},
}
