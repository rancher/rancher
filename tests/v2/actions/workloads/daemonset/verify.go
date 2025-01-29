package daemonset

import (
	projectsapi "github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/sirupsen/logrus"
)

func VerifyCreateDaemonSet(client *rancher.Client, clusterID string) error {
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	logrus.Info("Creating new daemonset and waiting for daemonset to come up active")
	_, err = CreateDaemonset(client, clusterID, namespace.Name, 1, "", "", false, false, true)
	if err != nil {
		return err
	}
	return err
}
