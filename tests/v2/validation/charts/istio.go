package charts

import (
	"strings"
	"time"
	"unicode"

	appv1 "k8s.io/api/apps/v1"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/charts"
	"github.com/rancher/rancher/tests/framework/extensions/workloads"
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
	kialiPath = "api/v1/namespaces/istio-system/services/http:kiali:20001/proxy/kiali/"
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
func getChartCaseEndpointUntilBodyHas(client *rancher.Client, host, path, bodyPart string) (bool, error) {
	trimAllSpaces := func(str string) string {
		return strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return -1
			}
			return r
		}, str)
	}

	timeout := 30 * time.Second
	for start := time.Now(); time.Since(start) < timeout; {
		result, err := charts.GetChartCaseEndpoint(client, host, path, false)
		if err != nil {
			return false, err
		}

		trimmedBody := trimAllSpaces(result.Body)
		if strings.Contains(trimmedBody, bodyPart) {
			return true, nil
		}
	}

	return false, nil
}

// listIstioDeployments is a private helper function
// that returns the deployment specs if deployments have "operator.istio.io/version" label
func listIstioDeployments(steveclient *v1.Client) (deploymentSpecList []*appv1.DeploymentSpec, err error) {
	deploymentList, err := steveclient.SteveType(workloads.DeploymentSteveType).List(&types.ListOpts{})
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
