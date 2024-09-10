package globaldns

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/rancher/rancher/pkg/controllers/management/rbac"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"

	"github.com/rancher/rancher/pkg/ingresswrapper"
	kextv1beta1 "k8s.io/api/extensions/v1beta1"
	knetworkingv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	GlobaldnsController    = "mgmt-global-dns-controller"
	annotationIngressClass = "kubernetes.io/ingress.class"
	annotationFilter       = "annotationFilter"
	annotationDNSTTL       = "external-dns.alpha.kubernetes.io/ttl"
	defaultIngressClass    = "rancher-external-dns"
)

type GDController struct {
	ingresses               ingresswrapper.CompatClient
	managementContext       *config.ManagementContext
	globalDNSProviderLister v3.GlobalDnsProviderLister
}

func newGlobalDNSController(ctx context.Context, mgmt *config.ManagementContext) *GDController {

	n := &GDController{
		ingresses:               ingresswrapper.NewCompatClient(mgmt.K8sClient, namespace.GlobalNamespace),
		managementContext:       mgmt,
		globalDNSProviderLister: mgmt.Management.GlobalDnsProviders(namespace.GlobalNamespace).Controller().Lister(),
	}
	return n
}

// sync is called periodically and on real updates
func (n *GDController) sync(key string, obj *v3.GlobalDns) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}

	metaAccessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	creatorID, ok := metaAccessor.GetAnnotations()[rbac.CreatorIDAnn]
	if !ok {
		return nil, fmt.Errorf("GlobalDNS %v has no creatorId annotation", metaAccessor.GetName())
	}

	if err := rbac.CreateRoleAndRoleBinding(rbac.GlobalDNSResource, v3.GlobalDnsGroupVersionKind.Kind, obj.Name, namespace.GlobalNamespace,
		rbac.RancherManagementAPIVersion, creatorID, []string{rbac.RancherManagementAPIGroup},
		obj.UID, obj.Spec.Members, n.managementContext); err != nil {
		return nil, err
	}
	//check if status.endpoints is set, if yes create a dummy ingress if not already present
	//if ingress exists, update endpoints if different

	var isUpdate bool

	//check if ingress for this globaldns is already present
	ingress, err := n.getIngressForGlobalDNS(obj)

	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, fmt.Errorf("GlobalDNSController: Error listing ingress for the GlobalDNS %v", err)
	}

	if ingress != nil && err == nil {
		isUpdate = true
	}

	if len(obj.Status.Endpoints) == 0 && !isUpdate {
		return nil, nil
	}

	if !isUpdate {
		ingress, err = n.createIngressForGlobalDNS(obj)
		if err != nil {
			return nil, fmt.Errorf("GlobalDNSController: Error creating an ingress for the GlobalDNS %v", err)
		}
	}

	err = n.updateIngressForDNS(ingress, obj)
	if err != nil {
		return nil, fmt.Errorf("GlobalDNSController: Error updating ingress for the GlobalDNS %v", err)
	}

	return nil, nil
}

func (n *GDController) getIngressForGlobalDNS(globaldns *v3.GlobalDns) (ingresswrapper.Ingress, error) {
	ingress, err := n.ingresses.Get(context.TODO(), strings.Join([]string{"globaldns-ingress", globaldns.Name}, "-"), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	//make sure the ingress is owned by this globalDNS
	if n.isIngressOwnedByGlobalDNS(ingress, globaldns) {
		return ingress, nil
	}
	return nil, nil
}

func (n *GDController) isIngressOwnedByGlobalDNS(obj ingresswrapper.Ingress, globaldns *v3.GlobalDns) bool {
	for i, owners := 0, obj.GetOwnerReferences(); owners != nil && i < len(owners); i++ {
		if owners[i].UID == globaldns.UID && owners[i].Kind == globaldns.Kind {
			return true
		}
	}
	return false
}

func (n *GDController) createIngressForGlobalDNS(globaldns *v3.GlobalDns) (ingresswrapper.Ingress, error) {
	var ingressSpec ingresswrapper.Ingress
	if n.ingresses.ServerSupportsIngressV1 {
		ingressSpec = n.generateNewIngressV1Spec(globaldns)
	} else {
		ingressSpec = n.generateNewIngressV1Beta1Spec(globaldns)
	}
	annotations := ingressSpec.GetAnnotations()
	if globaldns.Spec.TTL != 0 {
		annotations[annotationDNSTTL] = strconv.FormatInt(globaldns.Spec.TTL, 10)
	}
	if globaldns.Spec.ProviderName != "" {
		ingressClass, err := n.getIngressClass(globaldns.Spec.ProviderName)
		if err != nil {
			return nil, err
		}
		if ingressClass != "" {
			annotations[annotationIngressClass] = ingressClass
		}
	}
	ingressSpec.SetAnnotations(annotations)
	ingressObj, err := n.ingresses.Create(context.TODO(), ingressSpec, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	logrus.Infof("Created ingress %v for globalDNS %s", ingressObj.GetName(), globaldns.Name)
	return ingressObj, nil
}

func (n *GDController) generateNewIngressV1Spec(globaldns *v3.GlobalDns) *knetworkingv1.Ingress {
	controller := true
	pathType := knetworkingv1.PathTypeImplementationSpecific
	return &knetworkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: strings.Join([]string{"globaldns-ingress", globaldns.Name}, "-"),
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       globaldns.Name,
					APIVersion: globaldns.APIVersion,
					UID:        globaldns.UID,
					Kind:       globaldns.Kind,
					Controller: &controller,
				},
			},
			Annotations: map[string]string{
				annotationIngressClass: "rancher-external-dns",
			},
			Namespace: globaldns.Namespace,
		},
		Spec: knetworkingv1.IngressSpec{
			Rules: []knetworkingv1.IngressRule{
				{
					Host: globaldns.Spec.FQDN,
					IngressRuleValue: knetworkingv1.IngressRuleValue{
						HTTP: &knetworkingv1.HTTPIngressRuleValue{
							Paths: []knetworkingv1.HTTPIngressPath{
								{
									Backend: knetworkingv1.IngressBackend{
										Service: &knetworkingv1.IngressServiceBackend{
											Name: "http-svc-dummy",
											Port: knetworkingv1.ServiceBackendPort{
												Number: 42,
											},
										},
									},
									PathType: &pathType,
								},
							},
						},
					},
				},
			},
		},
	}
}

func (n *GDController) generateNewIngressV1Beta1Spec(globaldns *v3.GlobalDns) *kextv1beta1.Ingress {
	controller := true
	return &kextv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: strings.Join([]string{"globaldns-ingress", globaldns.Name}, "-"),
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       globaldns.Name,
					APIVersion: globaldns.APIVersion,
					UID:        globaldns.UID,
					Kind:       globaldns.Kind,
					Controller: &controller,
				},
			},
			Annotations: map[string]string{
				annotationIngressClass: "rancher-external-dns",
			},
			Namespace: globaldns.Namespace,
		},
		Spec: kextv1beta1.IngressSpec{
			Rules: []kextv1beta1.IngressRule{
				{
					Host: globaldns.Spec.FQDN,
					IngressRuleValue: kextv1beta1.IngressRuleValue{
						HTTP: &kextv1beta1.HTTPIngressRuleValue{
							Paths: []kextv1beta1.HTTPIngressPath{
								{
									Backend: kextv1beta1.IngressBackend{
										ServiceName: "http-svc-dummy",
										ServicePort: intstr.IntOrString{
											Type:   intstr.Int,
											IntVal: 42,
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
}

func (n *GDController) getIngressClass(globalDNSProviderName string) (string, error) {
	providerName, err := n.getGlobalDNSProviderName(globalDNSProviderName)
	if err != nil {
		return defaultIngressClass, err
	}

	provider, err := n.globalDNSProviderLister.Get(namespace.GlobalNamespace, providerName)
	if err != nil && k8serrors.IsNotFound(err) {
		logrus.Errorf("GlobalDNSController: Object Not found Error %v, while listing GlobalDNSProvider by name %v", err, providerName)
		return defaultIngressClass, nil
	}
	if err != nil {
		return defaultIngressClass, fmt.Errorf("GlobalDNSController: Error %v Listing GlobalDNSProvider by name %v", err, providerName)
	}

	var options map[string]string
	if provider.Spec.Route53ProviderConfig != nil {
		options = provider.Spec.Route53ProviderConfig.AdditionalOptions
	} else if provider.Spec.CloudflareProviderConfig != nil {
		options = provider.Spec.CloudflareProviderConfig.AdditionalOptions
	} else if provider.Spec.AlidnsProviderConfig != nil {
		options = provider.Spec.AlidnsProviderConfig.AdditionalOptions
	}

	if options != nil {
		ingressClassAnnotation, ok := options[annotationFilter]
		if ok {
			prefix := annotationIngressClass + "="
			if strings.HasPrefix(ingressClassAnnotation, prefix) {
				return strings.TrimPrefix(ingressClassAnnotation, prefix), nil
			}
		}
	}
	return defaultIngressClass, nil
}

func (n *GDController) getGlobalDNSProviderName(globalDNSProviderName string) (string, error) {
	split := strings.SplitN(globalDNSProviderName, ":", 2)
	if len(split) != 2 {
		return "", fmt.Errorf("error in splitting globalDNSProviderName %v", globalDNSProviderName)
	}
	provider := split[1]
	return provider, nil
}

func (n *GDController) updateIngressForDNS(ingress ingresswrapper.Ingress, obj *v3.GlobalDns) error {
	var err error

	ingressObj, err := ingresswrapper.ToCompatIngress(ingress)
	if err != nil {
		return err
	}
	if n.ifEndpointsDiffer(ingressObj.Status.LoadBalancer.Ingress, obj.Status.Endpoints) {
		ingressObj.Status.LoadBalancer.Ingress = n.sliceToStatus(obj.Status.Endpoints)
		ingressObj, err = n.ingresses.UpdateStatus(context.TODO(), ingress, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("GlobalDNSController: Error updating Ingress %v", err)
		}
	}

	var updateIngress bool
	if len(ingressObj.Spec.Rules) > 0 {
		if !strings.EqualFold(ingressObj.Spec.Rules[0].Host, obj.Spec.FQDN) {
			ingressObj.Spec.Rules[0].Host = obj.Spec.FQDN
			updateIngress = true
		}
	}

	ttlvalue := strconv.FormatInt(obj.Spec.TTL, 10)
	if !strings.EqualFold(ingressObj.Annotations[annotationDNSTTL], ttlvalue) {
		ingressObj.Annotations[annotationDNSTTL] = ttlvalue
		updateIngress = true
	}

	if obj.Spec.ProviderName != "" {
		ingressClass, err := n.getIngressClass(obj.Spec.ProviderName)
		if err != nil {
			return err
		}
		if !strings.EqualFold(ingressObj.Annotations[annotationIngressClass], ingressClass) {
			ingressObj.Annotations[annotationIngressClass] = ingressClass
			updateIngress = true
		}
	}

	if updateIngress {
		_, err := n.ingresses.Update(context.TODO(), ingressObj, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("GlobalDNSController: Error updating Ingress %v", err)
		}
	}

	return nil
}
func (n *GDController) ifEndpointsDiffer(ingressEps []knetworkingv1.IngressLoadBalancerIngress, endpoints []string) bool {
	if len(ingressEps) != len(endpoints) {
		return true
	}

	mapIngEndpoints := n.gatherIngressEndpoints(ingressEps)
	for _, ep := range endpoints {
		if !mapIngEndpoints[ep] {
			return true
		}
	}
	return false
}

func (n *GDController) gatherIngressEndpoints(ingressEps []knetworkingv1.IngressLoadBalancerIngress) map[string]bool {
	mapIngEndpoints := make(map[string]bool)
	for _, ep := range ingressEps {
		if ep.IP != "" {
			mapIngEndpoints[ep.IP] = true
		} else if ep.Hostname != "" {
			mapIngEndpoints[ep.Hostname] = true
		}
	}
	return mapIngEndpoints
}

// sliceToStatus converts a slice of IP and/or hostnames to LoadBalancerIngress
func (n *GDController) sliceToStatus(endpoints []string) []knetworkingv1.IngressLoadBalancerIngress {
	lbi := []knetworkingv1.IngressLoadBalancerIngress{}
	for _, ep := range endpoints {
		if net.ParseIP(ep) == nil {
			lbi = append(lbi, knetworkingv1.IngressLoadBalancerIngress{Hostname: ep})
		} else {
			lbi = append(lbi, knetworkingv1.IngressLoadBalancerIngress{IP: ep})
		}
	}
	return lbi
}
