package charts

import (
	"context"
	"time"

	"github.com/rancher/rancher/pkg/api/scheme"
	catalogv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	steveV1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/defaults"
	"github.com/rancher/rancher/tests/framework/extensions/kubeapi/workloads/daemonsets"
	"github.com/rancher/rancher/tests/framework/extensions/kubeapi/workloads/deployments"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	appv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	// defaultRegistrySettingID is a private constant string that contains the ID of system default registry setting.
	defaultRegistrySettingID = "system-default-registry"
	// serverURLSettingID is a private constant string that contains the ID of server URL setting.
	serverURLSettingID = "server-url"
	rancherChartsName  = "rancher-charts"
	active             = "active"
)

// InstallOptions is a struct of the required options to install a chart.
type InstallOptions struct {
	Version     string
	ClusterID   string
	ClusterName string
	ProjectID   string
}

// payloadOpts is a private struct that contains the options for the chart payloads.
// It is used to avoid passing the same options to different functions while using the chart helpers.
type payloadOpts struct {
	InstallOptions
	Name            string
	Namespace       string
	Host            string
	DefaultRegistry string
}

// RancherIstioOpts is a struct of the required options to install Rancher Istio with desired chart values.
type RancherIstioOpts struct {
	IngressGateways bool
	EgressGateways  bool
	Pilot           bool
	Telemetry       bool
	Kiali           bool
	Tracing         bool
	CNI             bool
}

// RancherMonitoringOpts is a struct of the required options to install Rancher Monitoring with desired chart values.
type RancherMonitoringOpts struct {
	IngressNginx         bool
	RKEControllerManager bool
	RKEEtcd              bool
	RKEProxy             bool
	RKEScheduler         bool
}

// RancherLoggingOpts is a struct of the required options to install Rancher Logging with desired chart values.
type RancherLoggingOpts struct {
	AdditionalLoggingSources bool
}

// GetChartCaseEndpointResult is a struct that GetChartCaseEndpoint helper function returns.
// It contains the boolean for healthy response and the request body.
type GetChartCaseEndpointResult struct {
	Ok   bool
	Body string
}

// ChartStatus is a struct that GetChartStatus helper function returns.
// It contains the boolean for is already installed and the chart information.
type ChartStatus struct {
	IsAlreadyInstalled bool
	ChartDetails       *catalogv1.App
}

// GetChartStatus is a helper function that takes client, clusterID, chartNamespace and chartName as args,
// uses admin catalog client to check if chart is already installed, if the chart is already installed returns chart information.
func GetChartStatus(client *rancher.Client, clusterID, chartNamespace, chartName string) (*ChartStatus, error) {
	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	if err != nil {
		return nil, err
	}
	adminCatalogClient, err := adminClient.GetClusterCatalogClient(clusterID)
	if err != nil {
		return nil, err
	}

	chartList, err := adminCatalogClient.Apps(chartNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, chart := range chartList.Items {
		if chart.Name == chartName {
			return &ChartStatus{
				IsAlreadyInstalled: true,
				ChartDetails:       &chart,
			}, nil
		}
	}

	return &ChartStatus{
		IsAlreadyInstalled: false,
		ChartDetails:       nil,
	}, nil
}

// WatchAndWaitDeployments is a helper function that watches the deployments
// sequentially in a specific namespace and waits until number of expected replicas is equal to number of available replicas.
func WatchAndWaitDeployments(client *rancher.Client, clusterID, namespace string, listOptions metav1.ListOptions) error {
	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	if err != nil {
		return err
	}
	adminDynamicClient, err := adminClient.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return err
	}
	adminDeploymentResource := adminDynamicClient.Resource(deployments.DeploymentGroupVersionResource).Namespace(namespace)

	deployments, err := adminDeploymentResource.List(context.TODO(), listOptions)
	if err != nil {
		return err
	}

	var deploymentList []appv1.Deployment

	for _, unstructuredDeployment := range deployments.Items {
		newDeployment := &appv1.Deployment{}
		err := scheme.Scheme.Convert(&unstructuredDeployment, newDeployment, unstructuredDeployment.GroupVersionKind())
		if err != nil {
			return err
		}

		deploymentList = append(deploymentList, *newDeployment)
	}

	for _, deployment := range deploymentList {
		watchAppInterface, err := adminDeploymentResource.Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  "metadata.name=" + deployment.Name,
			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
		})
		if err != nil {
			return err
		}

		wait.WatchWait(watchAppInterface, func(event watch.Event) (ready bool, err error) {
			deploymentsUnstructured := event.Object.(*unstructured.Unstructured)
			deployment := &appv1.Deployment{}

			err = scheme.Scheme.Convert(deploymentsUnstructured, deployment, deploymentsUnstructured.GroupVersionKind())
			if err != nil {
				return false, err
			}

			if *deployment.Spec.Replicas == deployment.Status.AvailableReplicas {
				return true, nil
			}
			return false, nil
		})
	}

	return nil
}

// WatchAndWaitDeploymentForAnnotation is a helper function that watches the deployment
// in a specific namespace and waits until expected annotation key and its value.
func WatchAndWaitDeploymentForAnnotation(client *rancher.Client, clusterID, namespace, deploymentName, annotationKey, annotationValue string) error {
	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	if err != nil {
		return err
	}
	adminDynamicClient, err := adminClient.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return err
	}
	adminDeploymentResource := adminDynamicClient.Resource(deployments.DeploymentGroupVersionResource).Namespace(namespace)

	watchAppInterface, err := adminDeploymentResource.Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + deploymentName,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return err
	}

	err = wait.WatchWait(watchAppInterface, func(event watch.Event) (ready bool, err error) {
		deploymentsUnstructured := event.Object.(*unstructured.Unstructured)
		deployment := &appv1.Deployment{}

		err = scheme.Scheme.Convert(deploymentsUnstructured, deployment, deploymentsUnstructured.GroupVersionKind())
		if err != nil {
			return false, err
		}

		if deployment.ObjectMeta.Annotations[annotationKey] == annotationValue {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return err
	}

	return nil
}

// WatchAndWaitDaemonSets is a helper function that watches the DaemonSets
// sequentially in a specific namespace and waits until number of available DeamonSets is equal to number of desired scheduled Daemonsets.
func WatchAndWaitDaemonSets(client *rancher.Client, clusterID, namespace string, listOptions metav1.ListOptions) error {
	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	if err != nil {
		return err
	}
	adminDynamicClient, err := adminClient.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return err
	}
	adminDaemonSetResource := adminDynamicClient.Resource(daemonsets.DaemonSetGroupVersionResource).Namespace(namespace)

	daemonSets, err := adminDaemonSetResource.List(context.TODO(), listOptions)
	if err != nil {
		return err
	}

	var daemonSetList []appv1.DaemonSet

	for _, unstructuredDaemonSet := range daemonSets.Items {
		newDaemonSet := &appv1.DaemonSet{}
		err := scheme.Scheme.Convert(&unstructuredDaemonSet, newDaemonSet, unstructuredDaemonSet.GroupVersionKind())
		if err != nil {
			return err
		}

		daemonSetList = append(daemonSetList, *newDaemonSet)
	}

	for _, daemonSet := range daemonSetList {
		watchAppInterface, err := adminDaemonSetResource.Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  "metadata.name=" + daemonSet.Name,
			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
		})
		if err != nil {
			return err
		}

		wait.WatchWait(watchAppInterface, func(event watch.Event) (ready bool, err error) {
			daemonsetsUnstructured := event.Object.(*unstructured.Unstructured)
			daemonset := &appv1.DaemonSet{}

			err = scheme.Scheme.Convert(daemonsetsUnstructured, daemonset, daemonsetsUnstructured.GroupVersionKind())
			if err != nil {
				return false, err
			}

			if daemonset.Status.DesiredNumberScheduled == daemonset.Status.NumberAvailable {
				return true, nil
			}
			return false, nil
		})
	}

	return nil
}

// WatchAndWaitStatefulSets is a helper function that watches the StatefulSets
// sequentially in a specific namespace and waits until number of expected replicas is equal to number of ready replicas.
func WatchAndWaitStatefulSets(client *rancher.Client, clusterID, namespace string, listOptions metav1.ListOptions) error {
	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	if err != nil {
		return err
	}
	adminDynamicClient, err := adminClient.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return err
	}
	adminStatefulSetResource := adminDynamicClient.Resource(appv1.SchemeGroupVersion.WithResource("statefulsets")).Namespace(namespace)

	statefulSets, err := adminStatefulSetResource.List(context.TODO(), listOptions)
	if err != nil {
		return err
	}

	var statefulSetList []appv1.StatefulSet

	for _, unstructuredStatefulSet := range statefulSets.Items {
		newStatefulSet := &appv1.StatefulSet{}
		err := scheme.Scheme.Convert(&unstructuredStatefulSet, newStatefulSet, unstructuredStatefulSet.GroupVersionKind())
		if err != nil {
			return err
		}

		statefulSetList = append(statefulSetList, *newStatefulSet)
	}

	for _, statefulSet := range statefulSetList {
		watchAppInterface, err := adminStatefulSetResource.Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  "metadata.name=" + statefulSet.Name,
			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
		})
		if err != nil {
			return err
		}

		wait.WatchWait(watchAppInterface, func(event watch.Event) (ready bool, err error) {
			statefulSetsUnstructured := event.Object.(*unstructured.Unstructured)
			statefulSet := &appv1.StatefulSet{}

			err = scheme.Scheme.Convert(statefulSetsUnstructured, statefulSet, statefulSetsUnstructured.GroupVersionKind())
			if err != nil {
				return false, err
			}

			if *statefulSet.Spec.Replicas == statefulSet.Status.ReadyReplicas {
				return true, nil
			}
			return false, nil
		})
	}

	return nil
}

// CreateChartRepoFromGithub creates a ClusterRepo in a given client via github instead of helm
func CreateChartRepoFromGithub(client *steveV1.Client, githubURL, githubBranch, repoName string) error {
	repoObject := v1.ClusterRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name: repoName,
		},
		Spec: v1.RepoSpec{
			GitRepo:               githubURL,
			GitBranch:             githubBranch,
			InsecureSkipTLSverify: true,
		},
	}
	_, err := client.SteveType(repoType).Create(repoObject)
	if err != nil {
		return err
	}

	err = kwait.Poll(1*time.Second, 2*time.Minute, func() (done bool, err error) {
		res, err := client.SteveType(repoType).List(nil)
		if err != nil {
			return false, err
		}

		for _, repo := range res.Data {
			if repo.Name == repoName {
				if repo.State.Name == active {
					return true, nil
				}
			}
		}

		return false, nil
	})
	return err
}
