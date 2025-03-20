package integration

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	rv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	"github.com/rancher/shepherd/extensions/kubeconfig"
	"github.com/rancher/shepherd/pkg/api/steve/catalog/types"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"helm.sh/helm/v3/pkg/action"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/pkg/util/taints"
)

var defaultPodTolerations = []v1.Toleration{
	{
		Key:      "cattle.io/os",
		Operator: v1.TolerationOpEqual,
		Value:    "linux",
		Effect:   "NoSchedule",
	},
	{
		Key:      "node-role.kubernetes.io/controlplane",
		Operator: v1.TolerationOpEqual,
		Value:    "true",
		Effect:   "NoSchedule",
	},
	{
		Key:      "node-role.kubernetes.io/control-plane",
		Operator: v1.TolerationOpExists,
		Effect:   "NoSchedule",
	},
	{
		Key:      "node-role.kubernetes.io/etcd",
		Operator: v1.TolerationOpExists,
		Effect:   "NoExecute",
	},
	{
		Key:      "node.cloudprovider.kubernetes.io/uninitialized",
		Operator: v1.TolerationOpEqual,
		Value:    "true",
		Effect:   "NoSchedule",
	},
}

type ChartsTest struct {
	suite.Suite
	client           *rancher.Client
	session          *session.Session
	restClientGetter genericclioptions.RESTClientGetter
	catalogClient    *catalog.Client
	corev1           corev1.CoreV1Interface
	backoff          kwait.Backoff
}

func (w *ChartsTest) TearDownSuite() {
	w.Require().NoError(w.catalogClient.ClusterRepos().Delete(context.Background(), "charts-small-fork", metav1.DeleteOptions{PropagationPolicy: &propagation}))
	taint := v1.Taint{
		Key:    "testTaint",
		Value:  "testValue",
		Effect: v1.TaintEffectPreferNoSchedule,
	}
	w.updateTaintOnNode(context.Background(), taint, taints.RemoveTaint)
	w.session.Cleanup()

}

func (w *ChartsTest) SetupSuite() {
	var err error
	testSession := session.NewSession()
	w.session = testSession
	w.client, err = rancher.NewClient("", testSession)
	require.NoError(w.T(), err)
	insecure := true
	w.client.RancherConfig.Insecure = &insecure
	w.catalogClient, err = w.client.GetClusterCatalogClient("local")
	require.NoError(w.T(), err)

	kubeConfig, err := kubeconfig.GetKubeconfig(w.client, "local")
	require.NoError(w.T(), err)

	restConfig, err := (*kubeConfig).ClientConfig()
	require.NoError(w.T(), err)
	cset, err := kubernetes.NewForConfig(restConfig)
	require.NoError(w.T(), err)
	w.corev1 = cset.CoreV1()

	w.restClientGetter, err = kubeconfig.NewRestGetter(restConfig, *kubeConfig)
	require.NoError(w.T(), err)
	_, err = w.catalogClient.ClusterRepos().Create(context.Background(), &rv1.ClusterRepo{
		ObjectMeta: metav1.ObjectMeta{Name: "charts-small-fork"},
		Spec:       rv1.RepoSpec{GitRepo: "https://github.com/rancher/charts-small-fork", GitBranch: "aks-integration-test-working-charts"},
	}, metav1.CreateOptions{})
	w.Require().NoError(err)
	w.Require().NoError(w.pollUntilDownloaded("charts-small-fork", metav1.Time{}))

	w.backoff = kwait.Backoff{
		Duration: 500 * time.Millisecond,
		Jitter:   0.2,
		Factor:   2,
		Steps:    10,
		Cap:      60 * time.Second,
	}
}

func TestChartsTestSuite(t *testing.T) {
	suite.Run(t, new(ChartsTest))
}

// TestInstallChartWithAutomaticTolerationOnTaintedCPNode tests the installation of a chart with automatic CP toleration on a tainted control plane node
func (w *ChartsTest) TestInstallChartWithAutomaticTolerationOnTaintedCPNode() {
	ctx := context.Background()
	taint := v1.Taint{
		Key:    "testTaint",
		Value:  "testValue",
		Effect: v1.TaintEffectPreferNoSchedule,
	}

	w.updateTaintOnNode(ctx, taint, taints.AddOrUpdateTaint)

	w.Require().NoError(w.catalogClient.InstallChart(&types.ChartInstallAction{
		DisableHooks:             false,
		Timeout:                  &metav1.Duration{Duration: 60 * time.Second},
		Wait:                     true,
		Namespace:                namespace.System,
		DisableOpenAPIValidation: false,
		AutomaticCPTolerations:   true,
		Charts: []types.ChartInstall{{
			ChartName:   "rancher-aks-operator-crd",
			Version:     "104.0.2+up1.9.0",
			ReleaseName: "rancher-aks-operator-crd",
			Description: "rancher aks operator crd",
		}},
	}, "charts-small-fork"))

	w.Require().NoError(w.waitForChart(rv1.StatusDeployed, "rancher-aks-operator-crd", 0))

	operationList, err := w.catalogClient.Operations(namespace.System).List(context.Background(), metav1.ListOptions{})
	w.Require().NoError(err)
	sort.Slice(operationList.Items, func(i, j int) bool {
		return !operationList.Items[i].CreationTimestamp.Before(&operationList.Items[j].CreationTimestamp)
	})

	var op rv1.Operation
	for _, item := range operationList.Items {
		if item.Status.Release == "rancher-aks-operator-crd" {
			op = item
			break
		}
	}

	pod, err := w.corev1.Pods(op.Status.PodNamespace).Get(ctx, op.Status.PodName, metav1.GetOptions{})
	w.Require().NoError(err)
	found := false
	for _, t := range pod.Spec.Tolerations {
		if t.Key == "testTaint" {
			found = true
			break
		}
	}
	w.Require().True(found)

	w.Require().NoError(w.uninstallApp(namespace.System, "rancher-aks-operator-crd"))

	w.updateTaintOnNode(ctx, taint, taints.RemoveTaint)
}

// TestInstallChartWithCustomTolerationOnTaintedCPNode tests the installation of a chart with custom toleration on a tainted control plane node
func (w *ChartsTest) TestInstallChartWithCustomTolerationOnTaintedCPNode() {
	ctx := context.Background()
	taint := v1.Taint{
		Key:    "testTaint",
		Value:  "testValue",
		Effect: v1.TaintEffectPreferNoSchedule,
	}

	w.updateTaintOnNode(ctx, taint, taints.AddOrUpdateTaint)

	w.Require().NoError(w.catalogClient.InstallChart(&types.ChartInstallAction{
		DisableHooks:             false,
		Timeout:                  &metav1.Duration{Duration: 60 * time.Second},
		Wait:                     true,
		Namespace:                namespace.System,
		DisableOpenAPIValidation: false,
		AutomaticCPTolerations:   false,
		OperationTolerations:     []v1.Toleration{{Key: "testTaint", Effect: v1.TaintEffectNoSchedule, Value: "testValue"}},
		Charts: []types.ChartInstall{{
			ChartName:   "rancher-aks-operator-crd",
			Version:     "104.0.2+up1.9.0",
			ReleaseName: "rancher-aks-operator-crd",
			Description: "rancher aks operator crd",
		}},
	}, "charts-small-fork"))

	w.Require().NoError(w.waitForChart(rv1.StatusDeployed, "rancher-aks-operator-crd", 0))

	operationList, err := w.catalogClient.Operations(namespace.System).List(context.Background(), metav1.ListOptions{})
	w.Require().NoError(err)
	sort.Slice(operationList.Items, func(i, j int) bool {
		return !operationList.Items[i].CreationTimestamp.Before(&operationList.Items[j].CreationTimestamp)
	})

	var op rv1.Operation
	for _, item := range operationList.Items {
		if item.Status.Release == "rancher-aks-operator-crd" {
			op = item
			break
		}
	}

	pod, err := w.corev1.Pods(op.Status.PodNamespace).Get(ctx, op.Status.PodName, metav1.GetOptions{})
	w.Require().NoError(err)
	found := false
	for _, t := range pod.Spec.Tolerations {
		if t.Key == "testTaint" {
			found = true
			break
		}
	}
	w.Require().True(found)

	w.Require().NoError(w.uninstallApp(namespace.System, "rancher-aks-operator-crd"))

	w.updateTaintOnNode(ctx, taint, taints.RemoveTaint)
}

// TestUpgradeChartWithCustomTolerationOnTaintedCPNode tests the upgrade of a chart with custom toleration on a tainted control plane node
func (w *ChartsTest) TestUpgradeChartWithCustomTolerationOnTaintedCPNode() {
	ctx := context.Background()
	taint := v1.Taint{
		Key:    "testTaint",
		Value:  "testValue",
		Effect: v1.TaintEffectPreferNoSchedule,
	}

	w.updateTaintOnNode(ctx, taint, taints.AddOrUpdateTaint)

	//install chart
	w.Require().NoError(w.catalogClient.InstallChart(&types.ChartInstallAction{
		DisableHooks:             false,
		Timeout:                  &metav1.Duration{Duration: 60 * time.Second},
		Wait:                     true,
		Namespace:                namespace.System,
		DisableOpenAPIValidation: false,
		AutomaticCPTolerations:   false,
		OperationTolerations:     []v1.Toleration{{Key: "testTaint", Effect: v1.TaintEffectNoSchedule, Value: "testValue"}},
		Charts: []types.ChartInstall{{
			ChartName:   "rancher-aks-operator-crd",
			Version:     "104.0.1+up1.9.0",
			ReleaseName: "rancher-aks-operator-crd",
			Description: "rancher aks operator crd",
		}},
	}, "charts-small-fork"))

	w.Require().NoError(w.waitForChart(rv1.StatusDeployed, "rancher-aks-operator-crd", 0))

	operationList, err := w.catalogClient.Operations(namespace.System).List(context.Background(), metav1.ListOptions{})
	w.Require().NoError(err)
	sort.Slice(operationList.Items, func(i, j int) bool {
		return !operationList.Items[i].CreationTimestamp.Before(&operationList.Items[j].CreationTimestamp)
	})

	var op rv1.Operation
	for _, item := range operationList.Items {
		if item.Status.Release == "rancher-aks-operator-crd" {
			op = item
			break
		}
	}

	pod, err := w.corev1.Pods(op.Status.PodNamespace).Get(ctx, op.Status.PodName, metav1.GetOptions{})
	w.Require().NoError(err)
	found := false
	for _, t := range pod.Spec.Tolerations {
		if t.Key == "testTaint" {
			found = true
			break
		}
	}
	w.Require().True(found)

	//upgrade chart
	w.Require().NoError(w.catalogClient.UpgradeChart(&types.ChartUpgradeAction{
		DisableHooks:             false,
		Timeout:                  &metav1.Duration{Duration: 60 * time.Second},
		Wait:                     true,
		Namespace:                namespace.System,
		DisableOpenAPIValidation: false,
		AutomaticCPTolerations:   false,
		OperationTolerations:     []v1.Toleration{{Key: "testTaint", Effect: v1.TaintEffectNoSchedule, Value: "testValue"}},
		Charts: []types.ChartUpgrade{{
			ChartName:   "rancher-aks-operator-crd",
			Version:     "104.0.2+up1.9.0",
			ReleaseName: "rancher-aks-operator-crd",
			Description: "rancher aks operator crd",
		}},
	}, "charts-small-fork"))

	w.Require().NoError(w.waitForChart(rv1.StatusDeployed, "rancher-aks-operator-crd", 1))

	operationList, err = w.catalogClient.Operations(namespace.System).List(context.Background(), metav1.ListOptions{})
	w.Require().NoError(err)
	sort.Slice(operationList.Items, func(i, j int) bool {
		return !operationList.Items[i].CreationTimestamp.Before(&operationList.Items[j].CreationTimestamp)
	})

	for _, item := range operationList.Items {
		if item.Status.Release == "rancher-aks-operator-crd" {
			op = item
			break
		}
	}

	pod, err = w.corev1.Pods(op.Status.PodNamespace).Get(ctx, op.Status.PodName, metav1.GetOptions{})
	w.Require().NoError(err)
	found = false
	for _, t := range pod.Spec.Tolerations {
		if t.Key == "testTaint" {
			found = true
			break
		}
	}
	w.Require().True(found)

	//cleanup
	w.Require().NoError(w.uninstallApp(namespace.System, "rancher-aks-operator-crd"))

	w.updateTaintOnNode(ctx, taint, taints.RemoveTaint)
}

// TestUpgradeChartWithAutomaticTolerationOnTaintedCPNode tests the upgrade of a chart with automatic CP toleration on a tainted control plane node
func (w *ChartsTest) TestUpgradeChartWithAutomaticTolerationOnTaintedCPNode() {
	ctx := context.Background()
	taint := v1.Taint{
		Key:    "testTaint",
		Value:  "testValue",
		Effect: v1.TaintEffectPreferNoSchedule,
	}

	w.updateTaintOnNode(ctx, taint, taints.AddOrUpdateTaint)

	//install chart
	w.Require().NoError(w.catalogClient.InstallChart(&types.ChartInstallAction{
		DisableHooks:             false,
		Timeout:                  &metav1.Duration{Duration: 60 * time.Second},
		Wait:                     true,
		Namespace:                namespace.System,
		DisableOpenAPIValidation: false,
		AutomaticCPTolerations:   false,
		OperationTolerations:     []v1.Toleration{{Key: "testTaint", Effect: v1.TaintEffectNoSchedule, Value: "testValue"}},
		Charts: []types.ChartInstall{{
			ChartName:   "rancher-aks-operator-crd",
			Version:     "104.0.1+up1.9.0",
			ReleaseName: "rancher-aks-operator-crd",
			Description: "rancher aks operator crd",
		}},
	}, "charts-small-fork"))

	w.Require().NoError(w.waitForChart(rv1.StatusDeployed, "rancher-aks-operator-crd", 0))

	operationList, err := w.catalogClient.Operations(namespace.System).List(context.Background(), metav1.ListOptions{})
	w.Require().NoError(err)
	sort.Slice(operationList.Items, func(i, j int) bool {
		return !operationList.Items[i].CreationTimestamp.Before(&operationList.Items[j].CreationTimestamp)
	})

	var op rv1.Operation
	for _, item := range operationList.Items {
		if item.Status.Release == "rancher-aks-operator-crd" {
			op = item
			break
		}
	}

	pod, err := w.corev1.Pods(op.Status.PodNamespace).Get(ctx, op.Status.PodName, metav1.GetOptions{})
	w.Require().NoError(err)
	found := false
	for _, t := range pod.Spec.Tolerations {
		if t.Key == "testTaint" {
			found = true
			break
		}
	}
	w.Require().True(found)

	//upgrade chart
	w.Require().NoError(w.catalogClient.UpgradeChart(&types.ChartUpgradeAction{
		DisableHooks:             false,
		Timeout:                  &metav1.Duration{Duration: 60 * time.Second},
		Wait:                     true,
		Namespace:                namespace.System,
		DisableOpenAPIValidation: false,
		AutomaticCPTolerations:   true,
		Charts: []types.ChartUpgrade{{
			ChartName:   "rancher-aks-operator-crd",
			Version:     "104.0.2+up1.9.0",
			ReleaseName: "rancher-aks-operator-crd",
			Description: "rancher aks operator crd",
		}},
	}, "charts-small-fork"))

	w.Require().NoError(w.waitForChart(rv1.StatusDeployed, "rancher-aks-operator-crd", 1))

	operationList, err = w.catalogClient.Operations(namespace.System).List(context.Background(), metav1.ListOptions{})
	w.Require().NoError(err)
	sort.Slice(operationList.Items, func(i, j int) bool {
		return !operationList.Items[i].CreationTimestamp.Before(&operationList.Items[j].CreationTimestamp)
	})

	for _, item := range operationList.Items {
		if item.Status.Release == "rancher-aks-operator-crd" {
			op = item
			break
		}
	}

	pod, err = w.corev1.Pods(op.Status.PodNamespace).Get(ctx, op.Status.PodName, metav1.GetOptions{})
	w.Require().NoError(err)
	found = false
	for _, t := range pod.Spec.Tolerations {
		if t.Key == "testTaint" {
			found = true
			break
		}
	}
	w.Require().True(found)

	//cleanup
	w.Require().NoError(w.uninstallApp(namespace.System, "rancher-aks-operator-crd"))

	w.updateTaintOnNode(ctx, taint, taints.RemoveTaint)
}

// TestUpgradeChartWithAutomaticTolerationOnTaintedCPNode tests the upgrade of a chart that was installed without automatic tolerations
// with automatic CP toleration on a tainted control plane node enabled for the upgrade
func (w *ChartsTest) TestUpgradeChartInstalledWithoutTolerationsUsingAutomaticTolerations() {
	ctx := context.Background()

	//install chart
	w.Require().NoError(w.catalogClient.InstallChart(&types.ChartInstallAction{
		DisableHooks:             false,
		Timeout:                  &metav1.Duration{Duration: 60 * time.Second},
		Wait:                     true,
		Namespace:                namespace.System,
		DisableOpenAPIValidation: false,
		AutomaticCPTolerations:   false,
		Charts: []types.ChartInstall{{
			ChartName:   "rancher-aks-operator-crd",
			Version:     "104.0.1+up1.9.0",
			ReleaseName: "rancher-aks-operator-crd",
			Description: "rancher aks operator crd",
		}},
	}, "charts-small-fork"))

	w.Require().NoError(w.waitForChart(rv1.StatusDeployed, "rancher-aks-operator-crd", 0))

	operationList, err := w.catalogClient.Operations(namespace.System).List(context.Background(), metav1.ListOptions{})
	w.Require().NoError(err)
	sort.Slice(operationList.Items, func(i, j int) bool {
		return !operationList.Items[i].CreationTimestamp.Before(&operationList.Items[j].CreationTimestamp)
	})

	var op rv1.Operation
	for _, item := range operationList.Items {
		if item.Status.Release == "rancher-aks-operator-crd" {
			op = item
			break
		}
	}

	pod, err := w.corev1.Pods(op.Status.PodNamespace).Get(ctx, op.Status.PodName, metav1.GetOptions{})
	w.Require().NoError(err)
	for _, toleration := range defaultPodTolerations {
		w.Require().Contains(pod.Spec.Tolerations, toleration)
	}

	//add taint to node
	taint := v1.Taint{
		Key:    "testTaint",
		Value:  "testValue",
		Effect: v1.TaintEffectPreferNoSchedule,
	}

	w.updateTaintOnNode(ctx, taint, taints.AddOrUpdateTaint)

	//upgrade chart
	w.Require().NoError(w.catalogClient.UpgradeChart(&types.ChartUpgradeAction{
		DisableHooks:             false,
		Timeout:                  &metav1.Duration{Duration: 60 * time.Second},
		Wait:                     true,
		Namespace:                namespace.System,
		DisableOpenAPIValidation: false,
		AutomaticCPTolerations:   true,
		Charts: []types.ChartUpgrade{{
			ChartName:   "rancher-aks-operator-crd",
			Version:     "104.0.2+up1.9.0",
			ReleaseName: "rancher-aks-operator-crd",
			Description: "rancher aks operator crd",
		}},
	}, "charts-small-fork"))

	w.Require().NoError(w.waitForChart(rv1.StatusDeployed, "rancher-aks-operator-crd", 1))

	operationList, err = w.catalogClient.Operations(namespace.System).List(context.Background(), metav1.ListOptions{})
	w.Require().NoError(err)
	sort.Slice(operationList.Items, func(i, j int) bool {
		return !operationList.Items[i].CreationTimestamp.Before(&operationList.Items[j].CreationTimestamp)
	})

	for _, item := range operationList.Items {
		if item.Status.Release == "rancher-aks-operator-crd" {
			op = item
			break
		}
	}

	pod, err = w.corev1.Pods(op.Status.PodNamespace).Get(ctx, op.Status.PodName, metav1.GetOptions{})
	w.Require().NoError(err)
	found := false
	for _, t := range pod.Spec.Tolerations {
		if t.Key == "testTaint" {
			found = true
			break
		}
	}
	w.Require().True(found)

	//cleanup
	w.Require().NoError(w.uninstallApp(namespace.System, "rancher-aks-operator-crd"))

	w.updateTaintOnNode(ctx, taint, taints.RemoveTaint)
}

// TestUninstallChartWithAutomaticTolerationOnTaintedCPNode tests the uninstallation of a chart with automatic CP toleration on a tainted control plane node
func (w *ChartsTest) TestUninstallChartWithAutomaticTolerationOnTaintedCPNode() {
	ctx := context.Background()
	taint := v1.Taint{
		Key:    "testTaint",
		Value:  "testValue",
		Effect: v1.TaintEffectPreferNoSchedule,
	}

	w.updateTaintOnNode(ctx, taint, taints.AddOrUpdateTaint)

	//install chart
	w.Require().NoError(w.catalogClient.InstallChart(&types.ChartInstallAction{
		DisableHooks:             false,
		Timeout:                  &metav1.Duration{Duration: 60 * time.Second},
		Wait:                     true,
		Namespace:                namespace.System,
		DisableOpenAPIValidation: false,
		AutomaticCPTolerations:   false,
		OperationTolerations:     []v1.Toleration{{Key: "testTaint", Effect: v1.TaintEffectNoSchedule, Value: "testValue"}},
		Charts: []types.ChartInstall{{
			ChartName:   "rancher-aks-operator-crd",
			Version:     "104.0.2+up1.9.0",
			ReleaseName: "rancher-aks-operator-crd",
			Description: "rancher aks operator crd",
		}},
	}, "charts-small-fork"))

	w.Require().NoError(w.waitForChart(rv1.StatusDeployed, "rancher-aks-operator-crd", 0))

	operationList, err := w.catalogClient.Operations(namespace.System).List(context.Background(), metav1.ListOptions{})
	w.Require().NoError(err)
	sort.Slice(operationList.Items, func(i, j int) bool {
		return !operationList.Items[i].CreationTimestamp.Before(&operationList.Items[j].CreationTimestamp)
	})

	var op rv1.Operation
	for _, item := range operationList.Items {
		if item.Status.Release == "rancher-aks-operator-crd" {
			op = item
			break
		}
	}

	pod, err := w.corev1.Pods(op.Status.PodNamespace).Get(ctx, op.Status.PodName, metav1.GetOptions{})
	w.Require().NoError(err)
	found := false
	for _, t := range pod.Spec.Tolerations {
		if t.Key == "testTaint" {
			found = true
			break
		}
	}
	w.Require().True(found)

	//uninstall chart
	w.Require().NoError(w.catalogClient.UninstallChart("rancher-aks-operator-crd", namespace.System, &types.ChartUninstallAction{
		DisableHooks:           false,
		Timeout:                &metav1.Duration{Duration: 60 * time.Second},
		AutomaticCPTolerations: true,
	}))

	w.Require().NoError(w.waitForChart(rv1.StatusUninstalled, "rancher-aks-operator-crd", 1))

	operationList, err = w.catalogClient.Operations(namespace.System).List(context.Background(), metav1.ListOptions{})
	w.Require().NoError(err)
	sort.Slice(operationList.Items, func(i, j int) bool {
		return !operationList.Items[i].CreationTimestamp.Before(&operationList.Items[j].CreationTimestamp)
	})

	for _, item := range operationList.Items {
		if item.Status.Release == "rancher-aks-operator-crd" {
			op = item
			break
		}
	}

	pod, err = w.corev1.Pods(op.Status.PodNamespace).Get(ctx, op.Status.PodName, metav1.GetOptions{})
	w.Require().NoError(err)
	found = false
	for _, t := range pod.Spec.Tolerations {
		if t.Key == "testTaint" {
			found = true
			break
		}
	}
	w.Require().True(found)

	//cleanup
	w.updateTaintOnNode(ctx, taint, taints.RemoveTaint)
}

// TestUninstallChartWithCustomTolerationOnTaintedCPNode tests the uninstallation of a chart with custom toleration on a tainted control plane node
func (w *ChartsTest) TestUninstallChartWithCustomTolerationOnTaintedCPNode() {
	ctx := context.Background()
	taint := v1.Taint{
		Key:    "testTaint",
		Value:  "testValue",
		Effect: v1.TaintEffectPreferNoSchedule,
	}

	w.updateTaintOnNode(ctx, taint, taints.AddOrUpdateTaint)

	//install chart
	w.Require().NoError(w.catalogClient.InstallChart(&types.ChartInstallAction{
		DisableHooks:             false,
		Timeout:                  &metav1.Duration{Duration: 60 * time.Second},
		Wait:                     true,
		Namespace:                namespace.System,
		DisableOpenAPIValidation: false,
		AutomaticCPTolerations:   false,
		OperationTolerations:     []v1.Toleration{{Key: "testTaint", Effect: v1.TaintEffectNoSchedule, Value: "testValue"}},
		Charts: []types.ChartInstall{{
			ChartName:   "rancher-aks-operator-crd",
			Version:     "104.0.2+up1.9.0",
			ReleaseName: "rancher-aks-operator-crd",
			Description: "rancher aks operator crd",
		}},
	}, "charts-small-fork"))

	w.Require().NoError(w.waitForChart(rv1.StatusDeployed, "rancher-aks-operator-crd", 0))

	operationList, err := w.catalogClient.Operations(namespace.System).List(context.Background(), metav1.ListOptions{})
	w.Require().NoError(err)
	sort.Slice(operationList.Items, func(i, j int) bool {
		return !operationList.Items[i].CreationTimestamp.Before(&operationList.Items[j].CreationTimestamp)
	})

	var op rv1.Operation
	for _, item := range operationList.Items {
		if item.Status.Release == "rancher-aks-operator-crd" {
			op = item
			break
		}
	}

	pod, err := w.corev1.Pods(op.Status.PodNamespace).Get(ctx, op.Status.PodName, metav1.GetOptions{})
	w.Require().NoError(err)
	found := false
	for _, t := range pod.Spec.Tolerations {
		if t.Key == "testTaint" {
			found = true
			break
		}
	}
	w.Require().True(found)

	//uninstall chart
	w.Require().NoError(w.catalogClient.UninstallChart("rancher-aks-operator-crd", namespace.System, &types.ChartUninstallAction{
		DisableHooks:           false,
		Timeout:                &metav1.Duration{Duration: 60 * time.Second},
		AutomaticCPTolerations: false,
		OperationTolerations:   []v1.Toleration{{Key: "testTaint", Effect: v1.TaintEffectNoSchedule, Value: "testValue"}},
	}))

	w.Require().NoError(w.waitForChart(rv1.StatusUninstalled, "rancher-aks-operator-crd", 1))

	operationList, err = w.catalogClient.Operations(namespace.System).List(context.Background(), metav1.ListOptions{})
	w.Require().NoError(err)
	sort.Slice(operationList.Items, func(i, j int) bool {
		return !operationList.Items[i].CreationTimestamp.Before(&operationList.Items[j].CreationTimestamp)
	})

	for _, item := range operationList.Items {
		if item.Status.Release == "rancher-aks-operator-crd" {
			op = item
			break
		}
	}

	pod, err = w.corev1.Pods(op.Status.PodNamespace).Get(ctx, op.Status.PodName, metav1.GetOptions{})
	w.Require().NoError(err)
	found = false
	for _, t := range pod.Spec.Tolerations {
		if t.Key == "testTaint" {
			found = true
			break
		}
	}
	w.Require().True(found)

	//cleanup
	w.updateTaintOnNode(ctx, taint, taints.RemoveTaint)
}

func (w *ChartsTest) waitForChart(status rv1.Status, name string, previousVersion int) error {
	var app *rv1.App
	err := kwait.Poll(500*time.Millisecond, 360*time.Second, func() (done bool, err error) {
		app, err = w.catalogClient.Apps(namespace.System).Get(context.TODO(), name, metav1.GetOptions{})
		e, ok := err.(*errors.StatusError)
		if ok && errors.IsNotFound(e) {
			if status == rv1.StatusUninstalled {
				return true, nil
			}
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if app.Spec.Info.Status == status && app.Spec.Version > previousVersion {
			return true, nil
		}
		return false, nil
	})
	w.Require().NoError(err)
	return nil
}

func (w *ChartsTest) uninstallApp(namespace, chartName string) error {
	var cfg action.Configuration
	if err := cfg.Init(w.restClientGetter, namespace, "", logrus.Infof); err != nil {
		return err
	}
	l := action.NewList(&cfg)
	l.All = true
	l.SetStateMask()
	releases, err := l.Run()
	if err != nil {
		return fmt.Errorf("failed to fetch all releases in the %s namespace: %w", namespace, err)
	}
	for _, r := range releases {
		if r.Chart.Name() == chartName {
			err = kwait.Poll(10*time.Second, time.Minute, func() (done bool, err error) {
				act := action.NewUninstall(&cfg)
				act.Wait = true
				act.Timeout = time.Minute
				if _, err = act.Run(r.Name); err != nil {
					return false, nil
				}
				return true, nil
			})
			w.Require().NoError(err)
		}
	}
	return nil
}

// pollUntilDownloaded Polls until the ClusterRepo of the given name has been downloaded (by comparing prevDownloadTime against the current DownloadTime)
func (w *ChartsTest) pollUntilDownloaded(ClusterRepoName string, prevDownloadTime metav1.Time) error {
	err := kwait.Poll(PollInterval, time.Minute, func() (done bool, err error) {
		clusterRepo, err := w.catalogClient.ClusterRepos().Get(context.TODO(), ClusterRepoName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		w.Require().NoError(err)

		return clusterRepo.Status.DownloadTime != prevDownloadTime, nil
	})
	return err
}

// updateTaintOnNode updates the taint on the control plane node according to the taintFunc received
func (w *ChartsTest) updateTaintOnNode(ctx context.Context, taint v1.Taint, taintFunc func(node *v1.Node, taint *v1.Taint) (*v1.Node, bool, error)) {
	w.Require().NoError(retry.RetryOnConflict(w.backoff, func() error {
		list, err := w.corev1.Nodes().List(ctx, metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/control-plane=true"})
		if err != nil {
			return err
		}
		w.Require().Greater(len(list.Items), 0)
		node := list.Items[0]
		n, b, err := taintFunc(&node, &taint)
		if err != nil {
			return err
		}
		if b {
			_, err = w.corev1.Nodes().Update(ctx, n, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
		return nil
	}))
}
