package cronjob

import (
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewCronJobTemplate is a constructor that creates the template for cronjob
func NewCronJobTemplate(namespaceName, schedule string, podTemplate corev1.PodTemplateSpec) *batchv1.CronJob {
	cronJobName := namegen.AppendRandomString("testcronjob")
	var limit int32 = 10

	podTemplate.Spec.RestartPolicy = corev1.RestartPolicyNever

	cronJobTemplate := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cronJobName,
			Namespace: namespaceName,
		},
		Spec: batchv1.CronJobSpec{
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespaceName,
				},
				Spec: batchv1.JobSpec{
					Template: podTemplate,
				},
			},
			Schedule:                   schedule,
			ConcurrencyPolicy:          batchv1.ForbidConcurrent,
			FailedJobsHistoryLimit:     &limit,
			SuccessfulJobsHistoryLimit: &limit,
		},
	}

	return cronJobTemplate
}
