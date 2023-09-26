package kubectl

import (
	"context"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/kubeconfig"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rancher/rancher/tests/framework/clients/dynamic"
	"github.com/rancher/rancher/tests/framework/extensions/unstructured"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	rbacv1 "k8s.io/api/rbac/v1"
	v1Unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/watch"
	k8Scheme "k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
)

const (
	// rancherShellSettingID is the setting ID that used to grab rancher/shell image
	rancherShellSettingID = "shell-image"
	Namespace             = "kube-system"
	JobName               = "kubectl"
)

var (
	importTimeout = int64(60 * 1)
	group         int64
	user          int64
)

// CreateJobAndRunKubectlCommands is a helper to create a job and run the kubectl commands in the pods of the Job.
// It then returns errors or nil from the job.
func CreateJobAndRunKubectlCommands(clusterID, jobname string, job *batchv1.Job, client *rancher.Client) error {
	job.ObjectMeta.Name = jobname
	job.Spec.Template.ObjectMeta.Name = jobname
	var restConfig *restclient.Config

	kubeConfig, err := kubeconfig.GetKubeconfig(client, clusterID)
	if err != nil {
		return err
	}

	restConfig, err = (*kubeConfig).ClientConfig()
	if err != nil {
		return err
	}
	restConfig.ContentConfig.NegotiatedSerializer = serializer.NewCodecFactory(k8Scheme.Scheme)

	ts := client.Session.NewSession()
	defer ts.Cleanup()

	downClient, err := dynamic.NewForConfig(ts, restConfig)
	if err != nil {
		return err
	}

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rancher-installer2",
		},
	}
	_, err = downClient.Resource(corev1.SchemeGroupVersion.WithResource("serviceaccounts")).Namespace(Namespace).Create(context.TODO(), unstructured.MustToUnstructured(sa), metav1.CreateOptions{})
	if err != nil {
		return err
	}

	rb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rancher-install-cluster-admin",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      sa.Name,
				Namespace: Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
	}
	_, err = downClient.Resource(rbacv1.SchemeGroupVersion.WithResource("clusterrolebindings")).Namespace("").Create(context.TODO(), unstructured.MustToUnstructured(rb), metav1.CreateOptions{})
	if err != nil {
		return err
	}

	job.Spec.Template.Spec.ServiceAccountName = sa.Name

	_, err = downClient.Resource(batchv1.SchemeGroupVersion.WithResource("jobs")).Namespace(Namespace).Create(context.TODO(), unstructured.MustToUnstructured(job), metav1.CreateOptions{})
	if err != nil {
		return err
	}

	jobWatch, err := downClient.Resource(batchv1.SchemeGroupVersion.WithResource("jobs")).Namespace(Namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  fields.OneTermEqualSelector("metadata.name", job.Name).String(),
		TimeoutSeconds: &importTimeout,
	})
	if err != nil {
		return err
	}

	err = wait.WatchWait(jobWatch, func(event watch.Event) (bool, error) {
		var wj batchv1.Job
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(event.Object.(*v1Unstructured.Unstructured).Object, &wj)
		return wj.Status.Succeeded == 1, nil
	})

	return err
}
