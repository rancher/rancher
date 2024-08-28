package jobs

import (
	"context"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/unstructured"
	"github.com/rancher/shepherd/pkg/api/scheme"
	"github.com/rancher/shepherd/pkg/wait"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// CreateJob is a helper function that uses the dynamic client to create a batch job on a namespace for a specific cluster.
// It registers a delete fuction a wait.WatchWait to ensure the job is deleted cleanly.
func CreateJob(client *rancher.Client, clusterName, jobName, namespace string, template corev1.PodTemplateSpec) (*batchv1.Job, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterName)
	if err != nil {
		return nil, err
	}

	template.Spec.RestartPolicy = corev1.RestartPolicyNever
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
		},
		Spec: batchv1.JobSpec{
			Template: template,
		},
	}

	jobResource := dynamicClient.Resource(JobGroupVersionResource).Namespace(namespace)

	unstructuredResp, err := jobResource.Create(context.TODO(), unstructured.MustToUnstructured(job), metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	client.Session.RegisterCleanupFunc(func() error {
		err := jobResource.Delete(context.TODO(), unstructuredResp.GetName(), metav1.DeleteOptions{})
		if errors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}

		watchInterface, err := jobResource.Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  "metadata.name=" + unstructuredResp.GetName(),
			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
		})

		if err != nil {
			return err
		}

		return wait.WatchWait(watchInterface, func(event watch.Event) (ready bool, err error) {
			if event.Type == watch.Deleted {
				return true, nil
			}
			return false, nil
		})
	})

	newJob := &batchv1.Job{}
	err = scheme.Scheme.Convert(unstructuredResp, newJob, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	return newJob, nil
}
