package usercontrollers

import (
	"context"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/management/imported"
	"github.com/rancher/rancher/pkg/controllers/managementagent/nslabels"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/helm"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	batchV1 "k8s.io/api/batch/v1"
	coreV1 "k8s.io/api/core/v1"
	rbacV1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const (
	WebhookClusterRoleBindingName = "rancher-webhook"
	WebhookConfigurationName      = "rancher.cattle.io"
)

var (
	// There is a mirror list in pkg/agent/clean/clean.go. If these are getting
	// updated consider if that update needs to apply to the user cluster as well

	// List of namespace labels that will be removed
	nsLabels = []string{
		nslabels.ProjectIDFieldLabel,
	}

	// List of namespace annotations that will be removed
	nsAnnotations = []string{
		"cattle.io/status",
		"field.cattle.io/creatorId",
		"field.cattle.io/resourceQuotaTemplateId",
		"lifecycle.cattle.io/create.namespace-auth",
		nslabels.ProjectIDFieldLabel,
		helm.AppIDsLabel,
	}
)

/*
RegisterEarly registers ClusterLifecycleCleanup controller which is responsible for stopping rancher agent in user cluster,
and de-registering k8s controllers, on cluster.remove
*/
func RegisterEarly(ctx context.Context, management *config.ManagementContext, manager *clustermanager.Manager) {
	lifecycle := &ClusterLifecycleCleanup{
		Manager: manager,
		ctx:     ctx,
	}

	clusterClient := management.Management.Clusters("")
	clusterClient.AddLifecycle(ctx, "cluster-agent-controller-cleanup", lifecycle)
}

type ClusterLifecycleCleanup struct {
	Manager *clustermanager.Manager
	ctx     context.Context
}

func (c *ClusterLifecycleCleanup) Create(obj *v3.Cluster) (runtime.Object, error) {
	return nil, nil
}

func (c *ClusterLifecycleCleanup) Remove(obj *v3.Cluster) (runtime.Object, error) {
	if obj == nil {
		return obj, nil
	}

	// Try to clean up an imported cluster 3 times, then move on.
	backoff := wait.Backoff{
		Duration: 3 * time.Second,
		Factor:   1,
		Steps:    3,
	}

	if err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		var err error
		if obj.Name == "local" && obj.Spec.Internal {
			err = c.cleanupLocalCluster(obj)
		} else if obj.Status.Driver == v32.ClusterDriverK3s ||
			obj.Status.Driver == v32.ClusterDriverK3os ||
			obj.Status.Driver == v32.ClusterDriverRke2 ||
			obj.Status.Driver == v32.ClusterDriverRancherD ||
			(obj.Status.Driver == v32.ClusterDriverImported && !imported.IsAdministratedByProvisioningCluster(obj)) ||
			(obj.Status.AKSStatus.UpstreamSpec != nil && obj.Status.AKSStatus.UpstreamSpec.Imported) ||
			(obj.Status.EKSStatus.UpstreamSpec != nil && obj.Status.EKSStatus.UpstreamSpec.Imported) ||
			(obj.Status.GKEStatus.UpstreamSpec != nil && obj.Status.GKEStatus.UpstreamSpec.Imported) {
			err = c.cleanupImportedCluster(obj)
		}
		if err != nil {
			logrus.Infof("[cluster-cleanup] error cleaning up cluster [%s]: %v", obj.Name, err)
		}
		return err == nil, nil
	}); err != nil {
		logrus.Warnf("[cluster-cleanup] could not clean imported cluster [%s], moving on with removing cluster: %v", obj.Name, err)
	}

	c.Manager.Stop(obj)
	return nil, nil
}

func (c *ClusterLifecycleCleanup) cleanupLocalCluster(obj *v3.Cluster) error {
	userContext, err := c.Manager.UserContextFromCluster(obj)
	if err != nil {
		return err
	}
	if userContext == nil {
		logrus.Debugf("could not get context for local cluster, skipping cleanup")
		return nil
	}

	err = cleanupNamespaces(userContext.K8sClient)
	if err != nil {
		return err
	}

	propagationBackground := metav1.DeletePropagationBackground
	deleteOptions := &metav1.DeleteOptions{
		PropagationPolicy: &propagationBackground,
	}

	err = userContext.Apps.Deployments("cattle-system").Delete("cattle-cluster-agent", deleteOptions)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	err = userContext.Apps.DaemonSets("cattle-system").Delete("cattle-node-agent", deleteOptions)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (c *ClusterLifecycleCleanup) Updated(obj *v3.Cluster) (runtime.Object, error) {
	return nil, nil
}

func (c *ClusterLifecycleCleanup) cleanupImportedCluster(cluster *v3.Cluster) error {
	userContext, err := c.Manager.UserContextFromCluster(cluster)
	if err != nil {
		return err
	}
	if userContext == nil {
		logrus.Debugf("could not get context for imported cluster, skipping cleanup")
		return nil
	}

	role, err := c.createCleanupClusterRole(userContext)
	if err != nil {
		return err
	}

	sa, err := c.createCleanupServiceAccount(userContext)
	if err != nil {
		return err
	}

	crb, err := c.createCleanupClusterRoleBinding(userContext, role.Name, sa.Name)
	if err != nil {
		return err
	}

	job, err := c.createCleanupJob(userContext, sa.Name)
	if err != nil {
		return err
	}

	or := []metav1.OwnerReference{
		metav1.OwnerReference{
			APIVersion: "batch/v1",
			Kind:       "Job",
			Name:       job.Name,
			UID:        job.UID,
		},
	}

	// These resouces need the ownerReference added so they get cleaned up after
	// the job deletes itself.

	err = c.updateClusterRoleOwner(userContext, role, or)
	if err != nil {
		return err
	}

	err = c.updateServiceAccountOwner(userContext, sa, or)
	if err != nil {
		return err
	}

	err = c.updateClusterRoleBindingOwner(userContext, crb, or)
	if err != nil {
		return err
	}

	return nil
}

func (c *ClusterLifecycleCleanup) createCleanupClusterRole(userContext *config.UserContext) (*rbacV1.ClusterRole, error) {
	meta := metav1.ObjectMeta{
		GenerateName: "cattle-cleanup-",
	}

	rules := []rbacV1.PolicyRule{
		// This is needed to check for cattle-system, remove finalizers and delete
		{
			Verbs:     []string{"list", "get", "update", "delete"},
			APIGroups: []string{""},
			Resources: []string{"namespaces"},
		},
		{
			Verbs:     []string{"list", "get", "delete"},
			APIGroups: []string{"rbac.authorization.k8s.io"},
			Resources: []string{"roles", "rolebindings", "clusterroles", "clusterrolebindings"},
		},
		// The job is going to delete itself after running to trigger ownerReference
		// cleanup of the clusterRole, serviceAccount and clusterRoleBinding
		{
			Verbs:     []string{"list", "get", "delete"},
			APIGroups: []string{"batch"},
			Resources: []string{"jobs"},
		},
		// The job checks for the presence of the rancher service first
		{
			Verbs:         []string{"get"},
			APIGroups:     []string{""},
			Resources:     []string{"services"},
			ResourceNames: []string{"rancher"},
		},
		{
			Verbs:         []string{"delete"},
			APIGroups:     []string{"admissionregistration.k8s.io"},
			Resources:     []string{"validatingwebhookconfigurations", "mutatingwebhookconfigurations"},
			ResourceNames: []string{WebhookConfigurationName},
		},
	}
	clusterRole := rbacV1.ClusterRole{
		ObjectMeta: meta,
		Rules:      rules,
	}
	return userContext.K8sClient.RbacV1().ClusterRoles().Create(context.TODO(), &clusterRole, metav1.CreateOptions{})
}

func (c *ClusterLifecycleCleanup) createCleanupServiceAccount(userContext *config.UserContext) (*coreV1.ServiceAccount, error) {
	meta := metav1.ObjectMeta{
		GenerateName: "cattle-cleanup-",
		Namespace:    "default",
	}
	serviceAccount := coreV1.ServiceAccount{
		ObjectMeta: meta,
	}
	return userContext.K8sClient.CoreV1().ServiceAccounts("default").Create(context.TODO(), &serviceAccount, metav1.CreateOptions{})
}

func (c *ClusterLifecycleCleanup) createCleanupClusterRoleBinding(
	userContext *config.UserContext,
	role, sa string,
) (*rbacV1.ClusterRoleBinding, error) {
	meta := metav1.ObjectMeta{
		GenerateName: "cattle-cleanup-",
		Namespace:    "default",
	}
	clusterRoleBinding := rbacV1.ClusterRoleBinding{
		ObjectMeta: meta,
		Subjects: []rbacV1.Subject{
			rbacV1.Subject{
				Kind:      "ServiceAccount",
				Name:      sa,
				Namespace: "default",
			},
		},
		RoleRef: rbacV1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     role,
		},
	}
	return userContext.K8sClient.RbacV1().ClusterRoleBindings().Create(context.TODO(), &clusterRoleBinding, metav1.CreateOptions{})
}

func (c *ClusterLifecycleCleanup) createCleanupJob(userContext *config.UserContext, sa string) (*batchV1.Job, error) {
	meta := metav1.ObjectMeta{
		GenerateName: "cattle-cleanup-",
		Namespace:    "default",
		Labels:       map[string]string{"cattle.io/creator": "norman"},
	}

	job := batchV1.Job{
		ObjectMeta: meta,
		Spec: batchV1.JobSpec{
			Template: coreV1.PodTemplateSpec{
				Spec: coreV1.PodSpec{
					ServiceAccountName: sa,
					Containers: []coreV1.Container{
						coreV1.Container{
							Name:  "cleanup-agent",
							Image: settings.AgentImage.Get(),
							Env: []coreV1.EnvVar{
								coreV1.EnvVar{
									Name:  "CLUSTER_CLEANUP",
									Value: "true",
								},
								coreV1.EnvVar{
									Name:  "SLEEP_FIRST",
									Value: "true",
								},
							},
							ImagePullPolicy: coreV1.PullAlways,
							SecurityContext: &coreV1.SecurityContext{
								RunAsUser:  &[]int64{1000}[0],
								RunAsGroup: &[]int64{1000}[0],
							},
						},
					},
					RestartPolicy: "OnFailure",
				},
			},
		},
	}
	return userContext.K8sClient.BatchV1().Jobs("default").Create(context.TODO(), &job, metav1.CreateOptions{})
}

func (c *ClusterLifecycleCleanup) updateClusterRoleOwner(
	userContext *config.UserContext,
	role *rbacV1.ClusterRole,
	or []metav1.OwnerReference,
) error {
	return tryUpdate(func() error {
		role, err := userContext.K8sClient.RbacV1().ClusterRoles().Get(context.TODO(), role.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		role.OwnerReferences = or

		_, err = userContext.K8sClient.RbacV1().ClusterRoles().Update(context.TODO(), role, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		return nil
	})
}

func (c *ClusterLifecycleCleanup) updateServiceAccountOwner(
	userContext *config.UserContext,
	sa *coreV1.ServiceAccount,
	or []metav1.OwnerReference,
) error {
	return tryUpdate(func() error {
		sa, err := userContext.K8sClient.CoreV1().ServiceAccounts("default").Get(context.TODO(), sa.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		sa.OwnerReferences = or

		_, err = userContext.K8sClient.CoreV1().ServiceAccounts("default").Update(context.TODO(), sa, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		return nil
	})
}

func (c *ClusterLifecycleCleanup) updateClusterRoleBindingOwner(
	userContext *config.UserContext,
	crb *rbacV1.ClusterRoleBinding,
	or []metav1.OwnerReference,
) error {
	return tryUpdate(func() error {
		crb, err := userContext.K8sClient.RbacV1().ClusterRoleBindings().Get(context.TODO(), crb.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		crb.OwnerReferences = or

		_, err = userContext.K8sClient.RbacV1().ClusterRoleBindings().Update(context.TODO(), crb, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		return nil
	})
}

func cleanupNamespaces(client kubernetes.Interface) error {
	logrus.Debug("Starting cleanup of local cluster namespaces")
	namespaces, err := client.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, ns := range namespaces.Items {
		err = tryUpdate(func() error {
			nameSpace, err := client.CoreV1().Namespaces().Get(context.TODO(), ns.Name, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				return err
			}

			var updated bool

			// Cleanup finalizers
			if len(nameSpace.Finalizers) > 0 {
				finalizers := []string{}
				for _, finalizer := range nameSpace.Finalizers {
					if finalizer != "controller.cattle.io/namespace-auth" {
						finalizers = append(finalizers, finalizer)
					}
				}
				if len(nameSpace.Finalizers) != len(finalizers) {
					updated = true
					nameSpace.Finalizers = finalizers
				}
			}

			// Cleanup labels
			for _, label := range nsLabels {
				if _, ok := nameSpace.Labels[label]; ok {
					updated = ok
					delete(nameSpace.Labels, label)
				}
			}

			// Cleanup annotations
			for _, anno := range nsAnnotations {
				if _, ok := nameSpace.Annotations[anno]; ok {
					updated = ok
					delete(nameSpace.Annotations, anno)
				}
			}

			if updated {
				logrus.Debugf("Updating local namespace: %v", nameSpace.Name)
				_, err = client.CoreV1().Namespaces().Update(context.TODO(), nameSpace, metav1.UpdateOptions{})
				if err != nil {
					return err
				}

			}

			return nil
		})

		if err != nil {
			return err
		}
	}

	return nil
}

// tryUpdate runs the input func and if the error returned is a conflict error
// from k8s it will sleep and attempt to run the func again. This is useful
// when attempting to update an object.
func tryUpdate(f func() error) error {
	timeout := 100
	for i := 0; i <= 3; i++ {
		err := f()
		if err != nil {
			if apierrors.IsConflict(err) {
				time.Sleep(time.Duration(timeout) * time.Millisecond)
				timeout *= 2
				continue
			}
			return err
		}
	}
	return nil
}
