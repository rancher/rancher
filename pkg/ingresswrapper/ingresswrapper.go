package ingresswrapper

import (
	"context"
	"fmt"
	"reflect"

	rextv1beta1 "github.com/rancher/rancher/pkg/generated/norman/extensions/v1beta1"
	rnetworkingv1 "github.com/rancher/rancher/pkg/generated/norman/networking.k8s.io/v1"
	kextv1beta1 "k8s.io/api/extensions/v1beta1"
	knetworkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	clientv1beta1 "k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
	clientv1 "k8s.io/client-go/kubernetes/typed/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Ingress is an interface with features of both metav1.Object and runtime.Object,
// and can be used whenever the metadata of a resource is needed but the actual
// spec data is not.
type Ingress interface {
	client.Object
}

// ServerSupportsIngressV1 runs a discovery query to check whether the
// networking.k8s.io/v1/Ingress resource is supported in the given cluster.
func ServerSupportsIngressV1(k8sClient kubernetes.Interface) bool {
	resources, err := k8sClient.Discovery().ServerResourcesForGroupVersion("networking.k8s.io/v1")
	if err != nil || resources == nil {
		return false
	}
	for _, r := range resources.APIResources {
		if r.Kind == "Ingress" {
			return true
		}
	}
	return false
}

// CompatSyncV1 returns a controller handler for networking.k8s.io/v1/Ingress resources.
func CompatSyncV1(fn func(string, Ingress) (runtime.Object, error)) func(string, *knetworkingv1.Ingress) (runtime.Object, error) {
	return func(key string, obj *knetworkingv1.Ingress) (runtime.Object, error) { return fn(key, obj) }
}

// CompatSyncV1Beta1 returns a controller handler for extensions/v1beta1/Ingress resources.
func CompatSyncV1Beta1(fn func(string, Ingress) (runtime.Object, error)) func(string, *kextv1beta1.Ingress) (runtime.Object, error) {
	return func(key string, obj *kextv1beta1.Ingress) (runtime.Object, error) { return fn(key, obj) }
}

// ToUnstructured converts a resource Object into a unstructured map.
func ToUnstructured(o interface{}) (map[string]interface{}, error) {
	obj := o.(client.Object)
	// Populate GVK explicitly. See https://github.com/kubernetes/kubernetes/issues/3030
	gvks, _, err := scheme.Scheme.ObjectKinds(obj)
	if err != nil {
		return nil, fmt.Errorf("error finding APIVersion or Kind for ingress; %w", err)
	}
	for _, gvk := range gvks {
		if len(gvk.Kind) == 0 {
			continue
		}
		if len(gvk.Version) == 0 || gvk.Version == runtime.APIVersionInternal {
			continue
		}
		obj.GetObjectKind().SetGroupVersionKind(gvk)
		break
	}
	return runtime.DefaultUnstructuredConverter.ToUnstructured(o)
}

// CompatIngress is an intermediary between networking.k8s.io/v1/Ingress and
// extensions/v1beta1/Ingress. It inlines networking.k8s.io/v1/Ingress and so
// it can be handled, and its attributes accessed, in the same way a v1 Ingress
// resource would be handled.
type CompatIngress struct {
	knetworkingv1.Ingress
}

// ToCompatIngress converts a networking.k8s.io/v1/Ingress or
// extensions/v1beta1/Ingress object to a CompatIngress object. If the object
// is already a CompatIngress object, it is returned unchanged.
func ToCompatIngress(obj interface{}) (*CompatIngress, error) {
	switch o := obj.(type) {
	case *CompatIngress:
		return o, nil
	case *knetworkingv1.Ingress:
		if reflect.ValueOf(o).IsNil() {
			return &CompatIngress{}, nil
		}
		return &CompatIngress{*o}, nil
	case *kextv1beta1.Ingress:
		unst, err := ToUnstructured(o)
		if err != nil {
			return nil, err
		}
		spec := unst["spec"].(map[string]interface{})
		if o.Spec.Backend != nil {
			spec["defaultBackend"] = map[string]interface{}{
				"service": map[string]interface{}{
					"name": o.Spec.Backend.ServiceName,
					"port": map[string]interface{}{
						"number": o.Spec.Backend.ServicePort.IntVal,
					},
				},
			}
			delete(spec, "backend")
		}
		if spec["rules"] != nil {
			rules := spec["rules"].([]interface{})
			for i, r := range o.Spec.Rules {
				rule := rules[i].(map[string]interface{})
				if r.HTTP != nil {
					for j, p := range r.HTTP.Paths {
						path := map[string]interface{}{
							"path": p.Path,
							"backend": map[string]interface{}{
								"service": map[string]interface{}{
									"name": p.Backend.ServiceName,
									"port": map[string]interface{}{
										"number": p.Backend.ServicePort.IntVal,
									},
								},
							},
						}
						rule["http"].(map[string]interface{})["paths"].([]interface{})[j] = path
					}
				}
				if rule["http"] != nil && rule["http"].(map[string]interface{})["backend"] != nil {
					delete(rule["http"].(map[string]interface{})["backend"].(map[string]interface{}), "serviceName")
					delete(rule["http"].(map[string]interface{})["backend"].(map[string]interface{}), "servicePort")
				}
			}
		}
		compat := CompatIngress{}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(unst, &compat.Ingress)
		if err != nil {
			return nil, err
		}
		return &compat, nil
	default:
		return nil, fmt.Errorf("unexpected ingress type: %T", o)
	}
}

// ToIngressV1FromCompat converts a CompatIngress object to a networking.k8s.io/v1/Ingress object.
func ToIngressV1FromCompat(obj *CompatIngress) (*knetworkingv1.Ingress, error) {
	unst, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	ingress := knetworkingv1.Ingress{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unst["ingress"].(map[string]interface{}), &ingress)
	if err != nil {
		return nil, fmt.Errorf("error converting CompatIngress to networking.k8s.io/v1/Ingress: %w", err)
	}
	return &ingress, nil
}

// ToIngressV1Beta1FromCompat converts a CompatIngress object to an extensions/v1beta1/Ingress object.
func ToIngressV1Beta1FromCompat(obj *CompatIngress) (*kextv1beta1.Ingress, error) {
	unst, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	spec := unst["ingress"].(map[string]interface{})["spec"].(map[string]interface{})
	if spec["defaultBackend"] != nil {
		spec["backend"] = map[string]interface{}{
			"serviceName": obj.Spec.DefaultBackend.Service.Name,
			"servicePort": intstr.IntOrString{
				IntVal: obj.Spec.DefaultBackend.Service.Port.Number,
			},
		}
	}
	delete(spec, "defaultBackend")
	if spec["rules"] != nil {
		rules := spec["rules"].([]interface{})
		for i, r := range obj.Spec.Rules {
			rule := rules[i].(map[string]interface{})
			if r.HTTP != nil {
				for j, p := range r.HTTP.Paths {
					backend := map[string]interface{}{
						"serviceName": p.Backend.Service.Name,
						"servicePort": intstr.IntOrString{
							IntVal: p.Backend.Service.Port.Number,
						},
					}
					rule["http"].(map[string]interface{})["paths"].([]interface{})[j].(map[string]interface{})["backend"] = backend
				}
			}
		}
	}
	ingress := kextv1beta1.Ingress{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unst["ingress"].(map[string]interface{}), &ingress)
	if err != nil {
		return nil, fmt.Errorf("error converting CompatIngress to extensions/v1beta1/Ingress: %w", err)
	}
	return &ingress, nil
}

// CompatInterface is a wrapper for the norman interfaces for
// networking.k8s.io/v1/Ingress and extensions/v1beta1/Ingress.
type CompatInterface struct {
	ingressInterface        rnetworkingv1.IngressInterface
	ingressLegacyInterface  rextv1beta1.IngressInterface
	ServerSupportsIngressV1 bool
}

// NewCompatInterface initializes either a networking.k8s.io/v1/Ingress client
// or an extensions/v1beta1 client and returns a CompatInterface.
func NewCompatInterface(networkingAPI rnetworkingv1.Interface, extensionsAPI rextv1beta1.Interface, k8sClient kubernetes.Interface) CompatInterface {
	c := CompatInterface{
		ServerSupportsIngressV1: ServerSupportsIngressV1(k8sClient),
	}
	if c.ServerSupportsIngressV1 {
		c.ingressInterface = networkingAPI.Ingresses("")
		return c
	}
	c.ingressLegacyInterface = extensionsAPI.Ingresses("")
	return c
}

// Update calls the Update method for the active ingress interface.
func (i *CompatInterface) Update(ingress Ingress) (runtime.Object, error) {
	obj := ingress.(*CompatIngress)
	if i.ServerSupportsIngressV1 {
		toUpdate, err := ToIngressV1FromCompat(obj)
		if err != nil {
			return toUpdate, err
		}
		return i.ingressInterface.Update(toUpdate)
	}
	toUpdate, err := ToIngressV1Beta1FromCompat(obj)
	if err != nil {
		return toUpdate, err
	}
	return i.ingressLegacyInterface.Update(toUpdate)
}

// CompatLister is a wrapper for the norman listers for
// networking.k8s.io/v1/Ingress and extensions/v1beta1/Ingress.
type CompatLister struct {
	ingressLister           rnetworkingv1.IngressLister
	ingressLegacyLister     rextv1beta1.IngressLister
	ServerSupportsIngressV1 bool
}

// NewCompatLister initializes either a networking.k8s.io/v1/Ingress watch or
// an extensions/v1beta1/Ingress watch and returns a CompatLister.
func NewCompatLister(networkingAPI rnetworkingv1.Interface, extensionsAPI rextv1beta1.Interface, k8sClient kubernetes.Interface) CompatLister {
	c := CompatLister{
		ServerSupportsIngressV1: ServerSupportsIngressV1(k8sClient),
	}
	if c.ServerSupportsIngressV1 {
		c.ingressLister = networkingAPI.Ingresses("").Controller().Lister()
		return c
	}
	c.ingressLegacyLister = extensionsAPI.Ingresses("").Controller().Lister()
	return c
}

// List calls the List method for the active ingress lister.
func (l *CompatLister) List(namespace string, selector labels.Selector) ([]*CompatIngress, error) {
	var list []*CompatIngress
	if l.ServerSupportsIngressV1 {
		ingresses, err := l.ingressLister.List(namespace, selector)
		if err != nil {
			return list, err
		}
		for _, i := range ingresses {
			ingressCompat, err := ToCompatIngress(i)
			if err != nil {
				return list, err
			}
			list = append(list, ingressCompat)
		}
		return list, nil
	}
	ingresses, err := l.ingressLegacyLister.List(namespace, selector)
	if err != nil {
		return list, err
	}
	for _, i := range ingresses {
		ingressCompat, err := ToCompatIngress(i)
		if err != nil {
			return list, err
		}
		list = append(list, ingressCompat)
	}
	return list, nil
}

// CompatClient is a wrapper for the client-go networking.k8s.io/v1/Ingress and
// extensions/v1beta1/Ingress clients.
type CompatClient struct {
	ingressClient           clientv1.IngressInterface
	ingressLegacyClient     clientv1beta1.IngressInterface
	ServerSupportsIngressV1 bool
}

// NewCompatClient initializes either a networking.k8s.io/v1/Ingress client-go client or
// an extensions/v1beta1/Ingress client-go client and returns a CompatClient.
func NewCompatClient(k8sClient kubernetes.Interface, namespace string) CompatClient {
	c := CompatClient{
		ServerSupportsIngressV1: ServerSupportsIngressV1(k8sClient),
	}
	if c.ServerSupportsIngressV1 {
		c.ingressClient = k8sClient.NetworkingV1().Ingresses(namespace)
		return c
	}
	c.ingressLegacyClient = k8sClient.ExtensionsV1beta1().Ingresses(namespace)
	return c
}

// Get calls the Get method for the active ingress client.
func (c *CompatClient) Get(ctx context.Context, name string, opts metav1.GetOptions) (*CompatIngress, error) {
	if c.ServerSupportsIngressV1 {
		ret, err := c.ingressClient.Get(ctx, name, opts)
		if err != nil {
			return nil, err
		}
		return ToCompatIngress(ret)
	}
	ret, err := c.ingressLegacyClient.Get(ctx, name, opts)
	if err != nil {
		return nil, err
	}
	return ToCompatIngress(ret)
}

// Create calls the Create method for the active ingress client.
func (c *CompatClient) Create(ctx context.Context, ingress Ingress, opts metav1.CreateOptions) (*CompatIngress, error) {
	if c.ServerSupportsIngressV1 {
		ret, err := c.ingressClient.Create(ctx, ingress.(*knetworkingv1.Ingress), opts)
		if err != nil {
			return nil, err
		}
		return ToCompatIngress(ret)
	}
	ret, err := c.ingressLegacyClient.Create(ctx, ingress.(*kextv1beta1.Ingress), opts)
	if err != nil {
		return nil, err
	}
	return ToCompatIngress(ret)
}

// UpdateStatus calls the UpdateStatus method for the active ingress client.
func (c *CompatClient) UpdateStatus(ctx context.Context, ingress Ingress, opts metav1.UpdateOptions) (*CompatIngress, error) {
	if c.ServerSupportsIngressV1 {
		var toUpdate *knetworkingv1.Ingress
		if o, ok := ingress.(*CompatIngress); ok {
			var err error
			toUpdate, err = ToIngressV1FromCompat(o)
			if err != nil {
				return nil, err
			}
		} else {
			toUpdate = ingress.(*knetworkingv1.Ingress)
		}
		ret, err := c.ingressClient.UpdateStatus(ctx, toUpdate, opts)
		if err != nil {
			return nil, err
		}
		return ToCompatIngress(ret)
	}
	var toUpdate *kextv1beta1.Ingress
	if o, ok := ingress.(*CompatIngress); ok {
		var err error
		toUpdate, err = ToIngressV1Beta1FromCompat(o)
		if err != nil {
			return nil, err
		}
	} else {
		toUpdate = ingress.(*kextv1beta1.Ingress)
	}
	ret, err := c.ingressLegacyClient.UpdateStatus(ctx, toUpdate, opts)
	if err != nil {
		return nil, err
	}
	return ToCompatIngress(ret)
}

// Update calls the Update method for the active ingress client.
func (c *CompatClient) Update(ctx context.Context, ingress Ingress, opts metav1.UpdateOptions) (*CompatIngress, error) {
	if c.ServerSupportsIngressV1 {
		var toUpdate *knetworkingv1.Ingress
		if o, ok := ingress.(*CompatIngress); ok {
			var err error
			toUpdate, err = ToIngressV1FromCompat(o)
			if err != nil {
				return nil, err
			}
		} else {
			toUpdate = ingress.(*knetworkingv1.Ingress)
		}
		ret, err := c.ingressClient.Update(ctx, toUpdate, opts)
		if err != nil {
			return nil, err
		}
		return ToCompatIngress(ret)
	}
	var toUpdate *kextv1beta1.Ingress
	if o, ok := ingress.(*CompatIngress); ok {
		var err error
		toUpdate, err = ToIngressV1Beta1FromCompat(o)
		if err != nil {
			return nil, err
		}
	} else {
		toUpdate = ingress.(*kextv1beta1.Ingress)
	}
	ret, err := c.ingressLegacyClient.Update(ctx, toUpdate, opts)
	if err != nil {
		return nil, err
	}
	return ToCompatIngress(ret)
}
