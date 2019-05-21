package cis

import (
	"fmt"
	"github.com/pkg/errors"

	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/utils"
	rcorev1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	projv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type clusterHandler struct {
	mgmtCtxClusterClient         v3.ClusterInterface
	mgmtCtxProjClient            mgmtv3.ProjectInterface
	mgmtCtxAppClient             projv3.AppInterface
	mgmtCtxTemplateVersionClient mgmtv3.CatalogTemplateVersionInterface
	userCtx                      *config.UserContext
	userCtxNSClient              rcorev1.NamespaceInterface
	clusterNamespace             string
	systemAccountManager         *systemaccount.Manager
}

func (ch *clusterHandler) Sync(key string, cluster *v3.Cluster) (runtime.Object, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil ||
		//cluster.Name != ch.clusterNamespace ||
		//!v3.ClusterConditionReady.IsTrue(cluster) {
		cluster.Name != ch.clusterNamespace {
		return nil, nil
	}

	runCisScan := cluster.Annotations[RunCISScanAnnotation]
	if runCisScan == "" {
		return nil, nil
	}
	logrus.Infof("CIS: clusterHandler Sync")

	// TODO: Check if it's ok to be part of a field or need a spec change on cluster object
	// Set a new or update the annotation to note that helm chart has been deployed

	cisSystemChartDeployed := cluster.Annotations[CisHelmChartDeployedAnnotation]
	sonobuoyDone := cluster.Annotations[SonobuoyCompletionAnnotation]

	if runCisScan == "true" && cisSystemChartDeployed == "" {
		// Deploy the system helm chart
		if err := ch.deployApp(cluster.Name); err != nil {
			logrus.Errorf("CIS: error deploying app: %v", err)
			return nil, err
		}
		logrus.Infof("CIS System Helm Chart deployed")

		updatedCluster := cluster.DeepCopy()
		updatedCluster.Annotations[CisHelmChartDeployedAnnotation] = "true"
		if _, err := ch.mgmtCtxClusterClient.Update(updatedCluster); err != nil {
			return nil, fmt.Errorf("failed to launch CIS System Helm Chart")
		}
		return nil, nil
	}

	if runCisScan == "true" && cisSystemChartDeployed == "true" && sonobuoyDone == "true" {
		// Delete the system helm chart
		if err := ch.deleteApp(); err != nil {
			logrus.Errorf("CIS: error deleting app: %v", err)
			return nil, err
		}
		logrus.Infof("CIS System Helm Chart deleted")

		//TODO: check if it's present in first place

		updatedCluster := cluster.DeepCopy()
		delete(updatedCluster.Annotations, SonobuoyCompletionAnnotation)
		delete(updatedCluster.Annotations, CisHelmChartDeployedAnnotation)
		delete(updatedCluster.Annotations, RunCISScanAnnotation)
		if _, err := ch.mgmtCtxClusterClient.Update(updatedCluster); err != nil {
			return nil, fmt.Errorf("clusterHandler: failed to update cluster about CIS scan completion")
		}
	}

	return nil, nil
}

func (ch *clusterHandler) deployApp(clusterName string) error {
	appCatalogID := settings.SystemCISBenchmarkCatalogID.Get()
	err := utils.DetectAppCatalogExistence(appCatalogID, ch.mgmtCtxTemplateVersionClient)
	if err != nil {
		return errors.Wrapf(err, "failed to find cis system catalog %q", appCatalogID)
	}
	appDeployProjectID, err := utils.GetSystemProjectID(ch.mgmtCtxProjClient)
	if err != nil {
		return err
	}

	appProjectName, err := utils.EnsureAppProjectName(ch.userCtxNSClient, appDeployProjectID, clusterName, DefaultNamespaceForCis)
	if err != nil {
		return err
	}

	creator, err := ch.systemAccountManager.GetSystemUser(clusterName)
	if err != nil {
		return err
	}

	app := &projv3.App{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{creatorIDAnno: creator.Name},
			Name:        defaultAppName,
			Namespace:   appDeployProjectID,
		},
		Spec: projv3.AppSpec{
			Description:     "Rancher CIS Benchmark",
			ExternalID:      appCatalogID,
			ProjectName:     appProjectName,
			TargetNamespace: DefaultNamespaceForCis,
		},
	}

	_, err = utils.DeployApp(ch.mgmtCtxAppClient, appDeployProjectID, app, false)
	if err != nil {
		return err
	}

	return nil
}

func (ch *clusterHandler) deleteApp() error {
	appDeployProjectID, err := utils.GetSystemProjectID(ch.mgmtCtxProjClient)
	if err != nil {
		return err
	}

	if err := utils.DeleteApp(ch.mgmtCtxAppClient, appDeployProjectID, defaultAppName); err != nil {
		return err
	}

	return nil
}
