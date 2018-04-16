package networkpolicy

import (
	"sort"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	knetworkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

type serviceHandler struct {
	npmgr            *netpolMgr
	clusterNamespace string
}

func (sh *serviceHandler) Sync(key string, service *corev1.Service) error {
	if service == nil || service.DeletionTimestamp != nil {
		return nil
	}
	logrus.Debugf("serviceHandler: Sync: %+v", *service)
	return sh.nodePortsUpdateHandler(service)
}

func (sh *serviceHandler) nodePortsUpdateHandler(service *corev1.Service) error {
	np := getServiceNetworkPolicy(service)
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
		return sh.npmgr.program(np)
	}

	return nil
}

func getServiceNetworkPolicy(service *corev1.Service) *knetworkingv1.NetworkPolicy {
	np := &knetworkingv1.NetworkPolicy{
		ObjectMeta: v1.ObjectMeta{
			Name:      "np-" + service.Name,
			Namespace: service.Namespace,
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
