package machineconfigcleanup

import (
	"context"
	"errors"
	"fmt"

	v3apis "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/rancher/pkg/controllers/capr/dynamicschema"
	crt "github.com/rancher/rancher/pkg/controllers/dashboard/clusterregistrationtoken"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/fleet"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	image2 "github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemtemplate"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/apply"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

const (
	setID  = "rke2-machine-config-cleanup"
	prefix = "rke2-machineconfig-cleanup"

	// legacyNamespace is where Rancher v2.13.x and earlier created the namespaced cleanup objects.
	legacyNamespace = fleet.ClustersDefaultNamespace
)

type handler struct {
	clusterRegistrationTokens v3.ClusterRegistrationTokenCache
	secretCache               corecontrollers.SecretCache
	apply                     apply.Apply
	k8s                       kubernetes.Interface
}

func Register(ctx context.Context, clients *wrangler.CAPIContext) {
	h := &handler{
		apply: clients.Apply.WithCacheTypes(
			clients.Batch.Job(),
			clients.RBAC.ClusterRole(),
			clients.RBAC.ClusterRoleBinding(),
			clients.Core.ServiceAccount(),
			clients.Core.ConfigMap()),
		clusterRegistrationTokens: clients.Mgmt.ClusterRegistrationToken().Cache(),
		secretCache:               clients.Core.Secret().Cache(),
		k8s:                       clients.K8s,
	}

	clients.Mgmt.ClusterRegistrationToken().OnChange(ctx, "cluster-registration-token", h.onChange)
}

// This handler deploys a CronJob that periodically deletes orphaned rke-machine-config resources.
// The cleanup is necessary because machine configurations are adopted by provisioning clusters only
// when the cluster creation is successful. If a user fails to create a cluster or cancels an update
// after using "Edit as YAML," the machine configuration objects can become orphaned.
//
// The CronJob collects all machine configuration CRDs on each run, ensuring that
// new machine configurations added post-startup are included. It also collects the
// list of namespaces where these machine configurations are created, the `fleetWorkspaceName`
// field of the provisioning cluster object could point to any namespace within the local cluster.
//
// The logic is triggered on every update to a ClusterRegistrationToken, as the job
// requires the most recent token to run `kubectl` successfully.
func (h *handler) onChange(_ string, obj *v3apis.ClusterRegistrationToken) (_ *v3apis.ClusterRegistrationToken, err error) {
	if obj == nil || obj.Namespace != "local" || obj.DeletionTimestamp != nil || obj.Status.TokenSecretName == "" {
		return obj, nil
	}

	token, err := crt.GetTokenFromSecret(h.secretCache, obj)
	if errors.Is(err, crt.ErrTokenNotReady) {
		return obj, nil
	}
	if err != nil {
		return nil, err
	}

	if err := h.apply.
		WithSetID(setID).
		WithDynamicLookup().
		WithNoDelete().ApplyObjects(cleanupObjects(token)...); err != nil {
		return nil, err
	}

	if err := h.removeLegacyObjects(context.TODO()); err != nil {
		return nil, err
	}

	return obj, nil
}

// removeLegacyObjects deletes the cronjob, config map, and service account that Rancher v2.13.x and earlier created in
// the fleet-default namespace.
func (h *handler) removeLegacyObjects(ctx context.Context) error {
	cronJobs := h.k8s.BatchV1().CronJobs(legacyNamespace)
	cronJob, err := cronJobs.Get(ctx, prefix+"-cronjob", metav1.GetOptions{})
	if err := deleteIfOwned(ctx, cronJob, err, cronJobs.Delete); err != nil {
		return err
	}

	configMaps := h.k8s.CoreV1().ConfigMaps(legacyNamespace)
	configMap, err := configMaps.Get(ctx, prefix+"-script", metav1.GetOptions{})
	if err := deleteIfOwned(ctx, configMap, err, configMaps.Delete); err != nil {
		return err
	}

	serviceAccounts := h.k8s.CoreV1().ServiceAccounts(legacyNamespace)
	serviceAccount, err := serviceAccounts.Get(ctx, prefix+"-sa", metav1.GetOptions{})
	return deleteIfOwned(ctx, serviceAccount, err, serviceAccounts.Delete)
}

// deleteIfOwned deletes obj if it was created by this handler's apply. getErr is the error returned when fetching obj.
func deleteIfOwned(ctx context.Context, obj metav1.Object, getErr error, deleteFunc func(context.Context, string, metav1.DeleteOptions) error) error {
	if apierrors.IsNotFound(getErr) {
		return nil
	} else if getErr != nil {
		return getErr
	}

	if obj.GetAnnotations()[apply.LabelID] != setID {
		return nil
	}

	uid, resourceVersion := obj.GetUID(), obj.GetResourceVersion()
	propagation := metav1.DeletePropagationBackground
	err := deleteFunc(ctx, obj.GetName(), metav1.DeleteOptions{
		Preconditions:     &metav1.Preconditions{UID: &uid, ResourceVersion: &resourceVersion},
		PropagationPolicy: &propagation,
	})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting legacy %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
	}
	return nil
}

// cleanupObjects returns the service account, cluster role+binding, config map, and cronjob required for periodically
// cleaning up outdated and unowned machine configs. This handler is only registered if the features.ProvisioningV2
// setting is true, but if features.RKE2 is false, the CRDs required for the cronjob script would not exist and the
// script will fail. For the Harvester use case, features.ProvisioningV2 and features.RKE2 will be enabled, but
// features.MCM will be disabled (and Harvester local does not have machine configs due to custom clusters), which is
// why the "fleet-local" cluster is used for this cronjob as it should always exist (as opposed to "fleet-default",
// which is absent for Harvester).
func cleanupObjects(token string) []runtime.Object {
	// Check features.ProvisioningV2 to be explicit, even though it's checked when registering.
	if !features.ProvisioningV2.Enabled() || !features.RKE2.Enabled() {
		return []runtime.Object{}
	}

	url := settings.ServerURL.Get()
	ca := systemtemplate.CAChecksum()
	image := image2.Resolve(settings.AgentImage.Get())

	// Use fleet-local namespace as it's guaranteed to exist.
	namespace := fleet.ClustersLocalNamespace

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      prefix + "-sa",
			Namespace: namespace,
		},
	}

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: prefix + "-role",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{dynamicschema.MachineConfigAPIGroup},
				Resources: []string{"*"},
				Verbs:     []string{"get", "list", "watch", "delete"},
			},
			{
				APIGroups: []string{"apiextensions.k8s.io"},
				Resources: []string{"customresourcedefinitions"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"provisioning.cattle.io"},
				Resources: []string{"clusters"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: prefix + "-binding",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      sa.Name,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      prefix + "-script",
			Namespace: namespace,
		},
		Data: map[string]string{
			"cleanup.sh": `#!/bin/bash

			# Fetch and save all CRD names containing 'rke-machine-config.cattle.io'
			crds=$(kubectl get crds -o custom-columns=NAME:.metadata.name --no-headers | grep 'rke-machine-config\.cattle\.io')
			
			# Collect all namespaces from fleetWorkspaceName field of provisioning clusters
			namespaces=$(kubectl get clusters.provisioning.cattle.io -A -o json | jq -r '.items[].status.fleetWorkspaceName // empty' | sort -u)
			
			if [ -z "$namespaces" ]; then
  				namespaces="fleet-default"
			fi

			# Loop through each namespace
			for ns in $namespaces; do
	
			  # Loop through each CRD name
			  for crd in $crds; do

				# Get resources of the current CRD and collect those with no ownerReferences and older than 1 hour
				resources=$(kubectl get $crd -n $ns -o json | \
				  jq -r '
					.items[] |
					select(.metadata.ownerReferences == null) |
					select((now - (.metadata.creationTimestamp | fromdateiso8601)) > 3600) |
					.metadata.name' | \
				  xargs)

				if [ -n "$resources" ]; then
				  echo "Deleting resources: $resources in namespace: $ns"
				  kubectl delete $crd -n $ns $resources
				fi
			  done
			done`,
		},
	}

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      prefix + "-cronjob",
			Namespace: namespace,
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "5 0 * * *", // at 12:05am every day
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					BackoffLimit: int32Ptr(10),
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							ServiceAccountName: sa.Name,
							Containers: []corev1.Container{
								{
									Name:  fmt.Sprintf("%s-pod", prefix),
									Image: image,
									SecurityContext: &corev1.SecurityContext{
										RunAsUser:  &[]int64{65534}[0],
										RunAsGroup: &[]int64{65534}[0],
									},
									Env: []corev1.EnvVar{
										{
											Name:  "CATTLE_SERVER",
											Value: url,
										},
										{
											Name:  "CATTLE_CA_CHECKSUM",
											Value: ca,
										},
										{
											Name:  "CATTLE_TOKEN",
											Value: token,
										},
									},
									Command: []string{"/bin/sh"},
									Args:    []string{"/helper/cleanup.sh"},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "config-volume",
											MountPath: "/helper",
										},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "config-volume",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: configMap.Name,
											},
										},
									},
								},
							},
							RestartPolicy: corev1.RestartPolicyNever,
						},
					},
				},
			},
		},
	}

	registry, _ := cluster.GetPrivateRegistry(nil)
	if registry != nil {
		cronJob.Spec.JobTemplate.Spec.Template.Spec.ImagePullSecrets = registry.PullSecretsAsObjectReferences()
	}

	return []runtime.Object{
		sa,
		clusterRole,
		clusterRoleBinding,
		configMap,
		cronJob,
	}
}

func int32Ptr(i int32) *int32 { return &i }
