package connectivity

import (
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/kubectl"
	"github.com/rancher/shepherd/extensions/workloads"
	"github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

const (
	pingPodProjectName = "ping-project"
	containerName      = "test1"
	containerImage     = "ranchertest/mytestcontainer"
)

type resourceNames struct {
	core   map[string]string
	random map[string]string
}

// newNames returns a new resourceNames struct
// it creates a random names with random suffix for each resource by using core and coreWithSuffix names
func newNames() *resourceNames {
	const (
		projectName             = "upgrade-wl-project"
		namespaceName           = "namespace"
		deploymentName          = "deployment"
		daemonsetName           = "daemonset"
		secretName              = "secret"
		serviceName             = "service"
		ingressName             = "ingress"
		defaultRandStringLength = 3
	)

	names := &resourceNames{
		core: map[string]string{
			"projectName":    projectName,
			"namespaceName":  namespaceName,
			"deploymentName": deploymentName,
			"daemonsetName":  daemonsetName,
			"secretName":     secretName,
			"serviceName":    serviceName,
			"ingressName":    ingressName,
		},
	}

	names.random = map[string]string{}
	for k, v := range names.core {
		names.random[k] = v + "-" + namegenerator.RandStringLower(defaultRandStringLength)
	}

	return names
}

// newPodTemplateWithTestContainer is a private constructor that returns pod template spec for workload creations
func newPodTemplateWithTestContainer() corev1.PodTemplateSpec {
	testContainer := newTestContainerMinimal()
	containers := []corev1.Container{testContainer}
	return workloads.NewPodTemplate(containers, nil, []corev1.LocalObjectReference{}, nil, nil)
}

// newTestContainerMinimal is a private constructor that returns container for minimal workload creations
func newTestContainerMinimal() corev1.Container {
	pullPolicy := corev1.PullAlways
	return workloads.NewContainer(containerName, containerImage, pullPolicy, nil, nil, nil, nil, nil)
}

// curlCommand is a helper to run a curl command on an SSH shell node
func curlCommand(client *rancher.Client, clusterID string, url string) (string, error) {
	logrus.Infof("Executing the kubectl command curl %s on the node", url)
	execCmd := []string{"curl", url}
	log, err := kubectl.Command(client, nil, clusterID, execCmd, "")
	if err != nil {
		return "", err
	}
	return log, nil
}
