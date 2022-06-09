package networkpolicy

import (
	"fmt"
	"net"
	"reflect"
	"sort"

	"github.com/rancher/rancher/pkg/controllers/managementagent/nslabels"
	cluster2 "github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	rancherv1 "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	typescorev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	rnetworkingv1 "github.com/rancher/rancher/pkg/generated/norman/networking.k8s.io/v1"
	rkecluster "github.com/rancher/rke/cluster"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	knetworkingv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

const (
	systemProjectLabel = "authz.management.cattle.io/system-project"
	creatorLabel       = "cattle.io/creator"
)

const (
	defaultNamespacePolicyName              = "np-default"
	defaultSystemProjectNamespacePolicyName = "np-default-allow-all"
	hostNetworkPolicyName                   = "hn-nodes"
	creatorNorman                           = "norman"
)

type netpolMgr struct {
	clusterLister    v3.ClusterLister
	clusters         rancherv1.ClusterCache
	nsLister         typescorev1.NamespaceLister
	nodeLister       typescorev1.NodeLister
	pods             typescorev1.PodInterface
	projects         v3.ProjectInterface
	npLister         rnetworkingv1.NetworkPolicyLister
	npClient         rnetworkingv1.Interface
	projLister       v3.ProjectLister
	clusterNamespace string
}

func (npmgr *netpolMgr) program(np *knetworkingv1.NetworkPolicy) error {
	existing, err := npmgr.npLister.Get(np.Namespace, np.Name)
	logrus.Debugf("netpolMgr: program: existing=%+v, err=%v", existing, err)
	if err != nil {
		if kerrors.IsNotFound(err) {
			logrus.Debugf("netpolMgr: program: about to create np=%+v", *np)
			_, err = npmgr.npClient.NetworkPolicies(np.Namespace).Create(np)
			if err != nil && !kerrors.IsAlreadyExists(err) && !kerrors.IsForbidden(err) {
				return fmt.Errorf("netpolMgr: program: error creating network policy err=%v", err)
			}
		} else {
			return fmt.Errorf("netpolMgr: program: got unexpected error while getting network policy=%v", err)
		}
	} else {
		logrus.Debugf("netpolMgr: program: existing=%+v", existing)
		if existing.DeletionTimestamp == nil && !reflect.DeepEqual(existing.Spec, np.Spec) {
			logrus.Debugf("netpolMgr: program: about to update np=%+v", *np)
			_, err = npmgr.npClient.NetworkPolicies(np.Namespace).Update(np)
			if err != nil {
				return fmt.Errorf("netpolMgr: program: error updating network policy err=%v", err)
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
		return fmt.Errorf("netpolMgr: delete: got unexpected error while getting network policy=%v", err)
	}
	logrus.Debugf("netpolMgr: delete: existing=%+v", existing)
	err = npmgr.npClient.NetworkPolicies(existing.Namespace).Delete(existing.Name, &v1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("netpolMgr: delete: error deleting network policy err=%v", err)
	}
	return nil
}

func (npmgr *netpolMgr) programNetworkPolicy(projectID string, clusterNamespace string) error {
	logrus.Debugf("netpolMgr: programNetworkPolicy: projectID=%v", projectID)
	// Get namespaces belonging to project
	set := labels.Set(map[string]string{nslabels.ProjectIDFieldLabel: projectID})
	namespaces, err := npmgr.nsLister.List("", set.AsSelector())
	if err != nil {
		return fmt.Errorf("netpolMgr: couldn't list namespaces with projectID %v err=%v", projectID, err)
	}
	logrus.Debugf("netpolMgr: programNetworkPolicy: namespaces=%+v", namespaces)

	systemNamespaces, systemProjectID, err := npmgr.getSystemNSInfo(clusterNamespace)
	if err != nil {
		return fmt.Errorf("netpolMgr: programNetworkPolicy getSystemNamespaces: err=%v", err)
	}

	for _, aNS := range namespaces {
		id, _ := aNS.Labels[nslabels.ProjectIDFieldLabel]

		// add an allow all network policy to system project namespaces
		// this is the same as having no network policy, i.e. it allows all ingress and egress traffic to/from the namespace
		// this is needed to ensure CIS Scans pass. See: https://github.com/rancher/rancher/issues/30211 for more info
		// we also guard against overriding existing network policies, the default network policy for a namespace in the system project
		// will only be added if there are no other network policies in the namespace (network policies are additive)
		if systemNamespaces[aNS.Name] {
			npmgr.delete(aNS.Name, defaultNamespacePolicyName)

			// this requirement includes objects with no creatorLabel or a value != creatorNorman
			labelReq, err := labels.NewRequirement(creatorLabel, selection.NotEquals, []string{creatorNorman})
			if err != nil { // this won't happen since creatorLabel and creatorNorman are constants and valid per validation rules in labels.NewRequirement
				return err
			}
			// select user network policies, which are ones that don't have a label with creator == norman
			nps, err := npmgr.npLister.List(aNS.Name, labels.NewSelector().Add(*labelReq))
			if err != nil {
				return err
			}

			// there are existing network policies in this system project based namespace, skip programming default
			if len(nps) > 0 {
				logrus.Debugf("netPolMgr: namespace=%s in project=%s has existing network policies, skipping programming %s", aNS.Name, id, defaultSystemProjectNamespacePolicyName)
				continue
			}

			// program default network policy for system project based namespace
			logrus.Debugf("netPolMgr: programming %s for namespace=%s in project=%s", defaultSystemProjectNamespacePolicyName, aNS.Name, id)
			if err := npmgr.program(generateAllowAllNetworkPolicy(aNS, systemProjectID)); err != nil {
				return fmt.Errorf(
					"netPolMgr: programNetworkPolicy: error programming network policy %s for system project based namespace=%s err=%v",
					defaultSystemProjectNamespacePolicyName, aNS.Name, err,
				)
			}
			continue
		}

		// namespace is not in system project, so ensure it doesn't have the default policy for system project based namespaces
		if id != systemProjectID {
			npmgr.delete(aNS.Name, defaultSystemProjectNamespacePolicyName)
		}
		if id == "" {
			npmgr.delete(aNS.Name, defaultNamespacePolicyName)
			continue
		}
		if aNS.DeletionTimestamp != nil {
			logrus.Debugf("netpolMgr: programNetworkPolicy: aNS=%+v marked for deletion, skipping", aNS)
			continue
		}

		np := generateDefaultNamespaceNetworkPolicy(aNS, projectID, systemProjectID)
		if err := npmgr.program(np); err != nil {
			return fmt.Errorf("netpolMgr: programNetworkPolicy: error programming default network policy for ns=%v err=%v", aNS.Name, err)
		}
	}
	return nil
}

const (
	calicoIPIPTunnelAddrAnno  = "projectcalico.org/IPv4IPIPTunnelAddr"
	calicoVXLANTunnelAddrAnno = "projectcalico.org/IPv4VXLANTunnelAddr"
)

func (npmgr *netpolMgr) handleHostNetwork(clusterNamespace string) error {
	nodes, err := npmgr.nodeLister.List("", labels.Everything())
	if err != nil {
		return fmt.Errorf("couldn't list nodes err=%v", err)
	}

	cni, err := npmgr.getClusterCNI(clusterNamespace)
	if err != nil {
		return err
	}

	logrus.Debugf("netpolMgr: handleHostNetwork: processing %d nodes", len(nodes))
	np := generateNodesNetworkPolicy()

	// This for loop builds CNI specific host network policies to allow traffic from ingress controllers through to endpoints.
	// Calico nodes require special handling. Calico has the following routing modes: IP in IP, VXLAN, and Direct.
	// RKE1 deploys calico with IP in IP routing, while RKE2 deploys calico with VXLAN routing.
	// When calico sends traffic to pods, the source IP address changes to either the IPIP tunnel address or the VXLAN tunnel address,
	// depending on the routing mode. These are the IPs that the host network policy needs to use.
	var policies []knetworkingv1.NetworkPolicyPeer
	for _, node := range nodes {
		// for calico, we get the IPs for the hn-nodes policy from the tunnel address annotations, which are managed by calico-node
		if cni == rkecluster.CalicoNetworkPlugin { // RKE1/2 use the same "calico" value
			tunnelAddr, ok := node.Annotations[calicoIPIPTunnelAddrAnno]
			if !ok {
				tunnelAddr = node.Annotations[calicoVXLANTunnelAddrAnno]
			}
			if tunnelAddr == "" {
				logrus.Debugf("netpolMgr: handleHostNetwork: calico: node=%+v", node)
				logrus.Errorf("netpolMgr: handleHostNetwork: calico: couldn't get tunnel address for node %v err=%v", node.Name, err)
				continue
			}
			ipBlock := knetworkingv1.IPBlock{
				CIDR: tunnelAddr + "/32",
			}
			policies = append(policies, knetworkingv1.NetworkPolicyPeer{IPBlock: &ipBlock})
			continue
		}

		// other CNIs
		podCIDRFirstIP, _, err := net.ParseCIDR(node.Spec.PodCIDR)
		if err != nil {
			logrus.Debugf("netpolMgr: handleHostNetwork: node=%+v", node)
			logrus.Errorf("netpolMgr: handleHostNetwork: couldn't parse PodCIDR(%v) for node %v err=%v", node.Spec.PodCIDR, node.Name, err)
			continue
		}
		ipBlock := knetworkingv1.IPBlock{
			CIDR: podCIDRFirstIP.String() + "/32",
		}
		policies = append(policies, knetworkingv1.NetworkPolicyPeer{IPBlock: &ipBlock})
	}

	// set the policies on the resource
	np.Spec.Ingress[0].From = policies

	// An empty ingress rule allows all traffic to the namespace
	// so we need to skip creating the network policy here if that's what we have.
	if len(np.Spec.Ingress[0].From) == 0 {
		logrus.Debugf("netpolMgr: handleHostNetwork: no host addresses found, skipping programming the %s policy", hostNetworkPolicyName)
		return nil
	}

	// sort ipblocks so it always appears in a certain order
	sort.Slice(np.Spec.Ingress[0].From, func(i, j int) bool {
		return np.Spec.Ingress[0].From[i].IPBlock.CIDR < np.Spec.Ingress[0].From[j].IPBlock.CIDR
	})

	namespaces, err := npmgr.nsLister.List("", labels.Everything())
	if err != nil {
		return fmt.Errorf("couldn't list namespaces err=%v", err)
	}

	systemNamespaces, _, err := npmgr.getSystemNSInfo(clusterNamespace)
	if err != nil {
		return fmt.Errorf("netpolMgr: handleHostNetwork getSystemNamespaces: err=%v", err)
	}
	for _, aNS := range namespaces {
		projectID, _ := aNS.Labels[nslabels.ProjectIDFieldLabel]
		if systemNamespaces[aNS.Name] || projectID == "" {
			npmgr.delete(aNS.Name, hostNetworkPolicyName)
			continue
		}
		if aNS.DeletionTimestamp != nil || aNS.Status.Phase == corev1.NamespaceTerminating {
			logrus.Debugf("netpolMgr: handleHostNetwork: aNS=%+v marked for deletion/termination, skipping", aNS)
			continue
		}
		if _, ok := aNS.Labels[nslabels.ProjectIDFieldLabel]; !ok {
			continue
		}

		logrus.Debugf("netpolMgr: handleHostNetwork: aNS=%+v", aNS)

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

// getClusterCNI returns the cluster's CNI name if it is found or an api error
func (npmgr *netpolMgr) getClusterCNI(clusterName string) (string, error) {
	cluster, err := npmgr.clusterLister.Get("", clusterName)
	if err != nil {
		return "", err
	}

	if cluster.Spec.Rke2Config != nil {
		return npmgr.getRKE2ClusterCNI(cluster)
	}

	if cluster.Spec.RancherKubernetesEngineConfig != nil {
		return cluster.Spec.RancherKubernetesEngineConfig.Network.Plugin, nil
	}

	return "", nil
}

// getRKE2ClusterCNI returns the RKE2 cluster CNI name if it is found, or an api error
func (npmgr *netpolMgr) getRKE2ClusterCNI(mgmtCluster *v3.Cluster) (string, error) {
	clusters, err := npmgr.clusters.GetByIndex(cluster2.ByCluster, mgmtCluster.Name)
	if err != nil {
		return "", err
	}
	if len(clusters) != 1 {
		return "", fmt.Errorf("could not map to v1.Cluster for v3.Cluster: %s", mgmtCluster.Name)
	}

	cluster := clusters[0]
	if rke := cluster.Spec.RKEConfig; rke != nil && rke.MachineGlobalConfig.Data["cni"] != "" {
		cni, ok := rke.MachineGlobalConfig.Data["cni"].(string)
		if !ok {
			return "", nil
		}
		return cni, nil
	}

	return "", nil
}

func (npmgr *netpolMgr) getSystemNSInfo(clusterNamespace string) (map[string]bool, string, error) {
	systemNamespaces := map[string]bool{}
	set := labels.Set(map[string]string{systemProjectLabel: "true"})
	projects, err := npmgr.projLister.List(clusterNamespace, set.AsSelector())
	systemProjectID := ""
	if err != nil {
		return nil, systemProjectID, err
	}
	if len(projects) == 0 {
		return systemNamespaces, systemProjectID,
			fmt.Errorf("systemNamespaces: no system project for cluster %s", clusterNamespace)
	}
	if len(projects) > 1 {
		return systemNamespaces, systemProjectID,
			fmt.Errorf("systemNamespaces: more than one system project in cluster %s", clusterNamespace)
	}
	// ns.Annotations[projectIDAnnotation] = fmt.Sprintf("%v:%v", n.m.clusterName, projects[0].Name)
	// ns.Labels[ProjectIDFieldLabel] = projectID / projects[0].Name
	systemProjectID = projects[0].Name
	if systemProjectID == "" {
		return nil, systemProjectID, fmt.Errorf("sytemNamespaces: system project id cannot be empty")
	}
	set = labels.Set(map[string]string{nslabels.ProjectIDFieldLabel: systemProjectID})
	namespaces, err := npmgr.nsLister.List("", set.AsSelector())
	if err != nil {
		return nil, systemProjectID,
			fmt.Errorf("sytemNamespaces: couldn't list namespaces err=%v", err)
	}
	for _, ns := range namespaces {
		if _, ok := systemNamespaces[ns.Name]; !ok {
			systemNamespaces[ns.Name] = true
		}
	}
	return systemNamespaces, systemProjectID, nil
}

func generateDefaultNamespaceNetworkPolicy(aNS *corev1.Namespace, projectID string, systemProjectID string) *knetworkingv1.NetworkPolicy {
	return &knetworkingv1.NetworkPolicy{
		ObjectMeta: v1.ObjectMeta{
			Name:      defaultNamespacePolicyName,
			Namespace: aNS.Name,
			Labels: map[string]string{
				nslabels.ProjectIDFieldLabel: projectID,
				creatorLabel:                 creatorNorman,
			},
		},
		Spec: knetworkingv1.NetworkPolicySpec{
			// An empty PodSelector selects all pods in this Namespace.
			PodSelector: v1.LabelSelector{},
			Ingress: []knetworkingv1.NetworkPolicyIngressRule{
				{
					From: []knetworkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &v1.LabelSelector{
								MatchLabels: map[string]string{nslabels.ProjectIDFieldLabel: projectID},
							},
						},
						{
							NamespaceSelector: &v1.LabelSelector{
								MatchLabels: map[string]string{nslabels.ProjectIDFieldLabel: systemProjectID},
							},
						},
					},
				},
			},
			PolicyTypes: []knetworkingv1.PolicyType{
				knetworkingv1.PolicyTypeIngress,
			},
		},
	}
}

func generateAllowAllNetworkPolicy(ns *corev1.Namespace, systemProjectID string) *knetworkingv1.NetworkPolicy {
	return &knetworkingv1.NetworkPolicy{
		ObjectMeta: v1.ObjectMeta{
			Name:      defaultSystemProjectNamespacePolicyName,
			Namespace: ns.Name,
			Labels: map[string]string{
				nslabels.ProjectIDFieldLabel: systemProjectID,
				creatorLabel:                 creatorNorman,
			},
		},
		Spec: knetworkingv1.NetworkPolicySpec{
			// An empty PodSelector selects all pods in this Namespace.
			PodSelector: v1.LabelSelector{},
			Ingress: []knetworkingv1.NetworkPolicyIngressRule{
				{},
			},
			Egress: []knetworkingv1.NetworkPolicyEgressRule{
				{},
			},
			PolicyTypes: []knetworkingv1.PolicyType{
				knetworkingv1.PolicyTypeIngress,
				knetworkingv1.PolicyTypeEgress,
			},
		},
	}
}

func generateNodesNetworkPolicy() *knetworkingv1.NetworkPolicy {
	return &knetworkingv1.NetworkPolicy{
		ObjectMeta: v1.ObjectMeta{
			Name: hostNetworkPolicyName,
			Labels: map[string]string{
				creatorLabel: creatorNorman,
			},
		},
		Spec: knetworkingv1.NetworkPolicySpec{
			PodSelector: v1.LabelSelector{},
			Ingress: []knetworkingv1.NetworkPolicyIngressRule{
				{
					From: []knetworkingv1.NetworkPolicyPeer{},
				},
			},
			PolicyTypes: []knetworkingv1.PolicyType{
				knetworkingv1.PolicyTypeIngress,
			},
		},
	}
}

func portToString(port knetworkingv1.NetworkPolicyPort) string {
	return fmt.Sprintf("%v/%v", port.Port, port.Protocol)
}
