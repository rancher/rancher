package networkpolicy

import (
	"fmt"

	"github.com/rancher/norman/types/convert"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/managementuser/nodesyncer"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

type clusterHandler struct {
	cluster          *config.UserContext
	pnpLister        v3.ProjectNetworkPolicyLister
	podLister        v1.PodLister
	serviceLister    v1.ServiceLister
	pLister          v3.ProjectLister
	clusters         v3.ClusterInterface
	pnps             v3.ProjectNetworkPolicyInterface
	npmgr            *netpolMgr
	clusterNamespace string
}

/*
clusterHandler enqueues resources for creating/deleting network policies
based on cluster.Annotations[netPolAnnotation] and sets status if successful
*/

func (ch *clusterHandler) Sync(key string, cluster *v3.Cluster) (runtime.Object, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil ||
		cluster.Name != ch.clusterNamespace ||
		!v32.ClusterConditionReady.IsTrue(cluster) {
		return nil, nil
	}

	if cluster.Spec.EnableNetworkPolicy == nil {
		return nil, nil
	}

	toEnable := convert.ToBool(cluster.Annotations[netPolAnnotation])

	if cluster.Status.AppliedEnableNetworkPolicy == toEnable {
		return nil, nil
	}

	if toEnable != *cluster.Spec.EnableNetworkPolicy {
		// allow clusterNetAnnHandler to update first
		return nil, nil
	}

	var err error
	if toEnable {
		logrus.Infof("clusterHandler: calling sync to create network policies for cluster %v", cluster.Name)
		err = ch.createNetworkPolicies(cluster)
	} else {
		logrus.Infof("clusterHandler: deleting network policies for cluster %s", cluster.Name)
		err = ch.deleteNetworkPolicies(cluster)
	}

	if err != nil {
		return nil, err
	}

	cluster.Status.AppliedEnableNetworkPolicy = toEnable

	_, err = ch.clusters.Update(cluster)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (ch *clusterHandler) createNetworkPolicies(cluster *v3.Cluster) error {
	projects, err := ch.pLister.List(cluster.Name, labels.NewSelector())
	if err != nil {
		return fmt.Errorf("projectLister: %v", err)
	}

	for _, project := range projects {
		ch.npmgr.projects.Controller().Enqueue(project.Namespace, project.Name)
	}

	systemNamespaces, _, err := ch.npmgr.getSystemNSInfo(cluster.Name)
	if err != nil {
		return fmt.Errorf("systemNS: %v", err)
	}

	//hostPort
	pods, err := ch.podLister.List("", labels.NewSelector())
	if err != nil {
		return fmt.Errorf("podLister: %v", err)
	}

	for _, pod := range pods {
		if systemNamespaces[pod.Namespace] {
			continue
		}
		if hostPortPod(pod) {
			ch.cluster.Core.Pods("").Controller().Enqueue(pod.Namespace, pod.Name)
		}
	}

	// nodePort
	svcs, err := ch.serviceLister.List("", labels.NewSelector())
	if err != nil {
		return err
	}
	for _, svc := range svcs {
		if systemNamespaces[svc.Namespace] {
			continue
		}
		if nodePortService(svc) {
			ch.cluster.Core.Services("").Controller().Enqueue(svc.Namespace, svc.Name)
		}
	}

	ch.cluster.Management.Management.Nodes(ch.cluster.ClusterName).Controller().Enqueue(
		cluster.Name, fmt.Sprintf("%s/%s", ch.cluster.ClusterName, nodesyncer.AllNodeKey))

	return nil
	//skipping nssyncer, projectSyncer + nodehandler would result into handling nssyncer as well
}

// deleteNetworkPolicies removes Rancher created NetworkPolicy resources from the downstream cluster and
// removes ProjectNetworkPolicy resources from the management cluster
func (ch *clusterHandler) deleteNetworkPolicies(cluster *v3.Cluster) error {
	// consider nps for deletion if they were created by Rancher, i.e. they have a label: "cattle.io/creator": "norman"
	set := labels.Set(map[string]string{creatorLabel: creatorNorman})
	nps, err := ch.npmgr.npLister.List("", set.AsSelector())
	if err != nil {
		return fmt.Errorf("npLister: %v", err)
	}
	for _, np := range nps {
		if err := ch.npmgr.delete(np.Namespace, np.Name); err != nil {
			return fmt.Errorf("npDelete: %v", err)
		}
	}

	projects, err := ch.pLister.List(cluster.Name, labels.NewSelector())
	if err != nil {
		return fmt.Errorf("projectLister: %v", err)
	}

	for _, project := range projects {
		pnps, err := ch.pnpLister.List(project.Name, labels.NewSelector())
		if err != nil {
			return fmt.Errorf("pnpLister: %v", err)
		}

		for _, pnp := range pnps {
			err := ch.pnps.DeleteNamespaced(pnp.Namespace, pnp.Name, &metav1.DeleteOptions{})
			if err != nil {
				return fmt.Errorf("pnpDelete: %v", err)
			}
		}
	}
	return nil
}
