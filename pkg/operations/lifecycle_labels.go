package operations

import (
	"fmt"

	planapi "github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// CheckLifecycleLabels collects every machine-plan secret of the operation's cluster and checks that
// each one's lifecycle labels tie it to that cluster and to a machine. It returns a description of the
// first secret that fails, for the operation to be rejected with, or "" if every secret passes. An
// error is returned only for a failure that may pass on retry.
//
// The machine-plan webhook resolves a plan's beacon, and holds the plan to its cluster, through these
// labels, so an operation checks them in its preflight, before it pauses the cluster or assigns
// anything. Every secret of the cluster is checked, not only the ones a step acts on, since the fence
// applies to every plan written to the cluster.
func CheckLifecycleLabels(secrets planapi.SecretClient, cluster *unstructured.Unstructured, namespace string, clusterRef *corev1.ObjectReference) (string, error) {
	collected, err := planapi.NewCollector(secrets, cluster, namespace).Collect()
	if planapi.IsTransient(err) {
		return "", err
	} else if err != nil {
		return fmt.Sprintf("encountered terminal error collecting machine-plan secrets: %v", err), nil
	}

	return LifecycleLabelsProblem(clusterRef, collected), nil
}

// LifecycleLabelsProblem returns a description of the first of the given machine-plan secrets whose
// lifecycle labels don't tie it to the cluster clusterRef names and to a machine, or "" if they all do.
//
// The cluster labels (group, kind and name) must equal the cluster's. The machine labels must be
// present and non-empty: which machine a plan belongs to isn't something the operation can check, but
// a plan that belongs to none can't be held to one.
func LifecycleLabelsProblem(clusterRef *corev1.ObjectReference, secrets []*corev1.Secret) string {
	if clusterRef == nil {
		return "the operation has no clusterRef to check machine-plan secrets against"
	}
	groupVersion, err := schema.ParseGroupVersion(clusterRef.APIVersion)
	if err != nil {
		return fmt.Sprintf("the operation's clusterRef has an invalid apiVersion %q: %v", clusterRef.APIVersion, err)
	}

	cluster := []struct{ key, want string }{
		{planv1alpha1.ClusterLifecycleGroupLabel, groupVersion.Group},
		{planv1alpha1.ClusterLifecycleKindLabel, clusterRef.Kind},
		{planv1alpha1.ClusterLifecycleNameLabel, clusterRef.Name},
	}
	machine := []string{
		planv1alpha1.MachineLifecycleGroupLabel,
		planv1alpha1.MachineLifecycleKindLabel,
		planv1alpha1.MachineLifecycleNameLabel,
	}

	for _, secret := range secrets {
		if secret == nil {
			continue
		}

		for _, label := range cluster {
			got, ok := secret.Labels[label.key]
			if !ok {
				return fmt.Sprintf("machine-plan secret %s/%s has no %s label", secret.Namespace, secret.Name, label.key)
			}
			if got != label.want {
				return fmt.Sprintf("machine-plan secret %s/%s has %s=%q, but the operation's cluster is %q",
					secret.Namespace, secret.Name, label.key, got, label.want)
			}
		}

		for _, key := range machine {
			if secret.Labels[key] == "" {
				return fmt.Sprintf("machine-plan secret %s/%s has no %s label", secret.Namespace, secret.Name, key)
			}
		}
	}

	return ""
}
