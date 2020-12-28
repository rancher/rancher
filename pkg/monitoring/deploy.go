package monitoring

import (
	"github.com/pkg/errors"
	projectv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func WithdrawApp(cattleAppClient projectv3.AppInterface, appLabels metav1.ListOptions) error {
	monitoringApps, err := cattleAppClient.List(appLabels)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}

		return errors.Wrapf(err, "failed to find App with %s", appLabels.String())
	}

	for _, app := range monitoringApps.Items {
		if app.DeletionTimestamp == nil {
			if err := cattleAppClient.DeleteNamespaced(app.Namespace, app.Name, &metav1.DeleteOptions{}); err != nil {
				return errors.Wrapf(err, "failed to remove App with %s", appLabels.String())
			}
		}
	}

	return nil
}
