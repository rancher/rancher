package daemonset

import (
	projectsapi "github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func VerifyCreateDaemonSet(client *rancher.Client, clusterID string) error {
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	logrus.Info("Creating new daemonset")
	createdDaemonset, err := CreateDaemonset(client, clusterID, namespace.Name, 1, "", "", false, false)
	if err != nil {
		return err
	}

	logrus.Infof("Waiting for daemonset %s to become active", createdDaemonset.Name)
	err = charts.WatchAndWaitDaemonSets(client, clusterID, namespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + createdDaemonset.Name,
	})

	return err
}
