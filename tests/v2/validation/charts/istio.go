package charts

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/rancher/rancher/tests/v2/actions/charts"
	"github.com/rancher/rancher/tests/v2/actions/workloads"
	"github.com/rancher/shepherd/clients/rancher"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/ingresses"
	appv1 "k8s.io/api/apps/v1"
	kubewait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	// Project that example app and charts are installed in
	exampleAppProjectName = "demo-project"
	// Namespace that example app objects are installed in
	exampleAppNamespaceName = "demo-namespace"

	// Example app port and path to be checked
	exampleAppPort            = "31380"
	exampleAppProductPagePath = "productpage"

	// Example app different review bodies to be checked
	firstReviewBodyPart  = `<small>Reviewer1</small></blockquote>`
	secondReviewBodyPart = `<fontcolor="black"><!--fullstars:-->`
	thirdReviewBodyPart  = `<fontcolor="red"><!--fullstars:-->`
)

var (
	// Rancher istio chart kiali path
	kialiPath = "api/v1/namespaces/istio-system/services/http:kiali:20001/proxy/console/"
	// Rancher istio chart tracing path
	tracingPath = "api/v1/namespaces/istio-system/services/http:tracing:16686/proxy/jaeger/search"
)

// chartInstallOptions is a private struct that has istio and monitoring charts install options
type chartInstallOptions struct {
	monitoring *charts.InstallOptions
	istio      *charts.InstallOptions
}

// chartFeatureOptions is a private struct that has istio and monitoring charts feature options
type chartFeatureOptions struct {
	monitoring *charts.RancherMonitoringOpts
	istio      *charts.RancherIstioOpts
}

// getChartCaseEndpointUntilBodyHas is a private helper function
// that awaits the body of the response until the desired string is found
func getChartCaseEndpointUntilBodyHas(client *rancher.Client, host, path, bodyPart string) (found bool, err error) {
	trimAllSpaces := func(str string) string {
		return strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return -1
			}
			return r
		}, str)
	}

	err = kubewait.PollUntilContextTimeout(context.TODO(), 500*time.Millisecond, 2*time.Minute, true, func(context.Context) (ongoing bool, err error) {
		bodyString, err := ingresses.GetExternalIngressResponse(client, host, path, false)
		if err != nil {
			return ongoing, err
		}

		trimmedBody := trimAllSpaces(bodyString)
		if strings.Contains(trimmedBody, bodyPart) {
			found = true
			return !ongoing, nil
		}

		return
	})
	if err != nil {
		return false, err
	}

	return
}

// listIstioDeployments is a private helper function
// that returns the deployment specs if deployments have "operator.istio.io/version" label
func listIstioDeployments(steveclient *v1.Client) (deploymentSpecList []*appv1.DeploymentSpec, err error) {
	deploymentList, err := steveclient.SteveType(workloads.DeploymentSteveType).List(nil)
	if err != nil {
		return
	}

	for _, deployment := range deploymentList.Data {
		_, ok := deployment.ObjectMeta.Labels["operator.istio.io/version"]

		if ok {
			deploymentSpec := &appv1.DeploymentSpec{}
			err := v1.ConvertToK8sType(deployment.Spec, deploymentSpec)
			if err != nil {
				return deploymentSpecList, err
			}

			deploymentSpecList = append(deploymentSpecList, deploymentSpec)
		}
	}

	return deploymentSpecList, nil
}
