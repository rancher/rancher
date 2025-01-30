package job

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/api/scheme"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/workloads/jobs"
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

// CreateJob is a helper to create a job in a namespace
func CreateJob(client *rancher.Client, clusterID, namespaceName string, podTemplate corev1.PodTemplateSpec, watchJob bool) (*batchv1.Job, error) {
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

	jobTemplate := NewJobTemplate(namespaceName, podTemplate)
	createdJob, err := ctx.Batch.Job().Create(&jobTemplate)

	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	if watchJob {
		err = WatchAndWaitJob(client, clusterID, namespaceName, createdJob)
		if err != nil {
			return nil, err
		}
	}

	return createdJob, nil
}

// WatchAndWaitJob is a helper to watch and wait for job to be active
func WatchAndWaitJob(client *rancher.Client, clusterID, namespaceName string, job *batchv1.Job) error {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return err
	}

	jobResource := dynamicClient.Resource(jobs.JobGroupVersionResource).Namespace(namespaceName)

	watchJobInterface, err := jobResource.Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + job.Name,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return err
	}

	return wait.WatchWait(watchJobInterface, func(event watch.Event) (bool, error) {
		jobUnstructured, ok := event.Object.(*unstructured.Unstructured)
		if !ok {
			return false, fmt.Errorf("failed to cast to unstructured object")
		}

		job := &batchv1.Job{}
		err := scheme.Scheme.Convert(jobUnstructured, job, jobUnstructured.GroupVersionKind())
		if err != nil {
			return false, err
		}

		if job.Status.Active > 0 {
			return true, nil
		}

		return false, nil
	})
}
