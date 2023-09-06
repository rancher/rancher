package image

import (
	"fmt"
	"os"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	rketypes "github.com/rancher/rke/types"
	assertlib "github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConvertMirroredImages(t *testing.T) {
	testCases := []struct {
		caseName                string
		inputRawImages          map[string]map[string]struct{}
		outputImagesShouldEqual map[string]map[string]struct{}
	}{
		{
			caseName: "normalize images",
			inputRawImages: map[string]map[string]struct{}{
				"rancher/rke-tools:v0.1.48": {"system": struct{}{}},
				"rancher/rke-tools:v0.1.49": {"system": struct{}{}},
				// for mirror
				"prom/prometheus:v2.0.1":                           {"system": struct{}{}},
				"quay.io/coreos/flannel:v1.2.3":                    {"system": struct{}{}},
				"gcr.io/google_containers/k8s-dns-kube-dns:1.15.0": {"system": struct{}{}},
				"test.io/test:v0.0.1":                              {"test": struct{}{}}, // not in mirror list
			},
			outputImagesShouldEqual: map[string]map[string]struct{}{
				"rancher/coreos-flannel:v1.2.3":   {"system": struct{}{}},
				"rancher/k8s-dns-kube-dns:1.15.0": {"system": struct{}{}},
				"rancher/prom-prometheus:v2.0.1":  {"system": struct{}{}},
				"rancher/rke-tools:v0.1.48":       {"system": struct{}{}},
				"rancher/rke-tools:v0.1.49":       {"system": struct{}{}},
				"test.io/test:v0.0.1":             {"test": struct{}{}},
			},
		},
	}

	assert := assertlib.New(t)
	for _, cs := range testCases {
		imagesSet := cs.inputRawImages
		convertMirroredImages(imagesSet)
		assert.Equal(cs.outputImagesShouldEqual, imagesSet)
	}
}

func TestResolveWithCluster(t *testing.T) {
	if os.Getenv("CATTLE_BASE_REGISTRY") != "" {
		fmt.Println("Skipping TestResolveWithCluster. Can't run the tests with CATTLE_BASE_REGISTRY set")
		return
	}

	clusterWithPrivateRegistry := func(s string) *v3.Cluster {
		return &v3.Cluster{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{},
			Spec: apimgmtv3.ClusterSpec{
				ClusterSpecBase: apimgmtv3.ClusterSpecBase{
					RancherKubernetesEngineConfig: &rketypes.RancherKubernetesEngineConfig{
						PrivateRegistries: []rketypes.PrivateRegistry{{
							URL: s,
						},
						},
					},
				},
			},
		}
	}
	type input struct {
		image              string
		CattleBaseRegistry string
		cluster            *v3.Cluster
	}
	tests := []struct {
		name     string
		input    input
		expected string
	}{
		{
			name: "No cluster no default registry",
			input: input{
				image:              "imagename",
				CattleBaseRegistry: "",
				cluster:            nil,
			},
			expected: "imagename",
		},
		{
			name: "No cluster with default registry, image without rancher/",
			input: input{
				image:              "imagename",
				CattleBaseRegistry: "custom-registry",
				cluster:            nil,
			},
			expected: "custom-registry/rancher/imagename",
		},
		{
			name: "No cluster with default registry, image with rancher/",
			input: input{
				image:              "rancher/imagename",
				CattleBaseRegistry: "custom-registry",
				cluster:            nil,
			},
			expected: "custom-registry/rancher/imagename",
		},
		{
			name: "Cluster empty URL, no default registry",
			input: input{
				image:              "imagename",
				CattleBaseRegistry: "",
				cluster:            &v3.Cluster{},
			},
			expected: "imagename",
		},
		{
			name: "Cluster empty URL, with default registry",
			input: input{
				image:              "imagename",
				CattleBaseRegistry: "default-registry.com",
				cluster:            &v3.Cluster{},
			},
			expected: "default-registry.com/rancher/imagename",
		},
		{
			name: "Cluster empty URL, with default registry and rancher on image name",
			input: input{
				image:              "rancher/imagename",
				CattleBaseRegistry: "default-registry.com",
				cluster:            &v3.Cluster{},
			},
			expected: "default-registry.com/rancher/imagename",
		},
		{
			name: "Cluster with URL, no default registry, no rancher/ on image",
			input: input{
				image:              "imagename",
				CattleBaseRegistry: "",
				cluster:            clusterWithPrivateRegistry("cluster-url.com"),
			},
			expected: "cluster-url.com/rancher/imagename",
		},
		{
			name: "Cluster with URL, no default registry, with rancher/ on image",
			input: input{
				image:              "imagename",
				CattleBaseRegistry: "",
				cluster:            clusterWithPrivateRegistry("cluster-url.com"),
			},
			expected: "cluster-url.com/rancher/imagename",
		},
		{
			name: "Cluster with URL, and default registry, no rancher/ on image",
			input: input{
				image:              "imagename",
				CattleBaseRegistry: "registry-url",
				cluster:            clusterWithPrivateRegistry("cluster-url.com"),
			},
			expected: "cluster-url.com/rancher/imagename",
		},
		{
			name: "Cluster with URL, and default registry, with rancher/ on image",
			input: input{
				image:              "rancher/imagename",
				CattleBaseRegistry: "registry-url",
				cluster:            clusterWithPrivateRegistry("cluster-url.com"),
			},
			expected: "cluster-url.com/rancher/imagename",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := settings.SystemDefaultRegistry.Set(tt.input.CattleBaseRegistry); err != nil {
				t.Errorf("Failed to test TestResolveWithCluster(), unable to set SystemDefaultRegistry with the value: %v", err)
			}
			assertlib.Equalf(t, tt.expected, ResolveWithCluster(tt.input.image, tt.input.cluster), "ResolveWithCluster(%v, %v)", tt.input.image, tt.input.cluster)
		})
	}
}

func TestResolve(t *testing.T) {
	if os.Getenv("CATTLE_BASE_REGISTRY") != "" {
		fmt.Println("Skipping TestResolve. Can't run the tests with CATTLE_BASE_REGISTRY set")
		return
	}

	type input struct {
		image              string
		CattleBaseRegistry string
	}
	tests := []struct {
		name     string
		input    input
		expected string
	}{
		{
			name: "No default",
			input: input{
				image:              "imagename",
				CattleBaseRegistry: "",
			},
			expected: "imagename",
		},
		{
			name: "Default without rancher",
			input: input{
				image:              "imagename",
				CattleBaseRegistry: "default-registry.com",
			},
			expected: "default-registry.com/rancher/imagename",
		},
		{
			name: "Default with rancher",
			input: input{
				image:              "rancher/imagename",
				CattleBaseRegistry: "default-registry.com",
			},
			expected: "default-registry.com/rancher/imagename",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := settings.SystemDefaultRegistry.Set(tt.input.CattleBaseRegistry); err != nil {
				t.Errorf("Failed to test TestResolveWithCluster(), unable to set SystemDefaultRegistry with the value: %v", err)
			}
			assertlib.Equalf(t, tt.expected, Resolve(tt.input.image), "Resolve(%v)", tt.input.image)
		})
	}
}
