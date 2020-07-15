package globaldns

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/rancher/rancher/pkg/controllers/management/rbac"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/types/namespace"
	"github.com/sirupsen/logrus"

	"strconv"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientv1beta1 "k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
)

const (
	GlobaldnsController    = "mgmt-global-dns-controller"
	annotationIngressClass = "kubernetes.io/ingress.class"
	annotationDNSTTL       = "external-dns.alpha.kubernetes.io/ttl"
)

type GDController struct {
	ingresses         clientv1beta1.IngressInterface //need to use client-go IngressInterface to update Ingress.Status field
	managementContext *config.ManagementContext
}

func newGlobalDNSController(ctx context.Context, mgmt *config.ManagementContext) *GDController {
	n := &GDController{
		ingresses:         mgmt.K8sClient.ExtensionsV1beta1().Ingresses(namespace.GlobalNamespace),
		managementContext: mgmt,
	}
	return n
}

//sync is called periodically and on real updates
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

func (n *GDController) getIngressForGlobalDNS(globaldns *v3.GlobalDns) (*v1beta1.Ingress, error) {
	ingress, err := n.ingresses.Get(context.TODO(), strings.Join([]string{"globaldns-ingress", globaldns.Name}, "-"), metav1.GetOptions{}) //n.Get("", strings.Join([]string{"globaldns-ingress", globaldns.Name}, "-"))
	if err != nil {
		return nil, err
	}
	//make sure the ingress is owned by this globalDNS
	if n.isIngressOwnedByGlobalDNS(ingress, globaldns) {
		return ingress, nil
	}
	return nil, nil
}

func (n *GDController) isIngressOwnedByGlobalDNS(ingress *v1beta1.Ingress, globaldns *v3.GlobalDns) bool {
	for i, owners := 0, ingress.GetOwnerReferences(); owners != nil && i < len(owners); i++ {
		if owners[i].UID == globaldns.UID && owners[i].Kind == globaldns.Kind {
			return true
		}
	}
	return false
}

func (n *GDController) createIngressForGlobalDNS(globaldns *v3.GlobalDns) (*v1beta1.Ingress, error) {
	ingressSpec := n.generateNewIngressSpec(globaldns)
	if globaldns.Spec.TTL != 0 {
		ingressSpec.ObjectMeta.Annotations[annotationDNSTTL] = strconv.FormatInt(globaldns.Spec.TTL, 10)
	}
	ingressObj, err := n.ingresses.Create(context.TODO(), ingressSpec, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	logrus.Debugf("Created ingress %v for globalDNS %s", ingressObj.Name, globaldns.Name)
	return ingressObj, nil
}

func (n *GDController) generateNewIngressSpec(globaldns *v3.GlobalDns) *v1beta1.Ingress {
	controller := true
	return &v1beta1.Ingress{
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
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					Host: globaldns.Spec.FQDN,
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								{
									Backend: v1beta1.IngressBackend{
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

func (n *GDController) updateIngressForDNS(ingress *v1beta1.Ingress, obj *v3.GlobalDns) error {
	var err error

	if n.ifEndpointsDiffer(ingress.Status.LoadBalancer.Ingress, obj.Status.Endpoints) {
		ingress.Status.LoadBalancer.Ingress = n.sliceToStatus(obj.Status.Endpoints)
		ingress, err = n.ingresses.UpdateStatus(context.TODO(), ingress, metav1.UpdateOptions{})

		if err != nil {
			return fmt.Errorf("GlobalDNSController: Error updating Ingress %v", err)
		}
	}

	var updateIngress bool
	if len(ingress.Spec.Rules) > 0 {
		if !strings.EqualFold(ingress.Spec.Rules[0].Host, obj.Spec.FQDN) {
			ingress.Spec.Rules[0].Host = obj.Spec.FQDN
			updateIngress = true
		}
	}

	ttlvalue := strconv.FormatInt(obj.Spec.TTL, 10)
	if !strings.EqualFold(ingress.ObjectMeta.Annotations[annotationDNSTTL], ttlvalue) {
		ingress.ObjectMeta.Annotations[annotationDNSTTL] = ttlvalue
		updateIngress = true
	}

	if updateIngress {
		_, err = n.ingresses.Update(context.TODO(), ingress, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("GlobalDNSController: Error updating Ingress %v", err)
		}
	}

	return nil
}

func (n *GDController) ifEndpointsDiffer(ingressEps []apiv1.LoadBalancerIngress, endpoints []string) bool {
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

func (n *GDController) gatherIngressEndpoints(ingressEps []apiv1.LoadBalancerIngress) map[string]bool {
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
func (n *GDController) sliceToStatus(endpoints []string) []apiv1.LoadBalancerIngress {
	lbi := []apiv1.LoadBalancerIngress{}
	for _, ep := range endpoints {
		if net.ParseIP(ep) == nil {
			lbi = append(lbi, apiv1.LoadBalancerIngress{Hostname: ep})
		} else {
			lbi = append(lbi, apiv1.LoadBalancerIngress{IP: ep})
		}
	}
	return lbi
}
