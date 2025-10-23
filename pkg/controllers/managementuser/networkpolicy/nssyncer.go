package networkpolicy

import (
	"fmt"

	"github.com/rancher/rancher/pkg/controllers/managementagent/nslabels"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	wcore "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type nsSyncer struct {
	npmgr            *netpolMgr
	clusterLister    v3.ClusterLister
	serviceLister    wcore.ServiceCache
	podLister        wcore.PodCache
	serviceClient    wcore.ServiceController
	podClient        wcore.PodController
	clusterNamespace string
}

// Sync invokes Policy Handler to program the native network policies
func (nss *nsSyncer) Sync(_ string, ns *corev1.Namespace) (*corev1.Namespace, error) {
	if ns == nil || ns.DeletionTimestamp != nil {
		return nil, nil
	}

	disabled, err := isNetworkPolicyDisabled(nss.clusterNamespace, nss.clusterLister)
	if err != nil {
		return nil, err
	}
	if disabled {
		return nil, nil
	}

	logrus.Debugf("nsSyncer: Sync: %v, %+v", ns.Name, *ns)

	projectID := ns.Labels[nslabels.ProjectIDFieldLabel]
	movedToNone := projectID == ""
	if !movedToNone {
		logrus.Debugf("nsSyncer: Sync: ns=%v projectID=%v", ns.Name, projectID)
		// program project isolation network policy
		if err := nss.npmgr.programNetworkPolicy(projectID, nss.clusterNamespace); err != nil {
			return nil, fmt.Errorf("nsSyncer: Sync: error programming network policy: %v (ns=%v, projectID=%v), ", err, ns.Name, projectID)
		}
	}
	// handle moving of namespace between projects
	if err := nss.syncOnMove(ns.Name, projectID, movedToNone); err != nil {
		return nil, fmt.Errorf("nsSyncer: Sync: error moving network policy: %v (ns=%v, projectID=%v), ", err, ns.Name, projectID)
	}
	if movedToNone {
		return nil, nil
	}

	// handle if hostNetwork policy is needed
	return nil, nss.npmgr.handleHostNetwork(nss.clusterNamespace)
}

func (nss *nsSyncer) syncOnMove(nsName string, projectID string, movedToNone bool) error {
	systemNamespaces, _, err := nss.npmgr.getSystemNSInfo(nss.clusterNamespace)
	if err != nil {
		return fmt.Errorf("nsSyncer: error getting systemNamespaces %v", err)
	}
	if movedToNone {
		nss.npmgr.delete(nsName, defaultNamespacePolicyName)
		nss.npmgr.delete(nsName, hostNetworkPolicyName)
		nss.npmgr.delete(nsName, defaultSystemProjectNamespacePolicyName)
	}
	if err = nss.syncNodePortServices(systemNamespaces, nsName, movedToNone); err != nil {
		return fmt.Errorf("nsSyncer: error syncing services %v", err)
	}
	if err = nss.syncHostPortPods(systemNamespaces, nsName, movedToNone); err != nil {
		return fmt.Errorf("nsSyncer: error syncing pods %v", err)
	}
	return nil
}

func (nss *nsSyncer) syncNodePortServices(systemNamespaces map[string]bool, nsName string, moved bool) error {
	svcs, err := nss.serviceLister.List(nsName, labels.NewSelector())
	if err != nil {
		return err
	}
	for _, svc := range svcs {
		if systemNamespaces[svc.Namespace] || moved {
			policyName := getNodePortsPolicyName(svc)
			nss.npmgr.delete(svc.Namespace, policyName)
			continue
		}
		if nodePortService(svc) {
			nss.serviceClient.Enqueue(svc.Namespace, svc.Name)
		}
	}
	return nil
}

func (nss *nsSyncer) syncHostPortPods(systemNamespaces map[string]bool, nsName string, moved bool) error {
	pods, err := nss.podLister.List(nsName, labels.NewSelector())
	if err != nil {
		return err
	}
	for _, pod := range pods {
		if systemNamespaces[pod.Namespace] || moved {
			policyName := getHostPortsPolicyName(pod)
			nss.npmgr.delete(pod.Namespace, policyName)
			continue
		}
		if hostPortPod(pod) {
			nss.podClient.Enqueue(pod.Namespace, pod.Name)
		}
	}
	return nil
}
