package cis

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/app/utils"
	"github.com/rancher/rancher/pkg/controllers/management/kontainerdrivermetadata"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rke/util"
	rcorev1 "github.com/rancher/types/apis/core/v1"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	projv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	NumberOfRetriesForConfigMapCreate = 3
	RetryIntervalInMilliseconds       = 100
	ConfigFileName                    = "config.json"
	CurrentBenchmarkKey               = "current"
)

type cisScanHandler struct {
	clusterLister                v3.ClusterLister
	projectLister                v3.ProjectLister
	mgmtCtxClusterClient         v3.ClusterInterface
	mgmtCtxAppClient             projv3.AppInterface
	mgmtCtxTemplateVersionLister v3.CatalogTemplateVersionLister
	mgmtCtxClusterScanClient     v3.ClusterScanInterface
	userCtxNSClient              rcorev1.NamespaceInterface
	userCtxCMLister              rcorev1.ConfigMapLister
	clusterNamespace             string
	systemAccountManager         *systemaccount.Manager
	configMapsClient             rcorev1.ConfigMapInterface
	cisConfig                    v3.CisConfigInterface
	cisConfigLister              v3.CisConfigLister
}

type appInfo struct {
	appName                  string
	clusterName              string
	skipConfigMapName        string
	debugMaster              string
	debugWorker              string
	overrideBenchmarkVersion string
}

type OverrideSkipInfoData struct {
	Skip map[string][]string `json:"skip"`
}

func getOverrideConfigMapName(cs *v3.ClusterScan) string {
	return fmt.Sprintf("%v-cfg", cs.Name)
}

func getOverrideSkipInfoData(skip []string) ([]byte, error) {
	s := OverrideSkipInfoData{Skip: map[string][]string{CurrentBenchmarkKey: skip}}
	return json.Marshal(s)
}

func (csh *cisScanHandler) Create(cs *v3.ClusterScan) (runtime.Object, error) {
	logrus.Debugf("cisScanHandler: Create: %+v", spew.Sdump(cs))
	var err error
	cluster, err := csh.clusterLister.Get("", cs.Spec.ClusterID)
	if err != nil {
		return cs, fmt.Errorf("cisScanHandler: Create: error listing cluster %v: %v", cs.ClusterName, err)
	}
	if !v3.ClusterConditionReady.IsTrue(cluster) {
		return cs, fmt.Errorf("cisScanHandler: Create: cluster %v not ready", cs.ClusterName)
	}
	if cluster.Spec.WindowsPreferedCluster {
		v3.ClusterScanConditionFailed.True(cs)
		v3.ClusterScanConditionFailed.Message(cs, "cannot run scan on a windows cluster")
		return cs, nil
	}
	if !v3.ClusterScanConditionCreated.IsTrue(cs) {
		logrus.Infof("cisScanHandler: Create: deploying helm chart")
		currentK8sVersion := cluster.Spec.RancherKubernetesEngineConfig.Version
		shortK8sVersion := util.GetTagMajorVersion(currentK8sVersion)
		cisConfigParams, err := kontainerdrivermetadata.GetCisConfigParams(
			shortK8sVersion,
			csh.cisConfigLister,
			csh.cisConfig,
		)
		if err != nil {
			logrus.Debugf("cisScanHandler: Create: benchmark version not found for k8s version: %v(%v), using default",
				currentK8sVersion, shortK8sVersion)
			cisConfigParams, err = kontainerdrivermetadata.GetCisConfigParams(
				"default",
				csh.cisConfigLister,
				csh.cisConfig,
			)
			if err != nil {
				return cs, fmt.Errorf("error fetching default cis config: %v", err)
			}
		}
		logrus.Debugf("cisScanHandler: Create: k8sVersion: %v, benchmarkVersion: %v",
			currentK8sVersion, cisConfigParams.BenchmarkVersion)
		skipOverride := false
		appInfo := &appInfo{
			appName:                  cs.Name,
			clusterName:              cs.Spec.ClusterID,
			overrideBenchmarkVersion: cisConfigParams.BenchmarkVersion,
		}
		if cs.Spec.ScanConfig.CisScanConfig != nil {
			if cs.Spec.ScanConfig.CisScanConfig.DebugMaster {
				appInfo.debugMaster = "true"
			}
			if cs.Spec.ScanConfig.CisScanConfig.DebugWorker {
				appInfo.debugWorker = "true"
			}
			if cs.Spec.ScanConfig.CisScanConfig.OverrideSkip != nil {
				skipOverride = true
			}
			if cs.Spec.ScanConfig.CisScanConfig.OverrideBenchmarkVersion != "" {
				logrus.Debugf("cisScanHandler: Create: user requested overrideBenchmarkVersion: %v",
					cs.Spec.ScanConfig.CisScanConfig.OverrideBenchmarkVersion)
				appInfo.overrideBenchmarkVersion = cs.Spec.ScanConfig.CisScanConfig.OverrideBenchmarkVersion
			}
		}

		var cm *v1.ConfigMap
		if skipOverride {
			// create the cm
			skipDataBytes, err := getOverrideSkipInfoData(cs.Spec.ScanConfig.CisScanConfig.OverrideSkip)
			if err != nil {
				logrus.Errorf("cisScanHandler: Create: error getting overrideSkip: %v", err)
			} else {
				cm = &v1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						Kind: "ConfigMap",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: getOverrideConfigMapName(cs),
					},
					Data: map[string]string{
						ConfigFileName: string(skipDataBytes),
					},
				}
				success := false
				for i := 0; i < NumberOfRetriesForConfigMapCreate; i++ {
					cm, err = csh.configMapsClient.Create(cm)
					if err == nil || kerrors.IsAlreadyExists(err) {
						logrus.Infof("cisScanHandler: Create: created skip override configmap %v with contents: %v", cm.Name, string(skipDataBytes))
						success = true
						break
					}
					time.Sleep(RetryIntervalInMilliseconds * time.Millisecond)
				}
				if !success {
					cm = nil
					logrus.Errorf("cisScanHandler: Create: error creating configmap: %v", err)
				}
			}
		} else {
			// Check if the configmap is populated
			cm, err = csh.userCtxCMLister.Get(v3.DefaultNamespaceForCis, v3.ConfigMapNameForUserConfig)
			if err != nil && !kerrors.IsNotFound(err) {
				return cs, fmt.Errorf("cisScanHandler: Create: error fetching configmap %v: %v", err, v3.ConfigMapNameForUserConfig)
			}
		}
		if cm != nil {
			appInfo.skipConfigMapName = cm.Name
		}
		// Deploy the system helm chart
		if err := csh.deployApp(appInfo); err != nil {
			return cs, fmt.Errorf("cisScanHandler: Create: error deploying app: %v", err)
		}
		v3.ClusterScanConditionCreated.True(cs)
		v3.ClusterScanConditionRunCompleted.Unknown(cs)
	}
	return cs, nil
}

func (csh *cisScanHandler) Remove(cs *v3.ClusterScan) (runtime.Object, error) {
	logrus.Debugf("cisScanHandler: Remove: %+v", cs)
	// Delete the configmap associated with this scan
	err := csh.configMapsClient.Delete(cs.Name, &metav1.DeleteOptions{})
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return cs, fmt.Errorf("cisScanHandler: Remove: error deleting cm=%v", cs.Name)
		}
	}

	appInfo := &appInfo{
		appName:     cs.Name,
		clusterName: cs.Spec.ClusterID,
	}
	if err := csh.deleteApp(appInfo); err != nil {
		if !kerrors.IsNotFound(err) {
			return nil, fmt.Errorf("cisScanHandler: Remove: error deleting app: %v", err)
		}
	}

	if cs.Spec.ScanConfig.CisScanConfig != nil {
		if cs.Spec.ScanConfig.CisScanConfig.OverrideSkip != nil {
			// Delete the configmap
			err := csh.configMapsClient.Delete(getOverrideConfigMapName(cs), nil)
			if err != nil && !kerrors.IsNotFound(err) {
				return nil, fmt.Errorf("cisScanHandler: Remove: error deleting configmap: %v", err)
			}
		}
	}

	cluster, err := csh.clusterLister.Get("", csh.clusterNamespace)
	if err != nil {
		return nil, fmt.Errorf("cisScanHandler: Remove: error getting cluster %v", err)
	}

	if owner, ok := cluster.Annotations[v3.RunCisScanAnnotation]; ok && owner == cs.Name {
		updatedCluster := cluster.DeepCopy()
		delete(updatedCluster.Annotations, v3.RunCisScanAnnotation)
		if _, err := csh.mgmtCtxClusterClient.Update(updatedCluster); err != nil {
			return nil, fmt.Errorf("cisScanHandler: Remove: failed to update cluster about CIS scan completion")
		}
	}

	return cs, nil
}

func (csh *cisScanHandler) Updated(cs *v3.ClusterScan) (runtime.Object, error) {
	logrus.Debugf("cisScanHandler: Updated: %+v", cs)
	if v3.ClusterScanConditionCreated.IsTrue(cs) &&
		!v3.ClusterScanConditionCompleted.IsTrue(cs) &&
		!v3.ClusterScanConditionRunCompleted.IsUnknown(cs) {
		// Delete the system helm chart
		appInfo := &appInfo{
			appName:     cs.Name,
			clusterName: cs.Spec.ClusterID,
		}
		if err := csh.deleteApp(appInfo); err != nil {
			return nil, fmt.Errorf("cisScanHandler: Updated: error deleting app: %v", err)
		}

		if cs.Spec.ScanConfig.CisScanConfig != nil {
			if cs.Spec.ScanConfig.CisScanConfig.OverrideSkip != nil {
				// Delete the configmap
				err := csh.configMapsClient.Delete(getOverrideConfigMapName(cs), nil)
				if err != nil && !kerrors.IsNotFound(err) {
					return nil, fmt.Errorf("cisScanHandler: Updated: error deleting configmap: %v", err)
				}
			}
		}

		cluster, err := csh.clusterLister.Get("", csh.clusterNamespace)
		if err != nil {
			return nil, fmt.Errorf("cisScanHandler: Updated: error getting cluster %v", err)
		}

		updatedCluster := cluster.DeepCopy()
		delete(updatedCluster.Annotations, v3.RunCisScanAnnotation)
		if _, err := csh.mgmtCtxClusterClient.Update(updatedCluster); err != nil {
			return nil, fmt.Errorf("cisScanHandler: Updated: failed to update cluster about CIS scan completion")
		}

		v3.ClusterScanConditionCompleted.True(cs)
	}
	return cs, nil
}

func (csh *cisScanHandler) deployApp(appInfo *appInfo) error {
	appCatalogID := settings.SystemCISBenchmarkCatalogID.Get()
	err := utils.DetectAppCatalogExistence(appCatalogID, csh.mgmtCtxTemplateVersionLister)
	if err != nil {
		return errors.Wrapf(err, "cisScanHandler: deployApp: failed to find cis system catalog %q", appCatalogID)
	}
	appDeployProjectID, err := utils.GetSystemProjectID(appInfo.clusterName, csh.projectLister)
	if err != nil {
		return err
	}

	creator, err := csh.systemAccountManager.GetSystemUser(appInfo.clusterName)
	if err != nil {
		return err
	}
	appProjectName, err := utils.EnsureAppProjectName(csh.userCtxNSClient, appDeployProjectID, appInfo.clusterName, v3.DefaultNamespaceForCis, creator.Name)
	if err != nil {
		return err
	}

	appAnswers := map[string]string{
		"owner":                    appInfo.appName,
		"skipConfigMapName":        appInfo.skipConfigMapName,
		"debugMaster":              appInfo.debugMaster,
		"debugWorker":              appInfo.debugWorker,
		"overrideBenchmarkVersion": appInfo.overrideBenchmarkVersion,
	}
	logrus.Debugf("cisScanHandler: deployApp: appAnswers: %v", appAnswers)
	app := &projv3.App{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{creatorIDAnno: creator.Name},
			Name:        appInfo.appName,
			Namespace:   appDeployProjectID,
		},
		Spec: projv3.AppSpec{
			Answers:         appAnswers,
			Description:     "Rancher CIS Benchmark",
			ExternalID:      appCatalogID,
			ProjectName:     appProjectName,
			TargetNamespace: v3.DefaultNamespaceForCis,
		},
	}

	_, err = utils.DeployApp(csh.mgmtCtxAppClient, appDeployProjectID, app, false)
	if err != nil {
		return err
	}

	return nil
}

func (csh *cisScanHandler) deleteApp(appInfo *appInfo) error {
	appDeployProjectID, err := utils.GetSystemProjectID(appInfo.clusterName, csh.projectLister)
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return err
		}
		return nil
	}

	err = utils.DeleteApp(csh.mgmtCtxAppClient, appDeployProjectID, appInfo.appName)
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return err
		}
		return nil
	}

	return nil
}
