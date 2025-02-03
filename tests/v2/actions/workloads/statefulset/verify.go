package statefulset

import (
	projectsapi "github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/workloads"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func VerifyCreateStatefulset(client *rancher.Client, clusterID string) error {
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

	logrus.Infof("Creating new statefulset %s", containerName)
	statefulsetTemplate, err := CreateStatefulset(client, clusterID, namespace.Name, podTemplate, 1)
	if err != nil {
		return err
	}

	err = charts.WatchAndWaitStatefulSets(client, clusterID, namespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + statefulsetTemplate.Name,
	})

	return err
}
