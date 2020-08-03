package monitoring

import (
	"fmt"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/pkg/errors"
	appsv1 "github.com/rancher/rancher/pkg/generated/norman/apps/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	ConditionGrafanaDeployed          = condition(v32.MonitoringConditionGrafanaDeployed)
	ConditionPrometheusDeployed       = condition(v32.MonitoringConditionPrometheusDeployed)
	ConditionMetricExpressionDeployed = condition(v32.MonitoringConditionMetricExpressionDeployed)
)

// All component names base on rancher-monitoring chart

func isGrafanaDeployed(agentDeploymentClient appsv1.DeploymentInterface, appNamespace, appNameSuffix string, monitoringStatus *v32.MonitoringStatus, clusterName string) error {
	_, err := ConditionGrafanaDeployed.DoUntilTrue(monitoringStatus, func() (*v32.MonitoringStatus, error) {
		obj, err := agentDeploymentClient.GetNamespaced(appNamespace, "grafana-"+appNameSuffix, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return nil, errors.New("Grafana Deployment isn't deployed")
			}

			return nil, errors.Wrap(err, "failed to get Grafana Deployment information")
		}

		status := obj.Status
		if status.Replicas != status.ReadyReplicas {
			return nil, errors.New("Grafana Deployment is deploying")
		}

		monitoringStatus.GrafanaEndpoint = fmt.Sprintf("/k8s/clusters/%s/api/v1/namespaces/%s/services/http:access-grafana:80/proxy/", clusterName, appNamespace)

		return monitoringStatus, nil
	})

	return err
}

func isGrafanaWithdrew(agentDeploymentClient appsv1.DeploymentInterface, appNamespace, appNameSuffix string, monitoringStatus *v32.MonitoringStatus) error {
	_, err := ConditionGrafanaDeployed.DoUntilFalse(monitoringStatus, func() (*v32.MonitoringStatus, error) {
		_, err := agentDeploymentClient.GetNamespaced(appNamespace, "grafana-"+appNameSuffix, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				monitoringStatus.GrafanaEndpoint = ""
				return monitoringStatus, nil
			}

			return nil, errors.Wrap(err, "failed to get Grafana Deployment information")
		}

		return nil, errors.New("Grafana Deployment is withdrawing")
	})

	return err
}

func isPrometheusDeployed(agentStatefulSetClient appsv1.StatefulSetInterface, appNamespace, appNameSuffix string, monitoringStatus *v32.MonitoringStatus) error {
	_, err := ConditionPrometheusDeployed.DoUntilTrue(monitoringStatus, func() (*v32.MonitoringStatus, error) {
		obj, err := agentStatefulSetClient.GetNamespaced(appNamespace, "prometheus-"+appNameSuffix, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return nil, errors.New("Prometheus StatefulSet isn't deployed")
			}

			return nil, errors.Wrap(err, "failed to get Prometheus StatefulSet information")
		}

		if obj.Status.Replicas != obj.Status.ReadyReplicas {
			return nil, errors.New("Prometheus StatefulSet is deploying")
		}

		return monitoringStatus, nil
	})

	return err
}

func isPrometheusWithdrew(agentStatefulSetClient appsv1.StatefulSetInterface, appNamespace, appNameSuffix string, monitoringStatus *v32.MonitoringStatus) error {
	_, err := ConditionPrometheusDeployed.DoUntilFalse(monitoringStatus, func() (*v32.MonitoringStatus, error) {
		_, err := agentStatefulSetClient.GetNamespaced(appNamespace, "prometheus-"+appNameSuffix, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return monitoringStatus, nil
			}

			return nil, errors.Wrap(err, "failed to get Prometheus StatefulSet information")
		}

		return nil, errors.New("Prometheus StatefulSet is withdrawing")
	})

	return err
}
