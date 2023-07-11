package clusters

import (
	"context"
	"fmt"
	"time"

	"github.com/rancher/norman/types"
	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/dynamic"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	ext_unstructured "github.com/rancher/rancher/tests/framework/extensions/unstructured"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	apisV3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
)

const (
	// rancherShellSettingID is the setting ID that used to grab rancher/shell image
	rancherShellSettingID = "shell-image"
	// kubeConfig is a basic kubeconfig that uses the pod's service account
	kubeConfig = `
apiVersion: v1
kind: Config
clusters:
- name: cluster
  cluster:
    certificate-authority: /run/secrets/kubernetes.io/serviceaccount/ca.crt
    server: https://kubernetes.default
contexts:
- name: default
  context:
    cluster: cluster
    user: user
current-context: default
users:
- name: user
  user:
    tokenFile: /run/secrets/kubernetes.io/serviceaccount/token
`
)

var (
	importTimeout = int64(60 * 20)
)

// ImportCluster creates a job using the given rest config that applies the import yaml from the given management cluster.
func ImportCluster(client *rancher.Client, cluster *apisV1.Cluster, rest *rest.Config) error {
	// create a sub session to clean up after we apply the manifest
	ts := client.Session.NewSession()
	defer ts.Cleanup()

	var token management.ClusterRegistrationToken
	err := kwait.Poll(500*time.Millisecond, 2*time.Minute, func() (done bool, err error) {
		res, err := client.Management.ClusterRegistrationToken.List(&types.ListOpts{Filters: map[string]interface{}{
			"clusterId": cluster.Status.ClusterName,
		}})
		if err != nil {
			return false, err
		}

		if len(res.Data) > 0 && res.Data[0].ManifestURL != "" {
			token = res.Data[0]
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return err
	}

	downClient, err := dynamic.NewForConfig(ts, rest)
	if err != nil {
		return err
	}

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rancher-installer",
		},
	}
	_, err = downClient.Resource(corev1.SchemeGroupVersion.WithResource("serviceaccounts")).Namespace("kube-system").Create(context.TODO(), ext_unstructured.MustToUnstructured(sa), metav1.CreateOptions{})
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
				Namespace: "kube-system",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
	}
	_, err = downClient.Resource(rbacv1.SchemeGroupVersion.WithResource("clusterrolebindings")).Namespace("").Create(context.TODO(), ext_unstructured.MustToUnstructured(rb), metav1.CreateOptions{})
	if err != nil {
		return err
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubeconfig",
		},
		Data: map[string]string{
			"config": kubeConfig,
		},
	}
	_, err = downClient.Resource(corev1.SchemeGroupVersion.WithResource("configmaps")).Namespace("kube-system").Create(context.TODO(), ext_unstructured.MustToUnstructured(cm), metav1.CreateOptions{})
	if err != nil {
		return err
	}

	imageSetting, err := client.Management.Setting.ByID(rancherShellSettingID)
	if err != nil {
		return err
	}

	var user int64
	var group int64
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rancher-import",
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: "rancher-import",
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      "Never",
					ServiceAccountName: sa.Name,
					Containers: []corev1.Container{
						{
							Name:    "kubectl",
							Image:   imageSetting.Value,
							Command: []string{"/bin/sh", "-c"},
							Args: []string{
								fmt.Sprintf("wget -qO- --tries=10 --no-check-certificate %s | kubectl apply -f - ;", token.ManifestURL),
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:  &user,
								RunAsGroup: &group,
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "config", MountPath: "/root/.kube/"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: cm.Name},
								},
							},
						},
					},
				},
			},
		},
	}
	_, err = downClient.Resource(batchv1.SchemeGroupVersion.WithResource("jobs")).Namespace("kube-system").Create(context.TODO(), ext_unstructured.MustToUnstructured(job), metav1.CreateOptions{})
	if err != nil {
		return err
	}

	jobWatch, err := downClient.Resource(batchv1.SchemeGroupVersion.WithResource("jobs")).Namespace("kube-system").Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  fields.OneTermEqualSelector("metadata.name", job.Name).String(),
		TimeoutSeconds: &importTimeout,
	})
	if err != nil {
		return err
	}

	err = wait.WatchWait(jobWatch, func(event watch.Event) (bool, error) {
		var wj batchv1.Job
		_ = runtime.DefaultUnstructuredConverter.FromUnstructured(event.Object.(*unstructured.Unstructured).Object, &wj)
		return wj.Status.Succeeded == 1, nil
	})
	if err != nil {
		return err
	}

	return nil
}

// IsClusterImported is a function to get a boolean value about if the cluster is imported or not.
// For custom and imported clusters the node driver value is different than "imported".
func IsClusterImported(client *rancher.Client, clusterID string) (isImported bool, err error) {
	cluster, err := client.Management.Cluster.ByID(clusterID)
	if err != nil {
		return
	}

	isImported = cluster.Driver == apisV3.ClusterDriverImported // For imported K3s and RKE2, driver != "imported", for custom and provisioning drive ones = "imported"

	return
}

// IsImportedClusterReady is basic check function that would be used for the wait.WatchWait func in pkg/wait.
// This functions just waits until an imported cluster becomes ready.
func IsImportedClusterReady(event watch.Event) (ready bool, err error) {
	cluster := event.Object.(*apisV1.Cluster)
	var readyCondition bool
	ready = cluster.Status.Ready
	agentDeployed := cluster.Status.AgentDeployed
	var numSuccess int
	for _, condition := range cluster.Status.Conditions {
		if condition.Type == "Ready" && condition.Status == corev1.ConditionTrue {
			numSuccess += 1
		}
		if condition.Type == "SystemAccountCreated" && condition.Status == corev1.ConditionTrue {
			numSuccess += 1
		}
		if condition.Type == "ServiceAccountSecretsMigrated" && condition.Status == corev1.ConditionTrue {
			numSuccess += 1
		}
	}

	if numSuccess == 3 {
		readyCondition = true
	}

	return ready && readyCondition && agentDeployed, nil
}
