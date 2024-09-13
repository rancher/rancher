package cli

import (
	ranchercli "github.com/rancher/shepherd/clients/ranchercli"
	"github.com/sirupsen/logrus"
)

const (
	rancher    = "rancher"
	context    = "context"
	projects   = "projects"
	namespaces = "namespaces"
	catalog    = "catalog"
)

// SwitchContext will display the current context and switch to the default one.
func SwitchContext(client *ranchercli.Client) error {
	logrus.Infof("Listing the current context...")
	err := client.ExecuteCommand(rancher, context, "current")
	if err != nil {
		return err
	}

	logrus.Infof("Switching to the default context...")
	err = client.ExecuteCommand(rancher, context, "switch")
	if err != nil {
		return err
	}

	return nil
}

// CreateProjects will create and projects in the specified cluster.
func CreateProjects(client *ranchercli.Client, projectName, cluster string) error {
	logrus.Infof("Creating a project...")
	err := client.ExecuteCommand(rancher, projects, "create", "--cluster", cluster, projectName)
	if err != nil {
		return err
	}

	logrus.Infof("Validating the project exists...")
	err = client.Exists(rancher, projects, projectName)
	if err != nil {
		return err
	}

	return nil
}

// DeleteProjects will delete projects in the specified cluster.
func DeleteProjects(client *ranchercli.Client, projectName string) error {
	logrus.Infof("Deleting the project...")
	err := client.Delete(projects, projectName)
	if err != nil {
		return err
	}

	logrus.Infof("Validating the project is deleted...")
	err = client.ExecuteCommand(rancher, projects, "ls", "|", "grep", projectName)
	if err != nil {
		return err
	}

	return nil
}

// CreateNamespaces will create namespaces in the specified cluster.
func CreateNamespaces(client *ranchercli.Client, namespaceName, projectName string) error {
	logrus.Infof("Creating a namespace in default project...")
	err := client.Create(namespaces, namespaceName)
	if err != nil {
		return err
	}

	logrus.Infof("Validating the namespace exists...")
	err = client.Exists(rancher, namespaces, namespaceName)
	if err != nil {
		return err
	}

	logrus.Infof("Creating test project...")
	err = client.Create(projects, "--cluster", "local", projectName)
	if err != nil {
		return err
	}

	logrus.Infof("Validating the project exists...")
	err = client.Exists(rancher, projects, projectName)
	if err != nil {
		return err
	}

	return nil
}

// DeleteNamespaces will delete namespaces in the specified cluster.
func DeleteNamespaces(client *ranchercli.Client, namespaceName, projectName string) error {
	logrus.Infof("Deleting the namespace...")
	err := client.Delete(namespaces, namespaceName)
	if err != nil {
		return err
	}

	logrus.Infof("Deleting the project...")
	err = client.Delete(projects, projectName)
	if err != nil {
		return err
	}

	return nil
}
