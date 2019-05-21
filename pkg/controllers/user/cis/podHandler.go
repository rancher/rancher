package cis

import (
	"fmt"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type podHandler struct {
	clusterClient    v3.ClusterInterface
	clusterLister    v3.ClusterLister
	clusterNamespace string
}

func (ph *podHandler) Sync(key string, pod *corev1.Pod) (runtime.Object, error) {
	if pod == nil || pod.DeletionTimestamp != nil || pod.Name != DefaultSonobuoyPodName {
		return nil, nil
	}
	logrus.Infof("cis: podHandler: Sync: %+v", *pod)

	// Check if the Pod belongs to the CIS helm chart

	// Check the annotation to see if it's done processing
	if _, ok := pod.Annotations[SonobuoyCompletionAnnotation]; !ok {
		return nil, nil
	}

	cluster, err := ph.clusterLister.Get("", ph.clusterNamespace)
	if err != nil {
		return nil, fmt.Errorf("cis: error getting cluster %v", err)
	}

	if cluster.Annotations[SonobuoyCompletionAnnotation] == "true" {
		return nil, nil
	}

	updatedCluster := cluster.DeepCopy()
	updatedCluster.Annotations[SonobuoyCompletionAnnotation] = "true"
	_, err = ph.clusterClient.Update(updatedCluster)
	if err != nil {
		return nil, fmt.Errorf("podHandler: failed to update cluster about CIS scan completion")
	}

	logrus.Infof("CIS scan complete")
	return nil, nil
}
