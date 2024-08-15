package machineconfigcleanup

import (
	"context"
	"fmt"
	v3apis "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/capr/dynamicschema"
	"github.com/rancher/rancher/pkg/fleet"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	image2 "github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemtemplate"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/apply"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type handler struct {
	clusterRegistrationTokens v3.ClusterRegistrationTokenCache
	apply                     apply.Apply
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		apply: clients.Apply.WithCacheTypes(
			clients.Batch.Job(),
			clients.RBAC.ClusterRole(),
			clients.RBAC.ClusterRoleBinding(),
			clients.Core.ServiceAccount(),
			clients.Core.ConfigMap()),
		clusterRegistrationTokens: clients.Mgmt.ClusterRegistrationToken().Cache(),
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
func (h *handler) onChange(key string, obj *v3apis.ClusterRegistrationToken) (_ *v3apis.ClusterRegistrationToken, err error) {
	if obj == nil || obj.Namespace != "local" || obj.DeletionTimestamp != nil || obj.Status.Token == "" {
		return obj, nil
	}

	if err := h.apply.
		WithSetID("rke2-machine-config-cleanup").
		WithDynamicLookup().
		WithNoDelete().ApplyObjects(cleanupObjects(obj.Status.Token)...); err != nil {
		return nil, err
	}

	return obj, nil
}

func cleanupObjects(token string) []runtime.Object {
	url := settings.ServerURL.Get()
	ca := systemtemplate.CAChecksum()
	image := image2.Resolve(settings.AgentImage.Get())
	fleetNamespace := fleet.ClustersDefaultNamespace
	prefix := "rke2-machineconfig-cleanup"

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      prefix + "-sa",
			Namespace: fleetNamespace,
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
				Namespace: fleetNamespace,
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
			Namespace: fleetNamespace,
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
			Namespace: fleetNamespace,
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

	return []runtime.Object{
		sa,
		clusterRole,
		clusterRoleBinding,
		configMap,
		cronJob,
	}
}

func int32Ptr(i int32) *int32 { return &i }
