package usercontrollers

import (
	"context"
	"time"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/user/helm"
	"github.com/rancher/rancher/pkg/controllers/user/nslabels"
	"github.com/rancher/rancher/pkg/settings"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	batchV1 "k8s.io/api/batch/v1"
	coreV1 "k8s.io/api/core/v1"
	rbacV1 "k8s.io/api/rbac/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
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
	var err error
	if obj.Name == "local" && obj.Spec.Internal {
		err = c.cleanupLocalCluster(obj)
	} else if obj.Status.Driver == v3.ClusterDriverImported ||
		obj.Status.Driver == v3.ClusterDriverK3s ||
		obj.Status.Driver == v3.ClusterDriverK3os {
		err = c.cleanupImportedCluster(obj)
	}
	if err != nil {
		apiError, ok := err.(*httperror.APIError)
		// If it's not an API error give it back
		if !ok {
			return nil, err
		}
		// If it's anything but clusterUnavailable give it back
		if apiError.Code != httperror.ClusterUnavailable {
			return nil, err
		}
	}

	c.Manager.Stop(obj)
	return nil, nil
}

func (c *ClusterLifecycleCleanup) cleanupLocalCluster(obj *v3.Cluster) error {
	userContext, err := c.Manager.UserContext(obj.Name)
	if err != nil {
		return err
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
	userContext, err := c.Manager.UserContext(cluster.Name)
	if err != nil {
		return err
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

	err = userContext.Core.Namespaces("").Delete("cattle-system", &metav1.DeleteOptions{})
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
		rbacV1.PolicyRule{
			Verbs:     []string{"list", "get", "update", "delete"},
			APIGroups: []string{""},
			Resources: []string{"namespaces"},
		},
		rbacV1.PolicyRule{
			Verbs:     []string{"list", "get", "delete"},
			APIGroups: []string{"rbac.authorization.k8s.io"},
			Resources: []string{"roles", "rolebindings", "clusterroles", "clusterrolebindings"},
		},
		// The job is going to delete itself after running to trigger ownerReference
		// cleanup of the clusterRole, serviceAccount and clusterRoleBinding
		rbacV1.PolicyRule{
			Verbs:     []string{"list", "get", "delete"},
			APIGroups: []string{"batch"},
			Resources: []string{"jobs"},
		},
	}
	clusterRole := rbacV1.ClusterRole{
		ObjectMeta: meta,
		Rules:      rules,
	}
	return userContext.K8sClient.RbacV1().ClusterRoles().Create(&clusterRole)
}

func (c *ClusterLifecycleCleanup) createCleanupServiceAccount(userContext *config.UserContext) (*coreV1.ServiceAccount, error) {
	meta := metav1.ObjectMeta{
		GenerateName: "cattle-cleanup-",
		Namespace:    "default",
	}
	serviceAccount := coreV1.ServiceAccount{
		ObjectMeta: meta,
	}
	return userContext.K8sClient.CoreV1().ServiceAccounts("default").Create(&serviceAccount)
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
	return userContext.K8sClient.RbacV1().ClusterRoleBindings().Create(&clusterRoleBinding)
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
						},
					},
					RestartPolicy: "OnFailure",
				},
			},
		},
	}
	return userContext.K8sClient.BatchV1().Jobs("default").Create(&job)
}

func (c *ClusterLifecycleCleanup) updateClusterRoleOwner(
	userContext *config.UserContext,
	role *rbacV1.ClusterRole,
	or []metav1.OwnerReference,
) error {
	return tryUpdate(func() error {
		role, err := userContext.K8sClient.RbacV1().ClusterRoles().Get(role.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		role.OwnerReferences = or

		_, err = userContext.K8sClient.RbacV1().ClusterRoles().Update(role)
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
		sa, err := userContext.K8sClient.CoreV1().ServiceAccounts("default").Get(sa.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		sa.OwnerReferences = or

		_, err = userContext.K8sClient.CoreV1().ServiceAccounts("default").Update(sa)
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
		crb, err := userContext.K8sClient.RbacV1().ClusterRoleBindings().Get(crb.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		crb.OwnerReferences = or

		_, err = userContext.K8sClient.RbacV1().ClusterRoleBindings().Update(crb)
		if err != nil {
			return err
		}
		return nil
	})
}

func cleanupNamespaces(client kubernetes.Interface) error {
	logrus.Debug("Starting cleanup of local cluster namespaces")
	namespaces, err := client.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, ns := range namespaces.Items {
		err = tryUpdate(func() error {
			nameSpace, err := client.CoreV1().Namespaces().Get(ns.Name, metav1.GetOptions{})
			if err != nil {
				if apierror.IsNotFound(err) {
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
				_, err = client.CoreV1().Namespaces().Update(nameSpace)
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
