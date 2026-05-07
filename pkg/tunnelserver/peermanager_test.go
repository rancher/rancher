package tunnelserver

import (
	"fmt"
	"testing"

	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// fakeServiceController is a minimal mock that implements corecontrollers.ServiceController
// with only the Get method returning useful results. The embedded interface satisfies
// all other methods — they will panic if called, which is acceptable for focused unit tests.
type fakeServiceController struct {
	corecontrollers.ServiceController
	services map[string]*v1.Service // key: "namespace/name"
}

func (f *fakeServiceController) Get(namespace, name string, options metav1.GetOptions) (*v1.Service, error) {
	key := namespace + "/" + name
	svc, ok := f.services[key]
	if !ok {
		return nil, fmt.Errorf("service %s not found", key)
	}
	return svc, nil
}

func TestDetectPrimaryIPFamily(t *testing.T) {
	tests := []struct {
		name         string
		services     map[string]*v1.Service
		namespace    string
		peerServices string
		expected     v1.IPFamily
	}{
		{
			name: "IPv4 primary family in dual-stack",
			services: map[string]*v1.Service{
				"cattle-system/rancher": {
					Spec: v1.ServiceSpec{
						IPFamilies: []v1.IPFamily{v1.IPv4Protocol, v1.IPv6Protocol},
					},
				},
			},
			namespace:    "cattle-system",
			peerServices: "rancher",
			expected:     v1.IPv4Protocol,
		},
		{
			name: "IPv6 primary family in dual-stack",
			services: map[string]*v1.Service{
				"cattle-system/rancher": {
					Spec: v1.ServiceSpec{
						IPFamilies: []v1.IPFamily{v1.IPv6Protocol, v1.IPv4Protocol},
					},
				},
			},
			namespace:    "cattle-system",
			peerServices: "rancher",
			expected:     v1.IPv6Protocol,
		},
		{
			name: "IPv6 single stack",
			services: map[string]*v1.Service{
				"cattle-system/rancher": {
					Spec: v1.ServiceSpec{
						IPFamilies: []v1.IPFamily{v1.IPv6Protocol},
					},
				},
			},
			namespace:    "cattle-system",
			peerServices: "rancher",
			expected:     v1.IPv6Protocol,
		},
		{
			name: "IPv4 single stack",
			services: map[string]*v1.Service{
				"cattle-system/rancher": {
					Spec: v1.ServiceSpec{
						IPFamilies: []v1.IPFamily{v1.IPv4Protocol},
					},
				},
			},
			namespace:    "cattle-system",
			peerServices: "rancher",
			expected:     v1.IPv4Protocol,
		},
		{
			name:         "service not found returns empty",
			services:     map[string]*v1.Service{},
			namespace:    "cattle-system",
			peerServices: "rancher",
			expected:     "",
		},
		{
			name: "empty IPFamilies returns empty",
			services: map[string]*v1.Service{
				"cattle-system/rancher": {
					Spec: v1.ServiceSpec{},
				},
			},
			namespace:    "cattle-system",
			peerServices: "rancher",
			expected:     "",
		},
		{
			name: "multiple peer services uses first available",
			services: map[string]*v1.Service{
				"cattle-system/rancher": {
					Spec: v1.ServiceSpec{
						IPFamilies: []v1.IPFamily{v1.IPv6Protocol},
					},
				},
			},
			namespace:    "cattle-system",
			peerServices: "rancher,rancher-other",
			expected:     v1.IPv6Protocol,
		},
		{
			name: "first peer service missing falls through to second",
			services: map[string]*v1.Service{
				"cattle-system/rancher-other": {
					Spec: v1.ServiceSpec{
						IPFamilies: []v1.IPFamily{v1.IPv6Protocol},
					},
				},
			},
			namespace:    "cattle-system",
			peerServices: "rancher,rancher-other",
			expected:     v1.IPv6Protocol,
		},
		{
			name:         "empty peer services string returns empty",
			services:     map[string]*v1.Service{},
			namespace:    "cattle-system",
			peerServices: "",
			expected:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeServiceController{services: tt.services}
			result := detectPrimaryIPFamily(fake, tt.namespace, tt.peerServices)
			assert.Equal(t, tt.expected, result)
		})
	}
}
