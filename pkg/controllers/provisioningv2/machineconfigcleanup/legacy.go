package machineconfigcleanup

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/fleet"
	"github.com/rancher/wrangler/v3/pkg/apply"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// legacyNamespace is where Rancher v2.13.x and earlier created the namespaced cleanup objects.
const legacyNamespace = fleet.ClustersDefaultNamespace

// RemoveLegacyObjects deletes the cronjob, config map, and service account that Rancher v2.13.x and earlier created in
// the fleet-default namespace.
func RemoveLegacyObjects(ctx context.Context, k8s kubernetes.Interface) error {
	cronJobs := k8s.BatchV1().CronJobs(legacyNamespace)
	cronJob, err := cronJobs.Get(ctx, prefix+"-cronjob", metav1.GetOptions{})
	if err := deleteIfOwned(ctx, cronJob, err, cronJobs.Delete); err != nil {
		return err
	}

	configMaps := k8s.CoreV1().ConfigMaps(legacyNamespace)
	configMap, err := configMaps.Get(ctx, prefix+"-script", metav1.GetOptions{})
	if err := deleteIfOwned(ctx, configMap, err, configMaps.Delete); err != nil {
		return err
	}

	serviceAccounts := k8s.CoreV1().ServiceAccounts(legacyNamespace)
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
