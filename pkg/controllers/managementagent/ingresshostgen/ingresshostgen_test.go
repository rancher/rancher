package ingresshostgen

import (
	"testing"

	rnetworkingv1 "github.com/rancher/rancher/pkg/generated/norman/networking.k8s.io/v1"
	rnetworkingv1fakes "github.com/rancher/rancher/pkg/generated/norman/networking.k8s.io/v1/fakes"
	"github.com/rancher/rancher/pkg/ingresswrapper"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/require"
	knetworkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
)

type NetworkingInterface struct {
	*rnetworkingv1fakes.IngressesGetterMock
	*rnetworkingv1fakes.NetworkPoliciesGetterMock
}

var (
	originalIngressIPDomain = settings.IngressIPDomain.Get()
)

type cleanupFunc func()

func Test_Sync(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	clientset.Discovery().(*fakediscovery.FakeDiscovery).Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "networking.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{
					Kind: "Ingress",
				},
			},
		},
	}

	mockedIngressesGetter := &rnetworkingv1fakes.IngressesGetterMock{
		IngressesFunc: func(namespace string) rnetworkingv1.IngressInterface {
			return &rnetworkingv1fakes.IngressInterfaceMock{
				UpdateFunc: func(obj *rnetworkingv1.Ingress) (*rnetworkingv1.Ingress, error) {
					return obj, nil
				},
			}
		},
	}
	ingressCompat := ingresswrapper.NewCompatInterface(&NetworkingInterface{
		IngressesGetterMock:       mockedIngressesGetter,
		NetworkPoliciesGetterMock: &rnetworkingv1fakes.NetworkPoliciesGetterMock{},
	}, nil, clientset)
	gen := &IngressHostGen{
		ingress: ingressCompat,
	}
	tests := []struct {
		desc          string
		name          string
		namespace     string
		setup         func() cleanupFunc
		rules         []knetworkingv1.IngressRule
		statusIngress []knetworkingv1.IngressLoadBalancerIngress
		// If nil, then we expect sync to return nil, nil
		expectedRules []knetworkingv1.IngressRule
	}{
		{
			desc:      "single ip and host matching",
			name:      "foo",
			namespace: "bar",
			rules: []knetworkingv1.IngressRule{
				{
					Host: "sslip.io",
				},
			},
			statusIngress: []knetworkingv1.IngressLoadBalancerIngress{
				{
					IP: "192.0.2.1",
				},
			},
			expectedRules: []knetworkingv1.IngressRule{
				{
					Host: "foo.bar.192.0.2.1.sslip.io",
				},
			},
		},
		{
			desc:      "not matching ingress IP domain",
			name:      "foo",
			namespace: "bar",
			rules: []knetworkingv1.IngressRule{
				{
					Host: "example.com",
				},
			},
			statusIngress: []knetworkingv1.IngressLoadBalancerIngress{
				{
					IP: "192.0.2.1",
				},
			},
		},
		{
			desc:      "already generated",
			name:      "foo",
			namespace: "bar",
			rules: []knetworkingv1.IngressRule{
				{
					Host: "foo.bar.192.0.2.1.sslip.io",
				},
			},
			statusIngress: []knetworkingv1.IngressLoadBalancerIngress{
				{
					IP: "192.0.2.1",
				},
			},
		},
		{
			desc:      "different ingress IP domain matching host",
			name:      "foo",
			namespace: "bar",
			setup: func() cleanupFunc {
				settings.IngressIPDomain.Set("example.com")
				return func() {
					settings.IngressIPDomain.Set(originalIngressIPDomain)
				}
			},
			rules: []knetworkingv1.IngressRule{
				{
					Host: "example.com",
				},
			},
			statusIngress: []knetworkingv1.IngressLoadBalancerIngress{
				{
					IP: "192.0.2.1",
				},
			},
			expectedRules: []knetworkingv1.IngressRule{
				{
					Host: "foo.bar.192.0.2.1.example.com",
				},
			},
		},
		{
			desc:      "different ingress IP domain not matching host",
			name:      "foo",
			namespace: "bar",
			setup: func() cleanupFunc {
				settings.IngressIPDomain.Set("example.com")
				return func() {
					settings.IngressIPDomain.Set(originalIngressIPDomain)
				}
			},
			rules: []knetworkingv1.IngressRule{
				{
					Host: "sslip.io",
				},
			},
			statusIngress: []knetworkingv1.IngressLoadBalancerIngress{
				{
					IP: "192.0.2.1",
				},
			},
		},
		{
			desc:      "many hosts and IPs matching IP domain",
			name:      "foo",
			namespace: "bar",
			rules: []knetworkingv1.IngressRule{
				{
					Host: "example.com",
				},
				{
					Host: "sslip.io",
				},
			},
			statusIngress: []knetworkingv1.IngressLoadBalancerIngress{
				{
					IP: "192.0.2.1",
				},
				{
					IP: "192.0.2.2",
				},
			},
			expectedRules: []knetworkingv1.IngressRule{
				{
					Host: "example.com",
				},
				{
					Host: "foo.bar.192.0.2.1.sslip.io",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			if tt.setup != nil {
				cleanup := tt.setup()
				defer cleanup()
			}

			obj := &knetworkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tt.name,
					Namespace: tt.namespace,
				},
				Spec: knetworkingv1.IngressSpec{
					Rules: tt.rules,
				},
				Status: knetworkingv1.IngressStatus{
					LoadBalancer: knetworkingv1.IngressLoadBalancerStatus{
						Ingress: tt.statusIngress,
					},
				},
			}
			updatedObj, err := gen.sync("", obj)
			require.NoError(t, err, "unexpected error")

			if updatedObj == nil {
				require.Nil(t, tt.expectedRules)
				return
			}

			updatedIngress := updatedObj.(*knetworkingv1.Ingress)
			require.Equal(t, tt.expectedRules, updatedIngress.Spec.Rules)
		})
	}
}
