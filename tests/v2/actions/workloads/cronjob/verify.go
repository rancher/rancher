package cronjob

import (
	projectsapi "github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/workloads"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

func VerifyCreateCronjob(client *rancher.Client, clusterID string) error {
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	containerName := namegen.AppendRandomString("test-container")
	pullPolicy := corev1.PullAlways

	containerTemplate := workloads.NewContainer(
		containerName,
		nginxImageName,
		pullPolicy,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil,
		nil,
		nil,
	)

	podTemplate := workloads.NewPodTemplate(
		[]corev1.Container{containerTemplate},
		[]corev1.Volume{},
		[]corev1.LocalObjectReference{},
		nil,
		nil,
	)

	cronJobTemplate, err := CreateCronjob(client, clusterID, namespace.Name, "*/1 * * * *", podTemplate)
	if err != nil {
		return err
	}

	logrus.Infof("Creating new cronjob %s", cronJobTemplate.Name)
	err = WatchAndWaitCronjob(client, clusterID, namespace.Name, cronJobTemplate)

	return err
}
