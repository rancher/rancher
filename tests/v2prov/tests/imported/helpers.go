package imported

import (
	"fmt"
	"testing"

	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1/snapshotutil"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func handleError(t *testing.T, clients *clients.Clients, name string, err error) {
	if err != nil {
		objs := map[string]any{}

		c, newErr := clients.Mgmt.Cluster().Get(name, metav1.GetOptions{})
		if newErr != nil {
			logrus.Error(newErr)
		} else {
			objs["mgmtCluster"] = c
			nodes, newErr := clients.Mgmt.Node().List(c.Name, metav1.ListOptions{})
			if newErr != nil {
				logrus.Error(newErr)
			} else {
				objs["mgmtNodes"] = nodes
			}

			beacon, newErr := clients.Plan.Beacon().Get(c.Name, c.Name, metav1.GetOptions{})
			if newErr != nil {
				logrus.Error(newErr)
			} else {
				objs["beacon"] = beacon
			}

			secrets, newErr := clients.Core.Secret().List(c.Name, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", capr.ClusterNameLabel, c.Name),
				FieldSelector: fmt.Sprintf("type=%s", capr.SecretTypeMachinePlan),
			})
			if newErr != nil {
				logrus.Error(newErr)
			} else {
				objs["machinePlans"] = secrets
			}

			creates, newErr := clients.Operation.ETCDSnapshotSave().List(c.Name, metav1.ListOptions{})
			if newErr != nil {
				logrus.Error(newErr)
			} else {
				objs["ETCDSnapshotSave"] = creates
			}
		}

		features, newErr := clients.Mgmt.Feature().List(metav1.ListOptions{})
		if newErr != nil {
			logrus.Error(newErr)
		} else {
			objs["features"] = features
		}

		settings, newErr := clients.Mgmt.Setting().List(metav1.ListOptions{})
		if newErr != nil {
			logrus.Error(newErr)
		} else {
			objs["settings"] = settings
		}

		data, newErr := snapshotutil.CompressInterface(objs)
		if newErr != nil {
			logrus.Error(newErr)
		}
		//nolint:revive
		err = fmt.Errorf("cluster %s operation wait failed on: %w\ncluster %s test data bundle: \n%s\n", name, err, name, data)
		t.Fatal(err)
	}
}
