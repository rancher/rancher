package cis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	app2 "github.com/rancher/rancher/pkg/app"

	v33 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"

	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	cutils "github.com/rancher/rancher/pkg/catalog/utils"
	versionutil "github.com/rancher/rancher/pkg/catalog/utils"
	appsv1 "github.com/rancher/rancher/pkg/generated/norman/apps/v1"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	rcorev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	projv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/security-scan/pkg/kb-summarizer/report"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// helm chart variable names
	varOwner                      = "owner"
	varUserSkipConfigMapName      = "userSkipConfigMapName"
	varDefaultSkipConfigMapName   = "defaultSkipConfigMapName"
	varNotApplicableConfigMapName = "notApplicableConfigMapName"
	varDebugMaster                = "debugMaster"
	varDebugWorker                = "debugWorker"
	varOverrideBenchmarkVersion   = "overrideBenchmarkVersion"
	runnerPodPrefix               = "security-scan-runner-"
	templateName                  = "rancher-cis-benchmark"
)

var (
	SonobuoyMasterLabel = map[string]string{"run": "sonobuoy-master"}
)

type cisScanHandler struct {
	clusterNamespace             string
	clusterClient                v3.ClusterInterface
	clusterLister                v3.ClusterLister
	projectLister                v3.ProjectLister
	nodeLister                   corev1.NodeLister
	appClient                    projv3.AppInterface
	catalogTemplateVersionLister v3.CatalogTemplateVersionLister
	clusterScanClient            v3.ClusterScanInterface
	nsClient                     rcorev1.NamespaceInterface
	cmClient                     rcorev1.ConfigMapInterface
	cmLister                     rcorev1.ConfigMapLister
	systemAccountManager         *systemaccount.Manager
	cisConfigClient              v3.CisConfigInterface
	cisConfigLister              v3.CisConfigLister
	cisBenchmarkVersionClient    v3.CisBenchmarkVersionInterface
	cisBenchmarkVersionLister    v3.CisBenchmarkVersionLister
	podClient                    rcorev1.PodInterface
	podLister                    rcorev1.PodLister
	dsClient                     appsv1.DaemonSetInterface
	dsLister                     appsv1.DaemonSetLister
	templateLister               v3.CatalogTemplateLister
	apiExtensionsClient          clientset.Interface
}

type appInfo struct {
	appName                        string
	clusterName                    string
	userSkipConfigMapName          string
	defaultSkipConfigMapName       string
	notApplicableSkipConfigMapName string
	debugMaster                    string
	debugWorker                    string
	overrideBenchmarkVersion       string
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
	if !v32.ClusterConditionReady.IsTrue(cluster) {
		return cs, fmt.Errorf("cisScanHandler: Create: cluster %v not ready", cs.ClusterName)
	}
	if cluster.Spec.WindowsPreferedCluster {
		v32.ClusterScanConditionFailed.True(cs)
		v32.ClusterScanConditionFailed.Message(cs, "cannot run scan on a windows cluster")
		return cs, nil
	}

	cisv2, err := csh.isCISv2Enabled(cluster)
	if err != nil {
		return cs, fmt.Errorf("cisScanHandler: Create: Error while checking CIS v2 CRD presence %v, will retry", err)
	}
	if cisv2 {
		v32.ClusterScanConditionFailed.True(cs)
		v32.ClusterScanConditionFailed.Message(cs, "cannot run scan on cluster, CIS v2 feature is enabled")
		return cs, nil
	}

	if err := isRunnerPodRemoved(csh.podLister); err != nil {
		return cs, fmt.Errorf("cisScanHandler: Create: %v, will retry", err)
	}

	if !v32.ClusterScanConditionCreated.IsTrue(cs) {
		logrus.Infof("cisScanHandler: Create: deploying helm chart")
		currentK8sVersion := cluster.Spec.RancherKubernetesEngineConfig.Version
		overrideBenchmarkVersion := ""
		if cs.Spec.ScanConfig.CisScanConfig != nil {
			overrideBenchmarkVersion = cs.Spec.ScanConfig.CisScanConfig.OverrideBenchmarkVersion
		}
		bv, bvManaged, err := GetBenchmarkVersionToUse(overrideBenchmarkVersion, currentK8sVersion,
			csh.cisConfigLister, csh.cisConfigClient,
			csh.cisBenchmarkVersionLister, csh.cisBenchmarkVersionClient,
		)
		if err != nil {
			return cs, err
		}
		logrus.Debugf("cisScanHandler: Create: k8sVersion: %v, benchmarkVersion: %v",
			currentK8sVersion, bv)
		skipOverride := false
		appInfo := &appInfo{
			appName:                  cs.Name,
			clusterName:              cs.Spec.ClusterID,
			overrideBenchmarkVersion: bv,
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
		}
		if bvManaged {
			appInfo.notApplicableSkipConfigMapName = getNotApplicableConfigMapName(bv)
			if cs.Spec.ScanConfig.CisScanConfig.Profile == "" ||
				cs.Spec.ScanConfig.CisScanConfig.Profile == v32.CisScanProfileTypePermissive {
				appInfo.defaultSkipConfigMapName = getDefaultSkipConfigMapName(bv)
			}
		}

		var cm *v1.ConfigMap
		if skipOverride {
			// create the cm
			skipDataBytes, err := getOverrideSkipInfoData(cs.Spec.ScanConfig.CisScanConfig.OverrideSkip)
			if err != nil {
				v32.ClusterScanConditionFailed.True(cs)
				v32.ClusterScanConditionFailed.Message(cs, fmt.Sprintf("error getting overrideSkip: %v", err))
				return cs, nil
			}
			cm = getConfigMapObject(getOverrideConfigMapName(cs), string(skipDataBytes))
			if err := createConfigMapWithRetry(csh.cmClient, cm); err != nil {
				return cs, fmt.Errorf("cisScanHandler: Create: %v", err)
			}
		} else {
			// Check if the configmap is populated
			userSkipConfigMapName := getUserSkipConfigMapName()
			cm, err = csh.cmLister.Get(v32.DefaultNamespaceForCis, userSkipConfigMapName)
			if err != nil && !kerrors.IsNotFound(err) {
				return cs, fmt.Errorf("cisScanHandler: Create: error fetching configmap %v: %v", err, userSkipConfigMapName)
			}
		}
		if cm != nil {
			appInfo.userSkipConfigMapName = cm.Name
		}

		// Deploy the system helm chart
		if err := csh.deployApp(appInfo); err != nil {
			return cs, fmt.Errorf("cisScanHandler: Create: error deploying app: %v", err)
		}
		v32.ClusterScanConditionCreated.True(cs)
		v32.ClusterScanConditionRunCompleted.Unknown(cs)
	}
	return cs, nil
}

func (csh *cisScanHandler) Remove(cs *v3.ClusterScan) (runtime.Object, error) {
	logrus.Debugf("cisScanHandler: Remove: %+v", cs)
	// Delete the configmap associated with this scan
	err := csh.cmClient.Delete(cs.Name, &metav1.DeleteOptions{})
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
			err := csh.cmClient.Delete(getOverrideConfigMapName(cs), nil)
			if err != nil && !kerrors.IsNotFound(err) {
				return nil, fmt.Errorf("cisScanHandler: Remove: error deleting configmap: %v", err)
			}
		}
	}

	cluster, err := csh.clusterLister.Get("", csh.clusterNamespace)
	if err != nil {
		return nil, fmt.Errorf("cisScanHandler: Remove: error getting cluster %v", err)
	}

	if cluster.Status.CurrentCisRunName == cs.Name {
		updatedCluster := cluster.DeepCopy()
		updatedCluster.Status.CurrentCisRunName = ""
		if _, err := csh.clusterClient.Update(updatedCluster); err != nil {
			return nil, fmt.Errorf("cisScanHandler: Remove: failed to update cluster about CIS scan completion")
		}
	}

	if err := csh.ensureCleanup(cs); err != nil {
		return nil, err
	}
	return cs, nil
}

func (csh *cisScanHandler) Updated(cs *v3.ClusterScan) (runtime.Object, error) {
	logrus.Debugf("cisScanHandler: Updated: %+v", cs)
	if v32.ClusterScanConditionCreated.IsTrue(cs) &&
		!v32.ClusterScanConditionCompleted.IsTrue(cs) &&
		!v32.ClusterScanConditionRunCompleted.IsUnknown(cs) {
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
				err := csh.cmClient.Delete(getOverrideConfigMapName(cs), nil)
				if err != nil && !kerrors.IsNotFound(err) {
					return nil, fmt.Errorf("cisScanHandler: Updated: error deleting configmap: %v", err)
				}
			}
		}

		if err := isRunnerPodRemoved(csh.podLister); err != nil {
			return cs, fmt.Errorf("cisScanHandler: Updated: %v, will retry", err)
		}

		cluster, err := csh.clusterLister.Get("", csh.clusterNamespace)
		if err != nil {
			return nil, fmt.Errorf("cisScanHandler: Updated: error getting cluster %v", err)
		}

		updatedCluster := cluster.DeepCopy()
		updatedCluster.Status.CurrentCisRunName = ""
		if _, err := csh.clusterClient.Update(updatedCluster); err != nil {
			return nil, fmt.Errorf("cisScanHandler: Updated: failed to update cluster about CIS scan completion")
		}

		if !v32.ClusterScanConditionFailed.IsTrue(cs) {
			cm, err := csh.cmClient.Get(cs.Name, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("cisScanHandler: Updated: error fetching configmap %v: %v", cs.Name, err)
			}
			r, err := report.Get([]byte(cm.Data[v32.DefaultScanOutputFileName]))
			if err != nil {
				return nil, fmt.Errorf("cisScanHandler: Updated: error getting report from configmap %v: %v", cs.Name, err)
			}
			if r == nil {
				return nil, fmt.Errorf("cisScanHandler: Updated: error: got empty report from configmap %v", cs.Name)
			}

			cisScanStatus := &v32.CisScanStatus{
				Total:         r.Total,
				Pass:          r.Pass,
				Fail:          r.Fail,
				Skip:          r.Skip,
				NotApplicable: r.NotApplicable,
			}

			cs.Status.CisScanStatus = cisScanStatus
		}
		v32.ClusterScanConditionCompleted.True(cs)
		v32.ClusterScanConditionAlerted.Unknown(cs)
	} else if v32.ClusterScanConditionFailed.IsTrue(cs) {
		cluster, err := csh.clusterLister.Get("", csh.clusterNamespace)
		if err != nil {
			return nil, fmt.Errorf("cisScanHandler: Updated: error getting cluster %v", err)
		}
		updatedCluster := cluster.DeepCopy()
		updatedCluster.Status.CurrentCisRunName = ""
		if _, err := csh.clusterClient.Update(updatedCluster); err != nil {
			return nil, fmt.Errorf("cisScanHandler: Updated: failed to update cluster about CIS scan completion with error %v", err)
		}
	}
	return cs, nil
}

func (csh *cisScanHandler) deployApp(appInfo *appInfo) error {
	appCatalogID, err := csh.getCISBenchmarkCatalogID(appInfo.clusterName)
	if err != nil {
		return errors.Wrapf(err, "cisScanHandler: deployApp: failed to find cis system catalog %q", appCatalogID)
	}
	err = app2.DetectAppCatalogExistence(appCatalogID, csh.catalogTemplateVersionLister)
	if err != nil {
		return errors.Wrapf(err, "cisScanHandler: deployApp: failed to find cis system catalog %q", appCatalogID)
	}
	appDeployProjectID, err := app2.GetSystemProjectID(appInfo.clusterName, csh.projectLister)
	if err != nil {
		return err
	}

	creator, err := csh.systemAccountManager.GetSystemUser(appInfo.clusterName)
	if err != nil {
		return err
	}
	appProjectName, err := app2.EnsureAppProjectName(csh.nsClient, appDeployProjectID, appInfo.clusterName, v32.DefaultNamespaceForCis, creator.Name)
	if err != nil {
		return err
	}

	appAnswers := map[string]string{
		varOwner:                      appInfo.appName,
		varUserSkipConfigMapName:      appInfo.userSkipConfigMapName,
		varDefaultSkipConfigMapName:   appInfo.defaultSkipConfigMapName,
		varNotApplicableConfigMapName: appInfo.notApplicableSkipConfigMapName,
		varDebugMaster:                appInfo.debugMaster,
		varDebugWorker:                appInfo.debugWorker,
		varOverrideBenchmarkVersion:   appInfo.overrideBenchmarkVersion,
	}

	taints, err := csh.collectTaints()
	if err != nil {
		return err
	}
	appAnswers = labels.Merge(appAnswers, taints)

	logrus.Debugf("cisScanHandler: deployApp: appAnswers: %+v", appAnswers)
	app := &projv3.App{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{creatorIDAnno: creator.Name},
			Name:        appInfo.appName,
			Namespace:   appDeployProjectID,
		},
		Spec: v33.AppSpec{
			Answers:         appAnswers,
			Description:     "Rancher CIS Benchmark",
			ExternalID:      appCatalogID,
			ProjectName:     appProjectName,
			TargetNamespace: v32.DefaultNamespaceForCis,
		},
	}

	_, err = app2.DeployApp(csh.appClient, appDeployProjectID, app, false)
	if err != nil {
		return err
	}

	return nil
}

// collectTaints collect all taints on kubernetes nodes except node.kubernetes.io/*
func (csh *cisScanHandler) collectTaints() (map[string]string, error) {
	r := map[string]string{}
	selector := labels.NewSelector()
	nodes, err := csh.nodeLister.List("", selector)
	if err != nil {
		return nil, err
	}

	index := 0
	for _, node := range nodes {
		for _, taint := range node.Spec.Taints {
			if !strings.HasPrefix(taint.Key, "node.kubernetes.io") {
				r[fmt.Sprintf("sonobuoy.tolerations[%v].key", index)] = taint.Key
				r[fmt.Sprintf("sonobuoy.tolerations[%v].operator", index)] = "Exists"
				r[fmt.Sprintf("sonobuoy.tolerations[%v].effect", index)] = string(taint.Effect)
				index++
			}
		}
	}
	return r, nil
}

func (csh *cisScanHandler) deleteApp(appInfo *appInfo) error {
	appDeployProjectID, err := app2.GetSystemProjectID(appInfo.clusterName, csh.projectLister)
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return err
		}
		return nil
	}

	err = app2.DeleteApp(csh.appClient, appDeployProjectID, appInfo.appName)
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return err
		}
		return nil
	}

	return nil
}

func (csh *cisScanHandler) ensureCleanup(cs *v3.ClusterScan) error {
	var err error

	// Delete the dameonset
	dss, e := csh.dsLister.List(v32.DefaultNamespaceForCis, labels.Everything())
	if e != nil {
		err = multierror.Append(err, fmt.Errorf("cis: ensureCleanup: error listing ds: %v", e))
	} else {
		for _, ds := range dss {
			if e := csh.dsClient.Delete(ds.Name, &metav1.DeleteOptions{}); e != nil && !kerrors.IsNotFound(e) {
				err = multierror.Append(err, fmt.Errorf("cis: ensureCleanup: error deleting ds %v: %v", ds.Name, e))
			}
		}
	}

	// Delete the pod
	podName := runnerPodPrefix + cs.Name
	if e := csh.podClient.Delete(podName, &metav1.DeleteOptions{}); e != nil && !kerrors.IsNotFound(e) {
		err = multierror.Append(err, fmt.Errorf("cis: ensureCleanup: error deleting pod %v: %v", podName, e))
	}

	// Delete cms
	cms, err := csh.cmLister.List(v32.DefaultNamespaceForCis, labels.Everything())
	if err != nil {
		err = multierror.Append(err, fmt.Errorf("cis: ensureCleanup: error listing cm: %v", e))
	} else {
		for _, cm := range cms {
			if !strings.Contains(cm.Name, cs.Name) {
				continue
			}
			if e := csh.cmClient.Delete(cm.Name, &metav1.DeleteOptions{}); e != nil && !kerrors.IsNotFound(e) {
				err = multierror.Append(err, fmt.Errorf("cis: ensureCleanup: error deleting cm %v: %v", cm.Name, e))
			}
		}
	}

	return err
}

func (csh *cisScanHandler) getCISBenchmarkCatalogID(clusterName string) (string, error) {
	templateVersionID := csh.getRancherCISBenchmarkTemplateID()
	return versionutil.GetSystemAppCatalogID(templateVersionID, csh.templateLister, csh.clusterLister, clusterName)
}

func (csh *cisScanHandler) getRancherCISBenchmarkTemplateID() string {
	return fmt.Sprintf("%s-%s", cutils.SystemLibraryName, templateName)
}

func (csh *cisScanHandler) isCISv2Enabled(cluster *v3.Cluster) (bool, error) {
	cisv2CRD := "clusterscanprofiles.cis.cattle.io"
	_, err := csh.apiExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.TODO(), cisv2CRD, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
