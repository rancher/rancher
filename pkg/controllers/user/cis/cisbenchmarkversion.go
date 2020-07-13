package cis

import (
	"encoding/json"
	"fmt"

	rcorev1 "github.com/rancher/rancher/pkg/types/apis/core/v1"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type cisBenchmarkVersionHandler struct {
	clusterName               string
	projectLister             v3.ProjectLister
	cisBenchmarkVersionLister v3.CisBenchmarkVersionLister
	configMapsClient          rcorev1.ConfigMapInterface
	nsClient                  rcorev1.NamespaceInterface
}

func (h *cisBenchmarkVersionHandler) Sync(_ string, benchmarkVersion *v3.CisBenchmarkVersion) (runtime.Object, error) {
	if benchmarkVersion == nil ||
		benchmarkVersion.DeletionTimestamp != nil ||
		!benchmarkVersion.Info.Managed {
		return nil, nil
	}

	if err := createSecurityScanNamespace(h.nsClient, h.projectLister, h.clusterName); err != nil {
		return nil, fmt.Errorf("cisBenchmarkVersionHandler: Sync: error creating namespace: %v", err)
	}
	naConfigMapName := getNotApplicableConfigMapName(benchmarkVersion.Name)
	naBytes, err := json.Marshal(benchmarkVersion.Info.NotApplicableChecks)
	if err != nil {
		return nil, fmt.Errorf("cisBenchmarkVersionHandler: Sync: error marshalling na checks: %v", err)
	}
	if err := h.syncConfigMap(naConfigMapName, string(naBytes)); err != nil {
		return nil, fmt.Errorf("cisBenchmarkVersionHandler: Sync: %v", err)
	}

	skipConfigMapName := getDefaultSkipConfigMapName(benchmarkVersion.Name)
	skipBytes, err := json.Marshal(benchmarkVersion.Info.SkippedChecks)
	if err != nil {
		return nil, fmt.Errorf("cisBenchmarkVersionHandler: Sync: error marshalling skip checks: %v", err)
	}
	if err := h.syncConfigMap(skipConfigMapName, string(skipBytes)); err != nil {
		return nil, fmt.Errorf("cisBenchmarkVersionHandler: Sync: %v", err)
	}

	return nil, nil
}

func (h *cisBenchmarkVersionHandler) syncConfigMap(cmName, data string) error {
	cm, err := h.configMapsClient.Get(cmName, v1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("while syncing configmap %v, got error during get: %v", cmName, err)
		}
		cm = getConfigMapObject(cmName, data)
		_, err = h.configMapsClient.Create(cm)
		if err != nil {
			return fmt.Errorf("error creating configmap %v: %v", cmName, err)
		}
		return nil
	}
	if cm.Data[ConfigFileName] == data {
		return nil
	}
	cm.Data[ConfigFileName] = data
	_, err = h.configMapsClient.Update(cm)
	if err != nil {
		return fmt.Errorf("error updating configmap %v: %v", cmName, err)
	}
	return nil
}
