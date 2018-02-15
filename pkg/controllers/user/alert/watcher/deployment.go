package watcher

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/rancher/pkg/controllers/user/alert/manager"
	"github.com/rancher/rancher/pkg/ticker"
	"github.com/rancher/types/apis/apps/v1beta2"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	appsv1beta2 "k8s.io/api/apps/v1beta2"
)

type DeploymentWatcher struct {
	deploymentLister   v1beta2.DeploymentLister
	projectAlertLister v3.ProjectAlertLister
	alertManager       *manager.Manager
	clusterName        string
}

func StartDeploymentWatcher(ctx context.Context, cluster *config.UserContext, manager *manager.Manager) {

	d := &DeploymentWatcher{
		deploymentLister:   cluster.Apps.Deployments("").Controller().Lister(),
		projectAlertLister: cluster.Management.Management.ProjectAlerts("").Controller().Lister(),
		alertManager:       manager,
		clusterName:        cluster.ClusterName,
	}

	go d.watch(ctx, syncInterval)
}

func (w *DeploymentWatcher) watch(ctx context.Context, interval time.Duration) {
	for range ticker.Context(ctx, interval) {
		err := w.watchRule()
		if err != nil {
			logrus.Infof("Failed to watch deployment", err)
		}
	}
}

func (w *DeploymentWatcher) watchRule() error {
	if w.alertManager.IsDeploy == false {
		return nil
	}

	projectAlerts, err := w.projectAlertLister.List("", labels.NewSelector())
	if err != nil {
		return err
	}

	pAlerts := []*v3.ProjectAlert{}
	for _, alert := range projectAlerts {
		if controller.ObjectInCluster(w.clusterName, alert) {
			pAlerts = append(pAlerts, alert)
		}
	}

	for _, alert := range pAlerts {
		if alert.Status.State == "inactive" {
			continue
		}
		if alert.Spec.TargetWorkload.Type == "deployment" {

			if alert.Spec.TargetWorkload.WorkloadID != "" {
				parts := strings.Split(alert.Spec.TargetWorkload.WorkloadID, ":")
				ns := parts[0]
				id := parts[1]
				dep, err := w.deploymentLister.Get(ns, id)
				if err != nil {
					logrus.Debugf("Failed to get deployment %s: %v", id, err)
					continue
				}
				w.checkUnavailble(dep, alert)

			} else if alert.Spec.TargetWorkload.Selector != nil {
				//TODO: should check if the deployment in the same project as the alert

				selector := labels.NewSelector()
				for key, value := range alert.Spec.TargetWorkload.Selector {
					r, _ := labels.NewRequirement(key, selection.Equals, []string{value})
					selector.Add(*r)
				}
				deps, err := w.deploymentLister.List("", selector)
				if err != nil {
					continue
				}
				for _, dep := range deps {
					w.checkUnavailble(dep, alert)
				}
			}
		}
	}

	return nil
}

func (w *DeploymentWatcher) checkUnavailble(deployment *appsv1beta2.Deployment, alert *v3.ProjectAlert) {
	alertID := alert.Namespace + "-" + alert.Name
	percentage := alert.Spec.TargetWorkload.AvailablePercentage

	if percentage == 0 {
		return
	}

	availableThreshold := int32(percentage) * (*deployment.Spec.Replicas) / 100

	if deployment.Status.AvailableReplicas <= availableThreshold {
		title := fmt.Sprintf("The deployment %s has available replicas less than  %s%%", deployment.Name, strconv.Itoa(percentage))
		desc := fmt.Sprintf("*Alert Name*: %s\n*Cluster Name*: %s\n*Available Replicas*: %s\n*Desired Replicas*: %s", alert.Spec.DisplayName, w.clusterName, strconv.Itoa(int(deployment.Status.AvailableReplicas)),
			strconv.Itoa(int(*deployment.Spec.Replicas)))

		if err := w.alertManager.SendAlert(alertID, desc, title, alert.Spec.Severity); err != nil {
			logrus.Debugf("Failed to send alert: %v", err)
		}
	}

}
