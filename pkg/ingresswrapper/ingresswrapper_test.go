package ingresswrapper

import (
	"context"
	"testing"

	rextv1beta1 "github.com/rancher/rancher/pkg/generated/norman/extensions/v1beta1"
	rextv1beta1fakes "github.com/rancher/rancher/pkg/generated/norman/extensions/v1beta1/fakes"
	rnetworkingv1 "github.com/rancher/rancher/pkg/generated/norman/networking.k8s.io/v1"
	rnetworkingv1fakes "github.com/rancher/rancher/pkg/generated/norman/networking.k8s.io/v1/fakes"
	"github.com/stretchr/testify/assert"
	kextv1beta1 "k8s.io/api/extensions/v1beta1"
	knetworkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
)

func TestServerSupportsIngressV1(t *testing.T) {
	tests := []struct {
		name               string
		discoveryResources []*metav1.APIResourceList
		want               bool
	}{
		{
			name: "supports v1 only",
			discoveryResources: []*metav1.APIResourceList{
				{
					GroupVersion: "networking.k8s.io/v1",
					APIResources: []metav1.APIResource{
						{
							Kind: "Ingress",
						},
					},
				},
			},
			want: true,
		},
		{
			name: "supports v1beta1 only",
			discoveryResources: []*metav1.APIResourceList{
				{
					GroupVersion: "extensions/v1beta1",
					APIResources: []metav1.APIResource{
						{
							Kind: "Ingress",
						},
					},
				},
			},
			want: false,
		},
		{
			name: "supports v1 and v1beta1",
			discoveryResources: []*metav1.APIResourceList{
				{
					GroupVersion: "extensions/v1beta1",
					APIResources: []metav1.APIResource{
						{
							Kind: "Ingress",
						},
					},
				},
				{
					GroupVersion: "networking.k8s.io/v1",
					APIResources: []metav1.APIResource{
						{
							Kind: "Ingress",
						},
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset()
			clientset.Discovery().(*fakediscovery.FakeDiscovery).Resources = tt.discoveryResources
			got := ServerSupportsIngressV1(clientset)
			assert.Equal(t, tt.want, got)
		})
	}
}

var pathTypeExact = knetworkingv1.PathTypeExact
var ingressV1Spec = knetworkingv1.IngressSpec{
	DefaultBackend: &knetworkingv1.IngressBackend{
		Service: &knetworkingv1.IngressServiceBackend{
			Name: "defaultService",
			Port: knetworkingv1.ServiceBackendPort{
				Name:   "defaultPort",
				Number: 333,
			},
		},
	},
	TLS: []knetworkingv1.IngressTLS{
		{
			Hosts: []string{
				"foo.com",
				"www.foo.com",
			},
		},
	},
	Rules: []knetworkingv1.IngressRule{
		{
			Host: "bar.com",
			IngressRuleValue: knetworkingv1.IngressRuleValue{
				HTTP: &knetworkingv1.HTTPIngressRuleValue{
					Paths: []knetworkingv1.HTTPIngressPath{
						{
							Path:     "/test",
							PathType: &pathTypeExact,
							Backend: knetworkingv1.IngressBackend{
								Service: &knetworkingv1.IngressServiceBackend{
									Name: "barService",
									Port: knetworkingv1.ServiceBackendPort{
										Name:   "barPort",
										Number: 444,
									},
								},
							},
						},
					},
				},
			},
		},
	},
}

var ingressBetaSpec = kextv1beta1.IngressSpec{
	Backend: &kextv1beta1.IngressBackend{
		ServiceName: "defaultBetaService",
		ServicePort: intstr.IntOrString{
			IntVal: 555,
		},
	},
	TLS: []kextv1beta1.IngressTLS{
		{
			Hosts: []string{
				"ftp.foo.com",
			},
		},
	},
	Rules: []kextv1beta1.IngressRule{
		{
			Host: "baz.com",
			IngressRuleValue: kextv1beta1.IngressRuleValue{
				HTTP: &kextv1beta1.HTTPIngressRuleValue{
					Paths: []kextv1beta1.HTTPIngressPath{
						{
							Path: "/test/v1beta1",
							Backend: kextv1beta1.IngressBackend{
								ServiceName: "specificBetaService",
								ServicePort: intstr.IntOrString{
									IntVal: 666,
								},
							},
						},
					},
				},
			},
		},
	},
}

var convertedSpec = knetworkingv1.IngressSpec{
	DefaultBackend: &knetworkingv1.IngressBackend{
		Service: &knetworkingv1.IngressServiceBackend{
			Name: "defaultBetaService",
			Port: knetworkingv1.ServiceBackendPort{
				Number: 555,
			},
		},
	},
	TLS: []knetworkingv1.IngressTLS{
		{
			Hosts: []string{
				"ftp.foo.com",
			},
		},
	},
	Rules: []knetworkingv1.IngressRule{
		{
			Host: "baz.com",
			IngressRuleValue: knetworkingv1.IngressRuleValue{
				HTTP: &knetworkingv1.HTTPIngressRuleValue{
					Paths: []knetworkingv1.HTTPIngressPath{
						{
							Path: "/test/v1beta1",
							Backend: knetworkingv1.IngressBackend{
								Service: &knetworkingv1.IngressServiceBackend{
									Name: "specificBetaService",
									Port: knetworkingv1.ServiceBackendPort{
										Number: 666,
									},
								},
							},
						},
					},
				},
			},
		},
	},
}

func TestToCompatIngress(t *testing.T) {
	tests := []struct {
		name    string
		obj     interface{}
		want    *CompatIngress
		wantErr bool
	}{
		{
			name:    "CompatIngress",
			obj:     &CompatIngress{},
			want:    &CompatIngress{},
			wantErr: false,
		},
		{
			name: "networking.k8s.io/v1/Ingress",
			obj: &knetworkingv1.Ingress{
				Spec: ingressV1Spec,
			},
			want: &CompatIngress{
				Ingress: knetworkingv1.Ingress{
					Spec: ingressV1Spec,
				},
			},
			wantErr: false,
		},
		{
			name: "extensions/v1beta1/Ingress",
			obj: &kextv1beta1.Ingress{
				Spec: ingressBetaSpec,
			},
			want: &CompatIngress{
				Ingress: knetworkingv1.Ingress{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Ingress",
						APIVersion: "extensions/v1beta1",
					},
					Spec: convertedSpec,
				},
			},
			wantErr: false,
		},
		{
			name:    "invalid object",
			obj:     struct{}{},
			want:    &CompatIngress{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToCompatIngress(tt.obj)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.Equal(t, *tt.want, *got)
				assert.Nil(t, err)
			}
		})
	}
}

func TestToIngressV1FromCompat(t *testing.T) {
	tests := []struct {
		name string
		obj  *CompatIngress
		want *knetworkingv1.Ingress
	}{
		{
			name: "empty CompatIngress",
			obj:  &CompatIngress{},
			want: &knetworkingv1.Ingress{},
		},
		{
			name: "full CompatIngress",
			obj: &CompatIngress{
				Ingress: knetworkingv1.Ingress{
					Spec: ingressV1Spec,
				},
			},
			want: &knetworkingv1.Ingress{
				Spec: ingressV1Spec,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := ToIngressV1FromCompat(tt.obj)
			assert.Equal(t, *tt.want, *got)
		})
	}
}

func TestToIngressV1Beta1FromCompat(t *testing.T) {
	tests := []struct {
		name    string
		obj     *CompatIngress
		want    *kextv1beta1.Ingress
		wantErr bool
	}{
		{
			name: "empty CompatIngress",
			obj:  &CompatIngress{},
			want: &kextv1beta1.Ingress{},
		},
		{
			name: "full CompatIngress",
			obj: &CompatIngress{knetworkingv1.Ingress{
				Spec: convertedSpec,
			}},
			want: &kextv1beta1.Ingress{
				Spec: ingressBetaSpec,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := ToIngressV1Beta1FromCompat(tt.obj)
			assert.Equal(t, *tt.want, *got)
		})
	}
}

func TestCompatReverse(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
	}{
		{
			name: "reverse networking.k8s.io/v1",
			obj: &knetworkingv1.Ingress{
				Spec: ingressV1Spec,
			},
		},
		{
			name: "reverse extensionsv1/v1beta1",
			obj: &kextv1beta1.Ingress{
				Spec: ingressBetaSpec,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compat, _ := ToCompatIngress(tt.obj)
			switch o := tt.obj.(type) {
			case *knetworkingv1.Ingress:
				got, _ := ToIngressV1FromCompat(compat)
				assert.Equal(t, *o, *got)
			case *kextv1beta1.Ingress:
				got, _ := ToIngressV1Beta1FromCompat(compat)
				assert.Equal(t, *o, *got)
			}
		})
	}
}

func TestCompatInterface(t *testing.T) {
	var v1called int
	var v1beta1called int
	tests := []struct {
		name            string
		compatInterface CompatInterface
		v1calls         int
		v1beta1calls    int
	}{
		{
			name: "update networking.k8s.io/v1/Ingress",
			compatInterface: CompatInterface{
				ingressInterface: &rnetworkingv1fakes.IngressInterfaceMock{
					UpdateFunc: func(*rnetworkingv1.Ingress) (*rnetworkingv1.Ingress, error) {
						v1called += 1
						return nil, nil
					},
				},
				ServerSupportsIngressV1: true,
			},
			v1calls: 1,
		},
		{
			name: "update extensions/v1/Ingress",
			compatInterface: CompatInterface{
				ingressLegacyInterface: &rextv1beta1fakes.IngressInterfaceMock{
					UpdateFunc: func(*rextv1beta1.Ingress) (*rextv1beta1.Ingress, error) {
						v1beta1called += 1
						return nil, nil
					},
				},
				ServerSupportsIngressV1: false,
			},
			v1beta1calls: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _ = tt.compatInterface.Update(&CompatIngress{})
			assert.Equal(t, tt.v1calls, v1called)
			assert.Equal(t, tt.v1beta1calls, v1beta1called)
			v1called = 0
			v1beta1called = 0
		})
	}
}

func TestCompatLister(t *testing.T) {
	var v1called int
	var v1beta1called int
	tests := []struct {
		name         string
		compatLister CompatLister
		v1calls      int
		v1beta1calls int
	}{
		{
			name: "list networking.k8s.io/v1/Ingress",
			compatLister: CompatLister{
				ingressLister: &rnetworkingv1fakes.IngressListerMock{
					ListFunc: func(string, labels.Selector) ([]*rnetworkingv1.Ingress, error) {
						v1called += 1
						return nil, nil
					},
				},
				ServerSupportsIngressV1: true,
			},
			v1calls: 1,
		},
		{
			name: "list extensions/v1/Ingress",
			compatLister: CompatLister{
				ingressLegacyLister: &rextv1beta1fakes.IngressListerMock{
					ListFunc: func(string, labels.Selector) ([]*rextv1beta1.Ingress, error) {
						v1beta1called += 1
						return nil, nil
					},
				},
				ServerSupportsIngressV1: false,
			},
			v1beta1calls: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _ = tt.compatLister.List("", labels.NewSelector())
			assert.Equal(t, tt.v1calls, v1called)
			assert.Equal(t, tt.v1beta1calls, v1beta1called)
			v1called = 0
			v1beta1called = 0
		})
	}
}

func TestCompatClient(t *testing.T) {
	tests := []struct {
		name         string
		compatClient CompatClient
		obj          Ingress
	}{
		{
			name: "list networking.k8s.io/v1/Ingress",
			compatClient: CompatClient{
				ingressClient:           fake.NewSimpleClientset(&knetworkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "test"}}).NetworkingV1().Ingresses(""),
				ServerSupportsIngressV1: true,
			},
			obj: &knetworkingv1.Ingress{},
		},
		{
			name: "list extensions/v1/Ingress",
			compatClient: CompatClient{
				ingressLegacyClient:     fake.NewSimpleClientset(&kextv1beta1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "test"}}).ExtensionsV1beta1().Ingresses(""),
				ServerSupportsIngressV1: false,
			},
			obj: &kextv1beta1.Ingress{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.compatClient.Get(context.Background(), "test", metav1.GetOptions{})
			assert.NoError(t, err)
			_, err = tt.compatClient.Create(context.Background(), tt.obj, metav1.CreateOptions{})
			assert.NoError(t, err)
			_, err = tt.compatClient.UpdateStatus(context.Background(), tt.obj, metav1.UpdateOptions{})
			assert.NoError(t, err)
			_, err = tt.compatClient.Update(context.Background(), tt.obj, metav1.UpdateOptions{})
			assert.NoError(t, err)
		})
	}
}
