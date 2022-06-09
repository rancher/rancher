package istio

import (
	"net/url"
	"strings"

	v32 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"

	"github.com/pkg/errors"
	cutils "github.com/rancher/rancher/pkg/catalog/utils"
	v3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"

	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	istioTemplateName = "rancher-istio"
	istioAppName      = "cluster-istio"
)

var (
	preDefinedIstioMetrics      = getPredefinedIstioMetrics()
	preDefinedIstioClusterGraph = getPredefinedIstioClusterGraph()
	PreDefinedIstioProjectGraph = getPredefinedIstioProjectGraph()
)

type appHandler struct {
	istioClusterGraphClient  mgmtv3.ClusterMonitorGraphInterface
	istioMonitorMetricClient mgmtv3.MonitorMetricInterface
	clusterInterface         mgmtv3.ClusterInterface

	clusterName string
}

func isIstioApp(obj *v3.App) bool {
	if obj.Name != istioAppName {
		return false
	}
	values, err := url.Parse(obj.Spec.ExternalID)
	if err != nil {
		logrus.Errorf("check catalog type failed: %s", err.Error())
		return false
	}

	catalogWithNamespace := values.Query().Get("catalog")
	split := strings.SplitN(catalogWithNamespace, "/", 2)

	catalog := split[len(split)-1]

	template := values.Query().Get("template")

	if catalog == cutils.SystemLibraryName && template == istioTemplateName {
		return true
	}
	return false
}

func isIstioMetricExpressionDeployed(clusterName string, istioClusterGraphClient mgmtv3.ClusterMonitorGraphInterface, istioMetricsClient mgmtv3.MonitorMetricInterface, obj *v3.App) error {
	_, err := v32.IstioConditionMetricExpressionDeployed.DoUntilTrue(obj, func() (runtime.Object, error) {
		for _, metric := range preDefinedIstioMetrics {
			newObj := metric.DeepCopy()
			newObj.Namespace = clusterName
			if _, err := istioMetricsClient.Create(newObj); err != nil && !apierrors.IsAlreadyExists(err) {
				return obj, err
			}
		}
		for _, clusterGraph := range preDefinedIstioClusterGraph {
			newObj := clusterGraph.DeepCopy()
			newObj.Namespace = clusterName
			if _, err := istioClusterGraphClient.Create(newObj); err != nil && !apierrors.IsAlreadyExists(err) {
				return obj, err
			}
		}
		for _, projectGraph := range PreDefinedIstioProjectGraph {
			newObj := projectGraph.DeepCopy()
			newObj.Namespace = clusterName
			if _, err := istioClusterGraphClient.Create(newObj); err != nil && !apierrors.IsAlreadyExists(err) {
				return obj, err
			}
		}

		return obj, nil
	})

	if err != nil {
		return errors.Wrapf(err, "failed to deploy istio metric expression into Cluster %s", clusterName)
	}

	logrus.Infof("deploy istio metric expression into CLuster %s successfully", clusterName)

	return nil
}

func isIstioMetricExpressionWithdraw(clusterName string, istioClusterGraphClient mgmtv3.ClusterMonitorGraphInterface, istioMetricsClient mgmtv3.MonitorMetricInterface, obj *v3.App) error {
	for _, metric := range preDefinedIstioMetrics {
		if err := istioMetricsClient.DeleteNamespaced(clusterName, metric.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	for _, clusterGraph := range preDefinedIstioClusterGraph {
		if err := istioClusterGraphClient.DeleteNamespaced(clusterName, clusterGraph.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	for _, projectGraph := range PreDefinedIstioProjectGraph {
		if err := istioClusterGraphClient.DeleteNamespaced(clusterName, projectGraph.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	logrus.Infof("withdraw istio metric expression from CLuster %s successfully", clusterName)

	return nil
}

func (ah *appHandler) sync(key string, obj *v3.App) (runtime.Object, error) {
	if obj == nil {
		return obj, nil
	}

	if !isIstioApp(obj) {
		return obj, nil
	}

	if obj == nil || obj.DeletionTimestamp != nil {
		err := ah.updateClusterIstioCondition(false)
		if err != nil {
			return obj, err
		}
		return obj, isIstioMetricExpressionWithdraw(ah.clusterName, ah.istioClusterGraphClient, ah.istioMonitorMetricClient, obj)
	}

	if err := ah.updateClusterIstioCondition(true); err != nil {
		return obj, err
	}

	if v32.AppConditionInstalled.IsFalse(obj) {
		return obj, errors.Errorf("waiting for the app %s to be installed", obj.Name)
	}

	return obj, isIstioMetricExpressionDeployed(ah.clusterName, ah.istioClusterGraphClient, ah.istioMonitorMetricClient, obj)
}

func (ah *appHandler) updateClusterIstioCondition(enabled bool) error {
	cluster, err := ah.clusterInterface.Get(ah.clusterName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if cluster.Status.IstioEnabled == enabled {
		return nil
	}
	cluster.Status.IstioEnabled = enabled
	_, err = ah.clusterInterface.Update(cluster)
	return err
}
