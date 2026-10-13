package cluster

import (
	"slices"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/apply"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProvisioningClusterRemoveFinalizer is the finalizer with which a provisioning cluster tears down its CAPI
// cluster and machines. It is removed once they are gone.
const ProvisioningClusterRemoveFinalizer = "wrangler.cattle.io/provisioning-cluster-remove"

var (
	ProvisioningClusterGVK = provv1.SchemeGroupVersion.WithKind("Cluster").String()
	ManagementClusterGVK   = apimgmtv3.SchemeGroupVersion.WithKind("Cluster").String()
)

// CreatedBy reports whether obj was created by the object of the given kind, namespace and name, according
// to the owner annotations wrangler apply sets on the objects it creates.
func CreatedBy(obj metav1.Object, gvk, namespace, name string) bool {
	annotations := obj.GetAnnotations()
	return annotations[apply.LabelGVK] == gvk &&
		annotations[apply.LabelNamespace] == namespace &&
		annotations[apply.LabelName] == name
}

// CreatedByProvisioningCluster returns the namespace and name of the provisioning cluster that created
// mgmtCluster, if a provisioning cluster did.
func CreatedByProvisioningCluster(mgmtCluster *apimgmtv3.Cluster) (string, string, bool) {
	annotations := mgmtCluster.GetAnnotations()
	if annotations[apply.LabelGVK] != ProvisioningClusterGVK || annotations[apply.LabelName] == "" {
		return "", "", false
	}
	return annotations[apply.LabelNamespace], annotations[apply.LabelName], true
}

// ProvisioningInfrastructurePending reports whether mgmtCluster was created by a provisioning cluster that
// still has CAPI machines to tear down. They are drained through the management cluster's tunnel, so the
// management cluster must not be disconnected until they are gone.
func ProvisioningInfrastructurePending(mgmtCluster *apimgmtv3.Cluster, getProvisioningCluster func(namespace, name string) (*provv1.Cluster, error)) (bool, error) {
	namespace, name, ok := CreatedByProvisioningCluster(mgmtCluster)
	if !ok {
		return false, nil
	}
	provCluster, err := getProvisioningCluster(namespace, name)
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return slices.Contains(provCluster.Finalizers, ProvisioningClusterRemoveFinalizer), nil
}
