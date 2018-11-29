package monitoring

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/pkg/errors"
	appsv1beta2 "github.com/rancher/types/apis/apps/v1beta2"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	ConditionGrafanaDeployed           = condition(mgmtv3.MonitoringConditionGrafanaDeployed)
	ConditionNodeExporterDeployed      = condition(mgmtv3.MonitoringConditionNodeExporterDeployed)
	ConditionKubeStateExporterDeployed = condition(mgmtv3.MonitoringConditionKubeStateExporterDeployed)
	ConditionPrometheusDeployed        = condition(mgmtv3.MonitoringConditionPrometheusDeployed)
	ConditionMetricExpressionDeployed  = condition(mgmtv3.MonitoringConditionMetricExpressionDeployed)
)

// All component names base on rancher-monitoring chart

func isGrafanaDeployed(workloadsClient appsv1beta2.Interface, appNamespace, appNameSuffix string, monitoringStatus *mgmtv3.MonitoringStatus, clusterName string) error {
	_, err := ConditionGrafanaDeployed.DoUntilTrue(monitoringStatus, func() (*mgmtv3.MonitoringStatus, error) {
		obj, err := workloadsClient.Deployments(appNamespace).Get("grafana-"+appNameSuffix, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return nil, errors.New("Grafana Deployment isn't deployed")
			}

			return nil, errors.Wrap(err, "failed to get Grafana Deployment information")
		}

		status := obj.Status
		if status.Replicas != (status.AvailableReplicas - status.UnavailableReplicas) {
			return nil, errors.New("Grafana Deployment is deploying")
		}

		monitoringStatus.GrafanaEndpoint = fmt.Sprintf("/k8s/clusters/%s/api/v1/namespaces/%s/services/http:access-grafana:80/proxy/", clusterName, appNamespace)

		return monitoringStatus, nil
	})

	return err
}

func isGrafanaWithdrew(workloadsClient appsv1beta2.Interface, appNamespace, appNameSuffix string, monitoringStatus *mgmtv3.MonitoringStatus) error {
	_, err := ConditionGrafanaDeployed.DoUntilFalse(monitoringStatus, func() (*mgmtv3.MonitoringStatus, error) {
		_, err := workloadsClient.Deployments(appNamespace).Get("grafana-"+appNameSuffix, metav1.GetOptions{})
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

func isNodeExporterDeployed(workloadsClient appsv1beta2.Interface, appNamespace, appNameSuffix string, monitoringStatus *mgmtv3.MonitoringStatus) error {
	_, err := ConditionNodeExporterDeployed.DoUntilTrue(monitoringStatus, func() (*mgmtv3.MonitoringStatus, error) {
		obj, err := workloadsClient.DaemonSets(appNamespace).Get("exporter-node-"+appNameSuffix, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return nil, errors.New("Node Exporter DaemonSet isn't deployed")
			}

			return nil, errors.Wrap(err, "failed to get Node Exporter DaemonSet information")
		}

		if obj.Status.DesiredNumberScheduled != obj.Status.CurrentNumberScheduled {
			return nil, errors.New("Node Exporter DaemonSet is deploying")
		}

		return monitoringStatus, nil
	})

	return err
}

func isNodeExporterWithdrew(workloadsClient appsv1beta2.Interface, appNamespace, appNameSuffix string, monitoringStatus *mgmtv3.MonitoringStatus) error {
	_, err := ConditionNodeExporterDeployed.DoUntilFalse(monitoringStatus, func() (*mgmtv3.MonitoringStatus, error) {
		_, err := workloadsClient.DaemonSets(appNamespace).Get("exporter-node-"+appNameSuffix, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return monitoringStatus, nil
			}

			return nil, errors.Wrap(err, "failed to get Node Exporter DaemonSet information")
		}

		return nil, errors.New("Node Exporter DaemonSet is withdrawing")
	})

	return err
}

func isKubeStateExporterDeployed(workloadsClient appsv1beta2.Interface, appNamespace, appNameSuffix string, monitoringStatus *mgmtv3.MonitoringStatus) error {
	_, err := ConditionKubeStateExporterDeployed.DoUntilTrue(monitoringStatus, func() (*mgmtv3.MonitoringStatus, error) {
		obj, err := workloadsClient.Deployments(appNamespace).Get("exporter-kube-state-"+appNameSuffix, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return nil, errors.New("Kube State Exporter Deployment isn't deployed")
			}

			return nil, errors.Wrap(err, "failed to get Kube State Exporter Deployment information")
		}

		status := obj.Status
		if status.Replicas != (status.AvailableReplicas - status.UnavailableReplicas) {
			return nil, errors.New("Kube State Exporter Deployment is deploying")
		}

		return monitoringStatus, nil
	})

	return err
}

func isKubeStateExporterWithdrew(workloadsClient appsv1beta2.Interface, appNamespace, appNameSuffix string, monitoringStatus *mgmtv3.MonitoringStatus) error {
	_, err := ConditionKubeStateExporterDeployed.DoUntilFalse(monitoringStatus, func() (*mgmtv3.MonitoringStatus, error) {
		_, err := workloadsClient.Deployments(appNamespace).Get("exporter-kube-state-"+appNameSuffix, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return monitoringStatus, nil
			}

			return nil, errors.Wrap(err, "failed to get Kube State Exporter Deployment information")
		}

		return nil, errors.New("Kube State Exporter Deployment is withdrawing")
	})

	return err
}

func isPrometheusDeployed(workloadsClient appsv1beta2.Interface, appNamespace, appNameSuffix string, monitoringStatus *mgmtv3.MonitoringStatus) error {
	_, err := ConditionPrometheusDeployed.DoUntilTrue(monitoringStatus, func() (*mgmtv3.MonitoringStatus, error) {
		obj, err := workloadsClient.StatefulSets(appNamespace).Get("prometheus-"+appNameSuffix, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return nil, errors.New("Prometheus StatefulSet isn't deployed")
			}

			return nil, errors.Wrap(err, "failed to get Prometheus StatefulSet information")
		}

		if obj.Status.Replicas != obj.Status.CurrentReplicas {
			return nil, errors.New("Prometheus StatefulSet is deploying")
		}

		return monitoringStatus, nil
	})

	return err
}

func isPrometheusWithdrew(workloadsClient appsv1beta2.Interface, appNamespace, appNameSuffix string, monitoringStatus *mgmtv3.MonitoringStatus) error {
	_, err := ConditionPrometheusDeployed.DoUntilFalse(monitoringStatus, func() (*mgmtv3.MonitoringStatus, error) {
		_, err := workloadsClient.StatefulSets(appNamespace).Get("prometheus-"+appNameSuffix, metav1.GetOptions{})
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

func isMetricExpressionDeployed(clusterName string, clusterGraphClient mgmtv3.ClusterMonitorGraphInterface, monitorMetricsClient mgmtv3.MonitorMetricInterface, monitoringStatus *mgmtv3.MonitoringStatus) error {
	_, err := ConditionMetricExpressionDeployed.DoUntilTrue(monitoringStatus, func() (*mgmtv3.MonitoringStatus, error) {
		for _, metric := range preDefinedClusterMetrics {
			newObj := metric.DeepCopy()
			newObj.Namespace = clusterName
			if _, err := monitorMetricsClient.Create(newObj); err != nil && !apierrors.IsAlreadyExists(err) {
				return monitoringStatus, err
			}
		}
		for _, graph := range preDefinedClusterGraph {
			newObj := graph.DeepCopy()
			newObj.Namespace = clusterName
			if _, err := clusterGraphClient.Create(newObj); err != nil && !apierrors.IsAlreadyExists(err) {
				return monitoringStatus, err
			}
		}
		return monitoringStatus, nil
	})

	if err != nil {
		return errors.Wrapf(err, "failed to deploy metric expression into Cluster %s", clusterName)
	}
	return nil
}

func isMetricExpressionWithdrew(clusterName string, clusterGraphClient mgmtv3.ClusterMonitorGraphInterface, monitorMetricsClient mgmtv3.MonitorMetricInterface, monitoringStatus *mgmtv3.MonitoringStatus) error {
	_, err := ConditionMetricExpressionDeployed.DoUntilFalse(monitoringStatus, func() (*mgmtv3.MonitoringStatus, error) {
		for _, metric := range preDefinedClusterMetrics {
			if err := monitorMetricsClient.DeleteNamespaced(clusterName, metric.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return monitoringStatus, err
			}
		}
		for _, graph := range preDefinedClusterGraph {
			if err := clusterGraphClient.DeleteNamespaced(clusterName, graph.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return monitoringStatus, err
			}
		}
		return monitoringStatus, nil
	})

	if err != nil {
		return errors.Wrapf(err, "failed to deploy metric expression into Cluster %s", clusterName)
	}
	return nil
}

func filterError(except string, buf *bytes.Buffer) string {
	var line string
	var err error
	newBuf := &bytes.Buffer{}
	for {
		line, err = buf.ReadString('\n')
		if err != nil {
			break
		}

		if !strings.Contains(line, except) {
			newBuf.WriteString(line)
			newBuf.WriteString("\n")
		}
	}

	if err != io.EOF {
		newBuf.WriteString(err.Error())
		newBuf.WriteString("\n")
		return newBuf.String()
	}

	return newBuf.String()
}
