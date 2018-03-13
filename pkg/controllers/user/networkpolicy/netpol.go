package networkpolicy

import (
	"fmt"
	"net"
	"reflect"

	"github.com/rancher/rancher/pkg/controllers/user/nslabels"
	typescorev1 "github.com/rancher/types/apis/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

const (
	//FlannelPresenceLabel is used to detect if a node is using flannel plugin or not
	FlannelPresenceLabel = "flannel.alpha.coreos.com/public-ip"
)

type netpolMgr struct {
	nsLister   typescorev1.NamespaceLister
	nodeLister typescorev1.NodeLister
	pods       typescorev1.PodInterface
	k8sClient  kubernetes.Interface
}

func (npmgr *netpolMgr) program(np *networkingv1.NetworkPolicy) error {
	existing, err := npmgr.k8sClient.NetworkingV1().NetworkPolicies(np.Namespace).Get(np.Name, v1.GetOptions{})
	logrus.Debugf("netpolMgr: program: existing=%+v, err=%v", existing, err)
	if err != nil {
		if kerrors.IsNotFound(err) {
			logrus.Debugf("about to create np=%+v", *np)
			_, err = npmgr.k8sClient.NetworkingV1().NetworkPolicies(np.Namespace).Create(np)
			if err != nil {
				logrus.Errorf("netpolMgr: program: error creating network policy err=%v", err)
				return err
			}
		} else {
			logrus.Errorf("netpolMgr: program: got unexpected error while getting network policy=%v", err)
		}
	} else {
		logrus.Debugf("netpolMgr: program: existing=%+v", existing)
		if !reflect.DeepEqual(existing, np) {
			logrus.Debugf("about to update np=%+v", *np)
			_, err = npmgr.k8sClient.NetworkingV1().NetworkPolicies(np.Namespace).Update(np)
			if err != nil {
				logrus.Errorf("netpolMgr: program: error updating network policy err=%v", err)
				return err
			}
		} else {
			logrus.Debugf("no need to update np=%+v", *np)
		}
	}
	return nil
}

func (npmgr *netpolMgr) delete(policyNamespace, policyName string) error {
	existing, err := npmgr.k8sClient.NetworkingV1().NetworkPolicies(policyNamespace).Get(policyName, v1.GetOptions{})
	logrus.Debugf("netpolMgr: delete: existing=%+v, err=%v", existing, err)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return nil
		}
		logrus.Errorf("netpolMgr: delete: got unexpected error while getting network policy=%v", err)
	} else {
		logrus.Debugf("netpolMgr: delete: existing=%+v", existing)
		err = npmgr.k8sClient.NetworkingV1().NetworkPolicies(existing.Namespace).Delete(existing.Name, &v1.DeleteOptions{})
		if err != nil {
			logrus.Errorf("netpolMgr: delete: error deleting network policy err=%v", err)
			return err
		}
	}
	return nil
}

func (npmgr *netpolMgr) programNetworkPolicy(projectID string) error {
	logrus.Debugf("programNetworkPolicy: projectID=%v", projectID)
	// Get namespaces belonging to project
	set := labels.Set(map[string]string{nslabels.ProjectIDFieldLabel: projectID})
	namespaces, err := npmgr.nsLister.List("", set.AsSelector())
	if err != nil {
		logrus.Errorf("programNetworkPolicy err=%v", err)
		return fmt.Errorf("couldn't list namespaces with projectID %v err=%v", projectID, err)
	}
	logrus.Debugf("namespaces=%+v", namespaces)

	for _, aNS := range namespaces {
		policyName := "np-default"
		np := &networkingv1.NetworkPolicy{
			ObjectMeta: v1.ObjectMeta{
				Name:      policyName,
				Namespace: aNS.Name,
				Labels:    labels.Set(map[string]string{nslabels.ProjectIDFieldLabel: projectID}),
			},
			Spec: networkingv1.NetworkPolicySpec{
				// An empty PodSelector selects all pods in this Namespace.
				PodSelector: v1.LabelSelector{},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					networkingv1.NetworkPolicyIngressRule{
						From: []networkingv1.NetworkPolicyPeer{
							networkingv1.NetworkPolicyPeer{
								NamespaceSelector: &v1.LabelSelector{
									MatchLabels: map[string]string{nslabels.ProjectIDFieldLabel: projectID},
								},
							},
						},
					},
				},
			},
		}

		return npmgr.program(np)
	}
	return nil
}

func (npmgr *netpolMgr) hostPortsUpdateHandler(pod *corev1.Pod) error {
	policyName := getHostPortsPolicyName(pod)
	np := &networkingv1.NetworkPolicy{
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
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: v1.LabelSelector{
				MatchLabels: map[string]string{PodNameFieldLabel: pod.Name},
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				networkingv1.NetworkPolicyIngressRule{
					From:  []networkingv1.NetworkPolicyPeer{},
					Ports: []networkingv1.NetworkPolicyPort{},
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
				p := networkingv1.NetworkPolicyPort{
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
	np := &networkingv1.NetworkPolicy{
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
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: v1.LabelSelector{
				MatchLabels: service.Spec.Selector,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				networkingv1.NetworkPolicyIngressRule{
					From:  []networkingv1.NetworkPolicyPeer{},
					Ports: []networkingv1.NetworkPolicyPort{},
				},
			},
		},
	}

	hasNodePorts := false
	for _, port := range service.Spec.Ports {
		if port.NodePort != 0 {
			tp := port.TargetPort
			proto := corev1.Protocol(port.Protocol)
			p := networkingv1.NetworkPolicyPort{
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
	np := &networkingv1.NetworkPolicy{
		ObjectMeta: v1.ObjectMeta{
			Name: policyName,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: v1.LabelSelector{},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				networkingv1.NetworkPolicyIngressRule{
					From: []networkingv1.NetworkPolicyPeer{},
				},
			},
		},
	}

	nodes, err := npmgr.nodeLister.List("", labels.Everything())
	if err != nil {
		return fmt.Errorf("couldn't list nodes err=%v", err)
	}
	logrus.Debugf("handleHostNetwork: nodes=%+v", nodes)

	for _, node := range nodes {
		// TODO: Ask @ibuildthecloud if I need to skip
		// a node marked for deletion?
		logrus.Debugf("node=%+v", node)
		if _, ok := node.Annotations[FlannelPresenceLabel]; !ok {
			logrus.Debugf("node=%v doesn't have flannel label, skipping", node.Name)
			continue
		}
		podCIDRFirstIP, _, err := net.ParseCIDR(node.Spec.PodCIDR)
		if err != nil {
			logrus.Errorf("couldn't parse PodCIDR(%v) err=%v", node.Spec.PodCIDR, err)
			continue
		}
		ipBlock := networkingv1.IPBlock{
			CIDR:   podCIDRFirstIP.String() + "/32",
			Except: []string{},
		}
		np.Spec.Ingress[0].From = append(np.Spec.Ingress[0].From, networkingv1.NetworkPolicyPeer{IPBlock: &ipBlock})
	}

	namespaces, err := npmgr.nsLister.List("", labels.Everything())
	if err != nil {
		return fmt.Errorf("couldn't list namespaces err=%v", err)
	}

	for _, aNS := range namespaces {
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
			logrus.Errorf("error programming hostNetwork network policy for ns=%v err=%v", aNS.Name, err)
		}
	}
	return nil
}
