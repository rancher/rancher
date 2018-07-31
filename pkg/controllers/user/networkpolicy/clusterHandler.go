package networkpolicy

import (
	"fmt"

	"time"

	"github.com/rancher/rancher/pkg/controllers/user/nodesyncer"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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

func (ch *clusterHandler) Sync(key string, cluster *v3.Cluster) error {
	if cluster == nil || cluster.DeletionTimestamp != nil ||
		cluster.Name != ch.clusterNamespace ||
		!v3.ClusterConditionReady.IsTrue(cluster) ||
		cluster.Spec.EnableNetworkPolicy == nil ||
		cluster.Status.AppliedEnableNetworkPolicy == *cluster.Spec.EnableNetworkPolicy {
		return nil
	}

	desired := *cluster.Spec.EnableNetworkPolicy
	cluster.Status.AppliedEnableNetworkPolicy = desired

	cluster, err := ch.clusters.Update(cluster)
	if err != nil {
		return err
	}

	err = ch.refresh(cluster)
	var updateErr error
	if err != nil {
		// reset if failure
		cluster.Status.AppliedEnableNetworkPolicy = !desired
		for i := 0; i < 3; i++ {
			_, updateErr = ch.clusters.Update(cluster)
			if updateErr == nil || apierrors.IsNotFound(updateErr) {
				break
			}
			time.Sleep(time.Second * 10)
		}
	}
	if err != nil || updateErr != nil {
		return fmt.Errorf("clusterHandler: %v %v", err, updateErr)
	}
	return nil
}

func (ch *clusterHandler) refresh(cluster *v3.Cluster) error {
	if cluster.Status.AppliedEnableNetworkPolicy {
		logrus.Infof("clusterHandler: calling sync to create network policies for cluster %v", cluster.Name)

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
			cluster.ClusterName, fmt.Sprintf("%s/%s", ch.cluster.ClusterName, nodesyncer.AllNodeKey))

		//skip nssyncer, projectSyncer + nodehandler would result into handling nssyncer as well

	} else {
		logrus.Infof("clusterHandler: deleting network policies for cluster %s", cluster.Name)

		nps, err := ch.npmgr.npLister.List("", labels.NewSelector())
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
	}
	return nil
}
