package charts

import (
	"context"

	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher/catalog"
	"github.com/rancher/rancher/tests/framework/extensions/defaults"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// VerifyChartInstall verifies that the app from a chart was successfully deployed
func VerifyChartInstall(client *catalog.Client, chartNamespace, chartName string) error {
	watchAppInterface, err := client.Apps(chartNamespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + chartName,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return err
	}

	err = wait.WatchWait(watchAppInterface, func(event watch.Event) (ready bool, err error) {
		app := event.Object.(*v1.App)

		state := app.Status.Summary.State
		if state == string(v1.StatusDeployed) {
			return true, nil
		}
		return false, nil
	})
	return err
}
