package networkpolicy

import (
	"fmt"

	"sort"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	knetworkingv1 "k8s.io/api/networking/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type serviceHandler struct {
	npmgr            *netpolMgr
	clusterLister    v3.ClusterLister
	clusterNamespace string
}

func (sh *serviceHandler) Sync(key string, service *corev1.Service) (runtime.Object, error) {
	if service == nil || service.DeletionTimestamp != nil {
		return nil, nil
	}
	disabled, err := isNetworkPolicyDisabled(sh.clusterNamespace, sh.clusterLister)
	if err != nil {
		return nil, err
	}
	if disabled {
		return nil, nil
	}
	moved, err := isNamespaceMoved(service.Namespace, sh.npmgr.nsLister)
	if err != nil {
		return nil, err
	}
	if moved {
		return nil, nil
	}
	logrus.Debugf("serviceHandler: Sync: %+v", *service)
	return nil, sh.npmgr.nodePortsUpdateHandler(service, sh.clusterNamespace)
}

func (npmgr *netpolMgr) nodePortsUpdateHandler(service *corev1.Service, clusterNamespace string) error {
	systemNamespaces, _, err := npmgr.getSystemNSInfo(clusterNamespace)
	if err != nil {
		return fmt.Errorf("netpolMgr: hostPortsUpdateHandler: getSystemNamespaces: err=%v", err)
	}
	policyName := getNodePortsPolicyName(service)
	if _, ok := systemNamespaces[service.Namespace]; ok {
		npmgr.delete(service.Namespace, policyName)
		return nil
	}
	np := generateServiceNetworkPolicy(service, policyName)
	hasNodePorts := false
	for _, port := range service.Spec.Ports {
		if port.NodePort != 0 {
			tp := port.TargetPort
			proto := corev1.Protocol(port.Protocol)
			p := knetworkingv1.NetworkPolicyPort{
				Protocol: &proto,
				Port:     &tp,
			}
			np.Spec.Ingress[0].Ports = append(np.Spec.Ingress[0].Ports, p)
			hasNodePorts = true
		}
	}

	// sort ports so it always appears in a certain order
	sort.Slice(np.Spec.Ingress[0].Ports, func(i, j int) bool {
		return portToString(np.Spec.Ingress[0].Ports[i]) < portToString(np.Spec.Ingress[0].Ports[j])
	})
	if hasNodePorts {
		logrus.Debugf("netpolMgr: nodePortsUpdateHandler: service=%+v has node ports, hence programming np=%+v", *service, *np)
		return npmgr.program(np)
	}

	return nil
}

func getNodePortsPolicyName(service *corev1.Service) string {
	return "np-" + service.Name
}

func generateServiceNetworkPolicy(service *corev1.Service, policyName string) *knetworkingv1.NetworkPolicy {
	np := &knetworkingv1.NetworkPolicy{
		ObjectMeta: v1.ObjectMeta{
			Name:      policyName,
			Namespace: service.Namespace,
			Labels: map[string]string{
				creatorLabel: creatorNorman,
			},
			OwnerReferences: []v1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Service",
					UID:        service.UID,
					Name:       service.Name,
				},
			},
		},
		Spec: knetworkingv1.NetworkPolicySpec{
			PodSelector: v1.LabelSelector{
				MatchLabels: service.Spec.Selector,
			},
			Ingress: []knetworkingv1.NetworkPolicyIngressRule{
				{
					From:  []knetworkingv1.NetworkPolicyPeer{},
					Ports: []knetworkingv1.NetworkPolicyPort{},
				},
			},
		},
	}
	return np
}
