package k8s

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ApplyK8sSystemJob(jobYaml, kubeConfigPath string) error {
	job := v1.Job{}
	if err := decodeYamlResource(&job, jobYaml); err != nil {
		return err
	}
	if job.Namespace == metav1.NamespaceNone {
		job.Namespace = metav1.NamespaceSystem
	}
	k8sClient, err := NewClient(kubeConfigPath)
	if err != nil {
		return err
	}
	if _, err = k8sClient.BatchV1().Jobs(job.Namespace).Create(&job); err != nil {
		if apierrors.IsAlreadyExists(err) {
			logrus.Debugf("[k8s] Job %s already exists..", job.Name)
			return nil
		}
		return err
	}
	logrus.Debugf("[k8s] waiting for job %s to complete..", job.Name)
	return retryTo(ensureJobCompleted, k8sClient, job, DefaultRetries, DefaultSleepSeconds)
}

func ensureJobCompleted(k8sClient *kubernetes.Clientset, j interface{}) error {
	job := j.(v1.Job)
	existingJob := &v1.Job{}
	existingJob, err := k8sClient.BatchV1().Jobs(job.Namespace).Get(job.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Failed to update job status: %v", err)

	}
	for _, condition := range existingJob.Status.Conditions {
		if condition.Type == v1.JobComplete && condition.Status == corev1.ConditionTrue {
			logrus.Debugf("[k8s] Job %s completed successfully..", job.Name)
			return nil
		}
	}
	return fmt.Errorf("Failed to get job complete status: %v", err)
}
