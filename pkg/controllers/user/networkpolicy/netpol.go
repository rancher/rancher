package networkpolicy

import (
	"fmt"
	"net"
	"reflect"

	"github.com/rancher/rancher/pkg/controllers/user/nslabels"
	typescorev1 "github.com/rancher/types/apis/core/v1"
	rnetworkingv1 "github.com/rancher/types/apis/networking.k8s.io/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	knetworkingv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	//FlannelPresenceLabel is used to detect if a node is using flannel plugin or not
	FlannelPresenceLabel = "flannel.alpha.coreos.com/public-ip"
)

type netpolMgr struct {
	nsLister   typescorev1.NamespaceLister
	nodeLister typescorev1.NodeLister
	pods       typescorev1.PodInterface
	npLister   rnetworkingv1.NetworkPolicyLister
	npClient   rnetworkingv1.Interface
}

func (npmgr *netpolMgr) program(np *knetworkingv1.NetworkPolicy) error {
	existing, err := npmgr.npLister.Get(np.Namespace, np.Name)
	logrus.Debugf("netpolMgr: program: existing=%+v, err=%v", existing, err)
	if err != nil {
		if kerrors.IsNotFound(err) {
			logrus.Debugf("netpolMgr: program: about to create np=%+v", *np)
			_, err = npmgr.npClient.NetworkPolicies(np.Namespace).Create(np)
			if err != nil && !kerrors.IsAlreadyExists(err) && !kerrors.IsForbidden(err) {
				logrus.Errorf("netpolMgr: program: error creating network policy err=%v", err)
				return err
			}
		} else {
			logrus.Errorf("netpolMgr: program: got unexpected error while getting network policy=%v", err)
		}
	} else {
		logrus.Debugf("netpolMgr: program: existing=%+v", existing)
		if existing.DeletionTimestamp == nil && !reflect.DeepEqual(existing, np) {
			logrus.Debugf("netpolMgr: program: about to update np=%+v", *np)
			_, err = npmgr.npClient.NetworkPolicies(np.Namespace).Update(np)
			if err != nil {
				logrus.Errorf("netpolMgr: program: error updating network policy err=%v", err)
				return err
			}
		} else {
			logrus.Debugf("netpolMgr: program: no need to update np=%+v", *np)
		}
	}
	return nil
}

func (npmgr *netpolMgr) delete(policyNamespace, policyName string) error {
	existing, err := npmgr.npLister.Get(policyNamespace, policyName)
	logrus.Debugf("netpolMgr: delete: existing=%+v, err=%v", existing, err)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return nil
		}
		logrus.Errorf("netpolMgr: delete: got unexpected error while getting network policy=%v", err)
	} else {
		logrus.Debugf("netpolMgr: delete: existing=%+v", existing)
		err = npmgr.npClient.NetworkPolicies(existing.Namespace).Delete(existing.Name, &v1.DeleteOptions{})
		if err != nil {
			logrus.Errorf("netpolMgr: delete: error deleting network policy err=%v", err)
			return err
		}
	}
	return nil
}

func (npmgr *netpolMgr) programNetworkPolicy(projectID string) error {
	logrus.Debugf("netpolMgr: programNetworkPolicy: projectID=%v", projectID)
	// Get namespaces belonging to project
	set := labels.Set(map[string]string{nslabels.ProjectIDFieldLabel: projectID})
	namespaces, err := npmgr.nsLister.List("", set.AsSelector())
	if err != nil {
		logrus.Errorf("netpolMgr: programNetworkPolicy: err=%v", err)
		return fmt.Errorf("couldn't list namespaces with projectID %v err=%v", projectID, err)
	}
	logrus.Debugf("netpolMgr: programNetworkPolicy: namespaces=%+v", namespaces)

	for _, aNS := range namespaces {
		if aNS.DeletionTimestamp != nil {
			logrus.Debugf("netpolMgr: programNetworkPolicy: aNS=%+v marked for deletion, skipping", aNS)
			continue
		}
		policyName := "np-default"
		np := &knetworkingv1.NetworkPolicy{
			ObjectMeta: v1.ObjectMeta{
				Name:      policyName,
				Namespace: aNS.Name,
				Labels:    labels.Set(map[string]string{nslabels.ProjectIDFieldLabel: projectID}),
			},
			Spec: knetworkingv1.NetworkPolicySpec{
				// An empty PodSelector selects all pods in this Namespace.
				PodSelector: v1.LabelSelector{},
				Ingress: []knetworkingv1.NetworkPolicyIngressRule{
					knetworkingv1.NetworkPolicyIngressRule{
						From: []knetworkingv1.NetworkPolicyPeer{
							knetworkingv1.NetworkPolicyPeer{
								NamespaceSelector: &v1.LabelSelector{
									MatchLabels: map[string]string{nslabels.ProjectIDFieldLabel: projectID},
								},
							},
						},
					},
				},
			},
		}
		if err := npmgr.program(np); err != nil {
			logrus.Errorf("netpolMgr: programNetworkPolicy: error programming default network policy for ns=%v err=%v", aNS.Name, err)
		}
	}
	return nil
}

func (npmgr *netpolMgr) hostPortsUpdateHandler(pod *corev1.Pod) error {
	policyName := getHostPortsPolicyName(pod)
	np := &knetworkingv1.NetworkPolicy{
		ObjectMeta: v1.ObjectMeta{
			Name:      policyName,
			Namespace: pod.Namespace,
			OwnerReferences: []v1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					UID:        pod.UID,
					Name:       pod.Name,
				},
			},
		},
		Spec: knetworkingv1.NetworkPolicySpec{
			PodSelector: v1.LabelSelector{
				MatchLabels: map[string]string{PodNameFieldLabel: pod.Name},
			},
			Ingress: []knetworkingv1.NetworkPolicyIngressRule{
				knetworkingv1.NetworkPolicyIngressRule{
					From:  []knetworkingv1.NetworkPolicyPeer{},
					Ports: []knetworkingv1.NetworkPolicyPort{},
				},
			},
		},
	}

	hasHostPorts := false
	for _, c := range pod.Spec.Containers {
		for _, port := range c.Ports {
			if port.HostPort != 0 {
				hp := intstr.FromInt(int(port.ContainerPort))
				proto := corev1.Protocol(port.Protocol)
				p := knetworkingv1.NetworkPolicyPort{
					Protocol: &proto,
					Port:     &hp,
				}
				np.Spec.Ingress[0].Ports = append(np.Spec.Ingress[0].Ports, p)
				hasHostPorts = true
			}
		}
	}
	if hasHostPorts {
		logrus.Debugf("netpolMgr: hostPortsUpdateHandler: pod=%+v has host ports, hence programming np=%+v", *pod, *np)
		return npmgr.program(np)
	}

	return nil
}

func (npmgr *netpolMgr) hostPortsRemoveHandler(pod *corev1.Pod) (*corev1.Pod, error) {
	return pod, npmgr.delete(pod.Namespace, getHostPortsPolicyName(pod))
}

func getHostPortsPolicyName(pod *corev1.Pod) string {
	return "hp-" + pod.Name
}

func getNodePortsPolicyName(service *corev1.Service) string {
	return "np-" + service.Name
}

func (npmgr *netpolMgr) nodePortsUpdateHandler(service *corev1.Service) error {
	policyName := getNodePortsPolicyName(service)
	np := &knetworkingv1.NetworkPolicy{
		ObjectMeta: v1.ObjectMeta{
			Name:      policyName,
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
				knetworkingv1.NetworkPolicyIngressRule{
					From:  []knetworkingv1.NetworkPolicyPeer{},
					Ports: []knetworkingv1.NetworkPolicyPort{},
				},
			},
		},
	}

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
	if hasNodePorts {
		logrus.Debugf("netpolMgr: nodePortsUpdateHandler: service=%+v has node ports, hence programming np=%+v", *service, *np)
		return npmgr.program(np)
	}

	return nil
}

func (npmgr *netpolMgr) handleHostNetwork() error {
	policyName := "hn-nodes"
	np := &knetworkingv1.NetworkPolicy{
		ObjectMeta: v1.ObjectMeta{
			Name: policyName,
		},
		Spec: knetworkingv1.NetworkPolicySpec{
			PodSelector: v1.LabelSelector{},
			Ingress: []knetworkingv1.NetworkPolicyIngressRule{
				knetworkingv1.NetworkPolicyIngressRule{
					From: []knetworkingv1.NetworkPolicyPeer{},
				},
			},
		},
	}

	nodes, err := npmgr.nodeLister.List("", labels.Everything())
	if err != nil {
		return fmt.Errorf("couldn't list nodes err=%v", err)
	}
	logrus.Debugf("netpolMgr: handleHostNetwork: nodes=%+v", nodes)

	for _, node := range nodes {
		logrus.Debugf("netpolMgr: handleHostNetwork: node=%+v", node)
		if _, ok := node.Annotations[FlannelPresenceLabel]; !ok {
			logrus.Debugf("netpolMgr: handleHostNetwork: node=%v doesn't have flannel label, skipping", node.Name)
			continue
		}
		podCIDRFirstIP, _, err := net.ParseCIDR(node.Spec.PodCIDR)
		if err != nil {
			logrus.Errorf("netpolMgr: handleHostNetwork: couldn't parse PodCIDR(%v) err=%v", node.Spec.PodCIDR, err)
			continue
		}
		ipBlock := knetworkingv1.IPBlock{
			CIDR:   podCIDRFirstIP.String() + "/32",
			Except: []string{},
		}
		np.Spec.Ingress[0].From = append(np.Spec.Ingress[0].From, knetworkingv1.NetworkPolicyPeer{IPBlock: &ipBlock})
	}

	namespaces, err := npmgr.nsLister.List("", labels.Everything())
	if err != nil {
		return fmt.Errorf("couldn't list namespaces err=%v", err)
	}

	for _, aNS := range namespaces {
		if aNS.DeletionTimestamp != nil || aNS.Status.Phase == corev1.NamespaceTerminating {
			logrus.Debugf("netpolMgr: handleHostNetwork: aNS=%+v marked for deletion/termination, skipping", aNS)
			continue
		}
		logrus.Debugf("netpolMgr: handleHostNetwork: aNS=%+v", aNS)
		if _, ok := aNS.Labels[nslabels.ProjectIDFieldLabel]; !ok {
			continue
		}
		np.OwnerReferences = []v1.OwnerReference{
			{
				APIVersion: "v1",
				Kind:       "Namespace",
				UID:        aNS.UID,
				Name:       aNS.Name,
			},
		}
		np.Namespace = aNS.Name
		if err := npmgr.program(np); err != nil {
			logrus.Errorf("netpolMgr: handleHostNetwork: error programming hostNetwork network policy for ns=%v err=%v", aNS.Name, err)
		}
	}
	return nil
}
