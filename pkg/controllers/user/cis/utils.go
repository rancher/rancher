package cis

import (
	"fmt"
	"time"

	"github.com/rancher/rancher/pkg/app/utils"
	"github.com/rancher/rancher/pkg/controllers/user/nslabels"
	rcorev1 "github.com/rancher/types/apis/core/v1"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func createConfigMapWithRetry(configMapsClient rcorev1.ConfigMapInterface, cm *v1.ConfigMap) error {
	var err error
	success := false
	for i := 0; i < NumberOfRetriesForConfigMapCreate; i++ {
		_, err = configMapsClient.Create(cm)
		if err == nil || kerrors.IsAlreadyExists(err) {
			success = true
			break
		}
		time.Sleep(RetryIntervalInMilliseconds * time.Millisecond)
	}
	if !success {
		return fmt.Errorf("error creating configmap %v: %v", cm.Name, err)
	}
	return nil
}

func isRunnerPodRemoved(podLister rcorev1.PodLister) error {
	pods, err := podLister.List(
		v3.DefaultNamespaceForCis,
		labels.Set(SonobuoyMasterLabel).AsSelector(),
	)
	if err != nil {
		return fmt.Errorf("error listing pods: %v", err)
	}
	if len(pods) != 0 {
		return fmt.Errorf("runner pod not yet deleted")
	}
	return nil
}

func getConfigMapObject(cmName, data string) *v1.ConfigMap {
	return &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind: "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: cmName,
		},
		Data: map[string]string{
			ConfigFileName: data,
		},
	}
}

func getNotApplicableConfigMapName(benchmarkVersion string) string {
	return fmt.Sprintf("na-%v", benchmarkVersion)
}

func getDefaultSkipConfigMapName(benchmarkVersion string) string {
	return fmt.Sprintf("ds-%v", benchmarkVersion)
}

func getUserSkipConfigMapName() string {
	return v3.ConfigMapNameForUserConfig
}

func createSecurityScanNamespace(nsClient rcorev1.NamespaceInterface, projectLister v3.ProjectLister, clusterName string) error {
	systemProjectID, err := utils.GetSystemProjectID(clusterName, projectLister)
	if err != nil {
		return err
	}

	nsName := v3.DefaultNamespaceForCis
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
			Annotations: map[string]string{
				nslabels.ProjectIDFieldLabel: fmt.Sprintf("%s:%s", clusterName, systemProjectID),
			},
		},
	}
	if ns, err = nsClient.Create(ns); err != nil && !kerrors.IsAlreadyExists(err) {
		return fmt.Errorf("error while creating namespace %v: %v", nsName, err)
	}
	return nil
}

func ValidateClusterBeforeLaunchingScan(cluster *v3.Cluster) error {
	if cluster.Spec.WindowsPreferedCluster {
		return fmt.Errorf("cannot run scan on a windows cluster")
	}
	if cluster.DeletionTimestamp != nil {
		return fmt.Errorf("cluster with id %v is being deleted", cluster.Name)
	}
	if !v3.ClusterConditionReady.IsTrue(cluster) {
		return fmt.Errorf("cluster not ready")
	}
	if cluster.Status.CurrentCisRunName != "" {
		return fmt.Errorf("CIS scan already running on cluster")
	}
	return nil
}
