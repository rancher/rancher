package usercontrollers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	pkgcluster "github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/managementagent/nslabels"
	corecontrollers "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/image"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const (
	WebhookClusterRoleBindingName = "rancher-webhook"
	WebhookConfigurationName      = "rancher.cattle.io"

	// Reasons for the AgentUninstallScheduled condition.
	reasonJobCreated       = "JobCreated"
	reasonCompleted        = "Completed"
	reasonNotRequired      = "NotRequired"
	reasonSchedulingFailed = "SchedulingFailed"

	// userControllersStoppedRequeue is how often a cluster being removed is checked again while it waits
	// for its user controllers to be reported stopped.
	userControllersStoppedRequeue = 10 * time.Second

	// reasonUnconfirmed is the reason for the UserControllersStopped condition when no replica reported
	// the user controllers stopped in time.
	reasonUnconfirmed = "Unconfirmed"

	// userControllersStoppedTimeout is how long, from the agent uninstall being recorded, the replica that
	// owns the cluster is given to report its user controllers stopped. Every replica stops them once the
	// uninstall is recorded; the report only confirms it, and no replica can give it while none can tell
	// that it owns the cluster, e.g. while the peers aren't ready. The removal then moves on.
	userControllersStoppedTimeout = 2 * time.Minute
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
	}
)

/*
RegisterEarly registers ClusterLifecycleCleanup controller which is responsible for stopping rancher agent in user cluster,
and de-registering k8s controllers, on cluster.remove
*/
func RegisterEarly(ctx context.Context, management *config.ManagementContext, manager *clustermanager.Manager) {
	clusterClient := management.Management.Clusters("")
	mgmt := management.Wrangler.Mgmt
	lifecycle := &ClusterLifecycleCleanup{
		Manager:                manager,
		mgmtCore:               management.Core,
		clusters:               clusterClient,
		getProvisioningCluster: management.Wrangler.Provisioning.Cluster().Cache().Get,
		roleTemplateBindings: &roleTemplateBindings{
			crtbs:        mgmt.ClusterRoleTemplateBinding(),
			crtbCache:    mgmt.ClusterRoleTemplateBinding().Cache(),
			prtbs:        mgmt.ProjectRoleTemplateBinding(),
			prtbCache:    mgmt.ProjectRoleTemplateBinding().Cache(),
			projectCache: mgmt.Project().Cache(),
		},
		ctx: ctx,
	}

	clusterClient.AddLifecycle(ctx, "cluster-agent-controller-cleanup", lifecycle)
}

type ClusterLifecycleCleanup struct {
	Manager                *clustermanager.Manager
	mgmtCore               corecontrollers.Interface
	clusters               mgmtcontrollers.ClusterInterface
	getProvisioningCluster func(namespace, name string) (*provv1.Cluster, error)
	roleTemplateBindings   *roleTemplateBindings
	ctx                    context.Context
}

func (c *ClusterLifecycleCleanup) Create(obj *apimgmtv3.Cluster) (runtime.Object, error) {
	return nil, nil
}

// Remove removes the cluster's role template bindings while the downstream cluster can be reached, then
// uninstalls the Rancher agent from the downstream cluster where that is needed, then holds the cluster
// until the replica that owns it reports its user controllers stopped, or for at most
// userControllersStoppedTimeout. Every replica stops its own controllers for the cluster once the agent
// uninstall has been recorded, see the user-controllers-controller.
func (c *ClusterLifecycleCleanup) Remove(obj *apimgmtv3.Cluster) (runtime.Object, error) {
	if obj == nil {
		return obj, nil
	}

	// First remove what the cluster's role template bindings grant in the downstream cluster, while it can
	// still be reached through the cluster agent.
	if !pkgcluster.ConditionConcluded(obj, apimgmtv3.ClusterConditionRoleTemplateBindingsRemoved) && c.roleTemplateBindings != nil {
		removal, err := c.roleTemplateBindings.removeRoleTemplateBindings(obj)
		if err != nil {
			return obj, fmt.Errorf("[cluster-cleanup] removing role template bindings of cluster [%s]: %w", obj.Name, err)
		}
		if err := pkgcluster.SetCondition(c.clusters, obj, apimgmtv3.ClusterConditionRoleTemplateBindingsRemoved, removal.status, removal.reason, removal.message); err != nil {
			return obj, fmt.Errorf("[cluster-cleanup] recording role template binding removal for cluster [%s]: %w", obj.Name, err)
		}
		if !removal.concluded {
			c.clusters.Controller().EnqueueAfter("", obj.Name, userControllersStoppedRequeue)
			return obj, generic.ErrSkip
		}
	}

	// A management cluster created by a provisioning cluster keeps its tunnel until the provisioning
	// cluster's machines, which are drained through it, are gone. This only waits when the management
	// cluster is deleted on its own: a provisioning cluster deletes it once they are gone.
	if !pkgcluster.ConditionConcluded(obj, apimgmtv3.ClusterConditionAgentUninstallScheduled) && c.getProvisioningCluster != nil {
		pending, err := pkgcluster.ProvisioningInfrastructurePending(obj, c.getProvisioningCluster)
		if err != nil {
			return obj, err
		}
		if pending {
			logrus.Debugf("[cluster-cleanup] waiting for the machines of the provisioning cluster that created cluster [%s] to be removed", obj.Name)
			c.clusters.Controller().EnqueueAfter("", obj.Name, userControllersStoppedRequeue)
			return obj, generic.ErrSkip
		}
	}

	if !pkgcluster.ConditionConcluded(obj, apimgmtv3.ClusterConditionAgentUninstallScheduled) {
		status, reason, message := c.scheduleAgentUninstall(obj)
		if err := pkgcluster.SetCondition(c.clusters, obj, apimgmtv3.ClusterConditionAgentUninstallScheduled, status, reason, message); err != nil {
			return obj, fmt.Errorf("[cluster-cleanup] recording agent uninstall for cluster [%s]: %w", obj.Name, err)
		}
	}

	// The user-controllers-controller does this on every replica, this one included, once the uninstall
	// is recorded. Stopping here as well keeps this replica from waiting on its own informer.
	c.Manager.Stop(obj)

	if !pkgcluster.ConditionConcluded(obj, apimgmtv3.ClusterConditionControllersStopped) {
		if !userControllersStoppedTimedOut(obj) {
			// Reporting the controllers stopped updates the cluster, which brings it back here. The
			// requeue covers that update being missed, and the timeout.
			c.clusters.Controller().EnqueueAfter("", obj.Name, userControllersStoppedRequeue)
			return obj, generic.ErrSkip
		}
		logrus.Warnf("[cluster-cleanup] no replica reported the user controllers of cluster [%s] stopped within %s, moving on with removing the cluster", obj.Name, userControllersStoppedTimeout)
		message := fmt.Sprintf("no replica reported the user controllers stopped within %s; every replica stops them once the agent uninstall is recorded", userControllersStoppedTimeout)
		if err := pkgcluster.SetCondition(c.clusters, obj, apimgmtv3.ClusterConditionControllersStopped, corev1.ConditionFalse, reasonUnconfirmed, message); err != nil {
			return obj, fmt.Errorf("[cluster-cleanup] recording unconfirmed user controllers stop for cluster [%s]: %w", obj.Name, err)
		}
	}
	return nil, nil
}

// userControllersStoppedTimedOut reports whether the replica that owns cluster has had
// userControllersStoppedTimeout to report its user controllers stopped, counted from the agent uninstall
// being recorded. An uninstall that cluster doesn't show yet was only just recorded.
func userControllersStoppedTimedOut(cluster *apimgmtv3.Cluster) bool {
	if !pkgcluster.ConditionConcluded(cluster, apimgmtv3.ClusterConditionAgentUninstallScheduled) {
		return false
	}
	recorded, err := time.Parse(time.RFC3339, apimgmtv3.ClusterConditionAgentUninstallScheduled.GetLastUpdated(cluster))
	if err != nil {
		if cluster.DeletionTimestamp == nil {
			return false
		}
		recorded = cluster.DeletionTimestamp.Time
	}
	return time.Since(recorded) > userControllersStoppedTimeout
}

// scheduleAgentUninstall uninstalls the Rancher agent from the downstream cluster if it needs it, trying
// 3 times before giving up, and returns how to record the outcome on AgentUninstallScheduled.
func (c *ClusterLifecycleCleanup) scheduleAgentUninstall(obj *apimgmtv3.Cluster) (corev1.ConditionStatus, string, string) {
	var uninstall func(*apimgmtv3.Cluster) (string, error)
	switch {
	case obj.Name == "local" && obj.Spec.Internal:
		uninstall = c.cleanupLocalCluster
	case pkgcluster.DownstreamCleanupRequired(obj):
		uninstall = c.cleanupImportedCluster
	default:
		return corev1.ConditionTrue, reasonNotRequired, "the agent is not uninstalled from this type of cluster"
	}

	backoff := wait.Backoff{
		Duration: 3 * time.Second,
		Factor:   1,
		Steps:    3,
	}

	var reason, message string
	var lastErr error
	if err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		reason, lastErr = uninstall(obj)
		if lastErr != nil {
			logrus.Infof("[cluster-cleanup] error cleaning up cluster [%s]: %v", obj.Name, lastErr)
		}
		return lastErr == nil, nil
	}); err != nil {
		logrus.Warnf("[cluster-cleanup] could not clean imported cluster [%s], moving on with removing cluster: %v", obj.Name, lastErr)
		return corev1.ConditionFalse, reasonSchedulingFailed, lastErr.Error()
	}

	switch reason {
	case reasonNotRequired:
		message = "the cluster has no connection details, so no agent was deployed to it"
	case reasonJobCreated:
		message = "created the job that uninstalls the agent in namespace default"
	case reasonCompleted:
		message = "removed the agent and Rancher's namespace metadata from the local cluster"
	}
	return corev1.ConditionTrue, reason, message
}

func (c *ClusterLifecycleCleanup) cleanupLocalCluster(obj *apimgmtv3.Cluster) (string, error) {
	userContext, err := c.Manager.UserContextFromCluster(obj)
	if err != nil {
		return "", err
	}
	if userContext == nil {
		logrus.Debugf("could not get context for local cluster, skipping cleanup")
		return reasonNotRequired, nil
	}

	err = cleanupNamespaces(userContext.K8sClient)
	if err != nil {
		return "", err
	}

	propagationBackground := metav1.DeletePropagationBackground
	deleteOptions := &metav1.DeleteOptions{
		PropagationPolicy: &propagationBackground,
	}

	err = userContext.Apps.Deployments("cattle-system").Delete("cattle-cluster-agent", deleteOptions)
	if err != nil && !apierrors.IsNotFound(err) {
		return "", err
	}

	return reasonCompleted, nil
}

func (c *ClusterLifecycleCleanup) Updated(obj *apimgmtv3.Cluster) (runtime.Object, error) {
	return nil, nil
}

func (c *ClusterLifecycleCleanup) cleanupImportedCluster(cluster *apimgmtv3.Cluster) (string, error) {
	userContext, err := c.Manager.UserContextFromCluster(cluster)
	if err != nil {
		return "", err
	}
	if userContext == nil {
		logrus.Debugf("could not get context for imported cluster, skipping cleanup")
		return reasonNotRequired, nil
	}

	role, err := c.createCleanupClusterRole(userContext)
	if err != nil {
		return "", err
	}

	sa, err := c.createCleanupServiceAccount(userContext)
	if err != nil {
		return "", err
	}

	crb, err := c.createCleanupClusterRoleBinding(userContext, role.Name, sa.Name)
	if err != nil {
		return "", err
	}

	// create cleanup image pull secret
	secrets, err := c.createCleanupImagePullSecrets(userContext, cluster)
	if err != nil {
		return "", err
	}

	job, err := c.createCleanupJob(userContext, cluster, sa.Name, secrets)
	if err != nil {
		cleanupErr := c.cleanupImagePullSecrets(userContext, secrets)
		if cleanupErr != nil {
			return "", errors.Join(err, cleanupErr)
		}
		return "", err
	}

	or := []metav1.OwnerReference{
		metav1.OwnerReference{
			APIVersion: "batch/v1",
			Kind:       "Job",
			Name:       job.Name,
			UID:        job.UID,
		},
	}

	// These resources need the ownerReference added so they get cleaned up after
	// the job deletes itself.

	err = c.updateClusterImagePullSecretsOwner(userContext, secrets, or)
	if err != nil {
		return "", err
	}

	err = c.updateClusterRoleOwner(userContext, role, or)
	if err != nil {
		return "", err
	}

	err = c.updateServiceAccountOwner(userContext, sa, or)
	if err != nil {
		return "", err
	}

	err = c.updateClusterRoleBindingOwner(userContext, crb, or)
	if err != nil {
		return "", err
	}

	return reasonJobCreated, nil
}

func (c *ClusterLifecycleCleanup) createCleanupClusterRole(userContext *config.UserContext) (*rbacv1.ClusterRole, error) {
	meta := metav1.ObjectMeta{
		GenerateName: "cattle-cleanup-",
	}

	rules := []rbacv1.PolicyRule{
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
		// This is needed to remove any image pull secrets that were added to various namespaces by Rancher
		{
			Verbs:     []string{"list", "delete"},
			APIGroups: []string{""},
			Resources: []string{"secrets"},
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
	clusterRole := rbacv1.ClusterRole{
		ObjectMeta: meta,
		Rules:      rules,
	}
	return userContext.K8sClient.RbacV1().ClusterRoles().Create(context.TODO(), &clusterRole, metav1.CreateOptions{})
}

func (c *ClusterLifecycleCleanup) createCleanupServiceAccount(userContext *config.UserContext) (*corev1.ServiceAccount, error) {
	meta := metav1.ObjectMeta{
		GenerateName: "cattle-cleanup-",
		Namespace:    "default",
	}
	serviceAccount := corev1.ServiceAccount{
		ObjectMeta: meta,
	}
	return userContext.K8sClient.CoreV1().ServiceAccounts("default").Create(context.TODO(), &serviceAccount, metav1.CreateOptions{})
}

func (c *ClusterLifecycleCleanup) createCleanupImagePullSecrets(userContext *config.UserContext, cluster *apimgmtv3.Cluster) ([]*corev1.Secret, error) {
	registry, isGlobal := pkgcluster.GetPrivateRegistry(cluster)
	if registry == nil {
		return nil, nil
	}

	ns := namespaces.System
	if !isGlobal {
		ns = "fleet-default"
	}

	var copiedSecrets []*corev1.Secret
	for _, sec := range registry.PullSecrets {
		existingPullSec, err := c.mgmtCore.Secrets(ns).Get(sec.Name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Warnf("image pull secret %s/%s was not found, skipping copy", sec.Namespace, sec.Name)
				continue
			}
			return nil, err
		}

		data, err := pkgcluster.ConvertToDockerConfigJson(registry.URL, existingPullSec)
		if err != nil {
			return nil, err
		}

		defaultPullSecretName := name.SafeConcatName("cattle-cleanup", pkgcluster.GeneratePullSecretName(sec.Name), cluster.Name)

		s := corev1.Secret{
			Name:      defaultPullSecretName,
			Namespace: "default",
			Data: map[string][]byte{
				corev1.DockerConfigJsonKey: data,
			},
			Type: "kubernetes.io/dockerconfigjson",
		}

		defPull, err := userContext.K8sClient.CoreV1().Secrets("default").Get(c.ctx, defaultPullSecretName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				defPull, err = userContext.K8sClient.CoreV1().Secrets("default").Create(c.ctx, &s, metav1.CreateOptions{})
				if err != nil {
					return nil, err
				}
				copiedSecrets = append(copiedSecrets, defPull)
				continue
			}
			return nil, err
		}

		if !bytes.Equal(defPull.Data[corev1.DockerConfigJsonKey], data) {
			defPull = defPull.DeepCopy()
			if defPull.Data == nil {
				defPull.Data = make(map[string][]byte)
			}
			defPull.Data[corev1.DockerConfigJsonKey] = data
			_, err = userContext.K8sClient.CoreV1().Secrets("default").Update(c.ctx, defPull, metav1.UpdateOptions{})
			if err != nil {
				return nil, err
			}
		}

		copiedSecrets = append(copiedSecrets, defPull)
	}

	return copiedSecrets, nil
}

func (c *ClusterLifecycleCleanup) createCleanupClusterRoleBinding(
	userContext *config.UserContext,
	role, sa string,
) (*rbacv1.ClusterRoleBinding, error) {
	meta := metav1.ObjectMeta{
		GenerateName: "cattle-cleanup-",
		Namespace:    "default",
	}
	clusterRoleBinding := rbacv1.ClusterRoleBinding{
		ObjectMeta: meta,
		Subjects: []rbacv1.Subject{
			rbacv1.Subject{
				Kind:      "ServiceAccount",
				Name:      sa,
				Namespace: "default",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     role,
		},
	}
	return userContext.K8sClient.RbacV1().ClusterRoleBindings().Create(context.TODO(), &clusterRoleBinding, metav1.CreateOptions{})
}

func (c *ClusterLifecycleCleanup) createCleanupJob(userContext *config.UserContext, cluster *apimgmtv3.Cluster, sa string, secrets []*corev1.Secret) (*batchv1.Job, error) {
	meta := metav1.ObjectMeta{
		GenerateName: "cattle-cleanup-",
		Namespace:    "default",
		Labels:       map[string]string{"cattle.io/creator": "norman"},
	}

	job := batchv1.Job{
		ObjectMeta: meta,
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: sa,
					Containers: []corev1.Container{
						corev1.Container{
							Name:  "cleanup-agent",
							Image: image.ResolveWithCluster(settings.AgentImage.Get(), cluster),
							Env: []corev1.EnvVar{
								corev1.EnvVar{
									Name:  "CLUSTER_CLEANUP",
									Value: "true",
								},
								corev1.EnvVar{
									Name:  "SLEEP_FIRST",
									Value: "true",
								},
							},
							ImagePullPolicy: corev1.PullAlways,
							SecurityContext: &corev1.SecurityContext{
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

	for _, secret := range secrets {
		job.Spec.Template.Spec.ImagePullSecrets = append(job.Spec.Template.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: secret.Name})
	}

	return userContext.K8sClient.BatchV1().Jobs("default").Create(context.TODO(), &job, metav1.CreateOptions{})
}

func (c *ClusterLifecycleCleanup) updateClusterRoleOwner(
	userContext *config.UserContext,
	role *rbacv1.ClusterRole,
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
	sa *corev1.ServiceAccount,
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

func (c *ClusterLifecycleCleanup) updateClusterImagePullSecretsOwner(userContext *config.UserContext, secrets []*corev1.Secret, or []metav1.OwnerReference) error {
	return tryUpdate(func() error {
		for _, s := range secrets {
			existing, err := userContext.K8sClient.CoreV1().Secrets(s.Namespace).Get(c.ctx, s.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}

			existing.OwnerReferences = or

			_, err = userContext.K8sClient.CoreV1().Secrets(s.Namespace).Update(c.ctx, existing, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (c *ClusterLifecycleCleanup) updateClusterRoleBindingOwner(
	userContext *config.UserContext,
	crb *rbacv1.ClusterRoleBinding,
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

func (c *ClusterLifecycleCleanup) cleanupImagePullSecrets(userContext *config.UserContext, secrets []*corev1.Secret) error {
	for _, secret := range secrets {
		if secret == nil {
			continue
		}
		if deleteErr := userContext.K8sClient.CoreV1().Secrets("default").Delete(c.ctx, secret.Name, metav1.DeleteOptions{}); deleteErr != nil && !apierrors.IsNotFound(deleteErr) {
			logrus.WithError(deleteErr).Warnf("failed to delete cleanup image pull secret %s/%s after cleanup job creation failed", corev1.NamespaceDefault, secret.Name)
		}
	}
	return nil
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
