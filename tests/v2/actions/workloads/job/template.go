package job

import (
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewJobTemplate is a constructor that creates the template for jobs
func NewJobTemplate(namespaceName string, podTemplate corev1.PodTemplateSpec) batchv1.Job {
	jobName := namegen.AppendRandomString("testjob")

	podTemplate.Spec.RestartPolicy = corev1.RestartPolicyOnFailure

	return batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespaceName,
		},
		Spec: batchv1.JobSpec{
			Template: podTemplate,
		},
	}
}
