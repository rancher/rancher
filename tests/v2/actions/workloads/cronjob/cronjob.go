package cronjob

import (
	"context"

	"github.com/rancher/rancher/pkg/api/scheme"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/shepherd/clients/rancher"
	unstruc "github.com/rancher/shepherd/extensions/unstructured"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/wait"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	nginxImageName = "public.ecr.aws/docker/library/nginx"
)

var CronJobGroupVersionResource = schema.GroupVersionResource{
	Group:    "batch",
	Version:  "v1",
	Resource: "cronjobs",
}

// CreateCronjob is a helper to create a cronjob
func CreateCronjob(client *rancher.Client, clusterID, namespaceName string, schedule string, template corev1.PodTemplateSpec) (*v1.CronJob, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return nil, err
	}

	cronjobName := namegen.AppendRandomString("testcronjob")
	var limit int32 = 10

	template.Spec.RestartPolicy = corev1.RestartPolicyNever
	cronJobTemplate := &v1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cronjobName,
			Namespace: namespaceName,
		},
		Spec: v1.CronJobSpec{
			JobTemplate: v1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespaceName,
				},
				Spec: v1.JobSpec{
					Template: template,
				},
			},
			Schedule:                   schedule,
			ConcurrencyPolicy:          v1.ForbidConcurrent,
			FailedJobsHistoryLimit:     &limit,
			SuccessfulJobsHistoryLimit: &limit,
		},
	}

	cronJobResource := dynamicClient.Resource(CronJobGroupVersionResource).Namespace(namespaceName)

	unstructuredResp, err := cronJobResource.Create(context.TODO(), unstruc.MustToUnstructured(cronJobTemplate), metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	newCronjob := &v1.CronJob{}
	err = scheme.Scheme.Convert(unstructuredResp, newCronjob, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	return newCronjob, err
}

// WatchAndWaitCronjob is a helper to watch and wait cronjob
func WatchAndWaitCronjob(client *rancher.Client, clusterID, namespaceName string, cronJobTemplate *v1.CronJob) error {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return err
	}

	cronJobResource := dynamicClient.Resource(CronJobGroupVersionResource).Namespace(namespaceName)

	watchCronjobInterface, err := cronJobResource.Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + cronJobTemplate.Name,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return err
	}

	return wait.WatchWait(watchCronjobInterface, func(event watch.Event) (ready bool, err error) {
		cronjobUnstructured := event.Object.(*unstructured.Unstructured)
		cronjob := &v1.CronJob{}

		err = scheme.Scheme.Convert(cronjobUnstructured, cronjob, cronjobUnstructured.GroupVersionKind())
		if err != nil {
			return false, err
		}

		if len(cronjob.Status.Active) > 0 {
			return true, nil
		}
		return false, nil
	})
}
