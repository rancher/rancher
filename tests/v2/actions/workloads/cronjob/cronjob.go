package cronjob

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/api/scheme"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/workloads/cronjobs"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/wait"
	"github.com/rancher/shepherd/pkg/wrangler"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	nginxImageName = "public.ecr.aws/docker/library/nginx"
)

// CreateCronJob is a helper to create a cronjob in a namespace
func CreateCronJob(client *rancher.Client, clusterID, namespaceName, schedule string, podTemplate corev1.PodTemplateSpec, watchCronJob bool) (*batchv1.CronJob, error) {
	var ctx *wrangler.Context
	var err error

	if clusterID != "local" {
		ctx, err = client.WranglerContext.DownStreamClusterWranglerContext(clusterID)
		if err != nil {
			return nil, fmt.Errorf("failed to get downstream context: %w", err)
		}
	} else {
		ctx = client.WranglerContext
	}

	cronJobTemplate := NewCronJobTemplate(namespaceName, schedule, podTemplate)
	createdCronJob, err := ctx.Batch.CronJob().Create(cronJobTemplate)

	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	if watchCronJob {
		err = WatchAndWaitCronJob(client, clusterID, namespaceName, createdCronJob)
		if err != nil {
			return nil, err
		}
	}

	return createdCronJob, nil
}

// WatchAndWaitCronJob is a helper to watch and wait for cronjob to be active
func WatchAndWaitCronJob(client *rancher.Client, clusterID, namespaceName string, cronJob *batchv1.CronJob) error {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return err
	}

	cronJobResource := dynamicClient.Resource(cronjobs.CronJobGroupVersionResource).Namespace(namespaceName)

	watchCronJobInterface, err := cronJobResource.Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + cronJob.Name,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return err
	}

	return wait.WatchWait(watchCronJobInterface, func(event watch.Event) (bool, error) {
		cronJobUnstructured, ok := event.Object.(*unstructured.Unstructured)
		if !ok {
			return false, fmt.Errorf("failed to cast to unstructured object")
		}

		cronJob := &batchv1.CronJob{}
		err := scheme.Scheme.Convert(cronJobUnstructured, cronJob, cronJobUnstructured.GroupVersionKind())
		if err != nil {
			return false, err
		}

		if len(cronJob.Status.Active) > 0 {
			return true, nil
		}

		return false, nil
	})
}
