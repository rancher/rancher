package harvestercleanup

import (
	"context"

	"github.com/rancher/wrangler/v3/pkg/data"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8dynamic "k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/capr/machineprovision"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	provcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
)

const (
	removedAllPVCsAnnotationKey = "harvesterhci.io/removeAllPersistentVolumeClaims"
)

type handler struct {
	capiClusters capicontrollers.ClusterCache
	clusterCache provcontrollers.ClusterCache
	secretCache  corecontrollers.SecretCache
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := handler{
		capiClusters: clients.CAPI.Cluster().Cache(),
		clusterCache: clients.Provisioning.Cluster().Cache(),
		secretCache:  clients.Core.Secret().Cache(),
	}

	clients.Dynamic.OnChange(ctx, "harvester-machine-provision-cleanup", validGVK, h.onChange)
}

func validGVK(gvk schema.GroupVersionKind) bool {
	// It is not necessary to check the version here.
	return gvk.Group == "rke-machine.cattle.io" && gvk.Kind == "HarvesterMachine"
}

// onChange adds an annotation to the `VirtualMachine` resource of the
// downstream cluster that informs the Harvester node driver to remove all
// PVCs when the VM is deleted from it.
// There is no other way to do this because the node driver does not know
// whether the node is only deleted because it is to be redeployed or
// whether it is to be permanently deleted because the parent cluster is
// deleted.
func (h *handler) onChange(obj runtime.Object) (runtime.Object, error) {
	if obj == nil {
		return obj, nil
	}

	objMeta, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	// Exit immediately if the object is not being deleted.
	if objMeta.GetDeletionTimestamp().IsZero() {
		return obj, nil
	}

	objData, err := data.Convert(obj)
	if err != nil {
		return nil, err
	}

	key := objMeta.GetNamespace() + "/" + objMeta.GetName()

	capiCluster, err := capr.GetCAPIClusterFromLabel(obj, h.capiClusters)
	if err != nil && !apierrors.IsNotFound(err) {
		logrus.Errorf("[harvestercleanup] %s: error getting cluster (apiversion=cluster.x-k8s.io): %v", key, err)
		return obj, err
	}
	if err != nil && apierrors.IsNotFound(err) {
		logrus.Debugf("[harvestercleanup] %s: there is no cluster (apiversion=cluster.x-k8s.io). Continue ...", key)
		return obj, nil
	}

	// Skip if the CAPI cluster (cluster.x-k8s.io) is not being deleted.
	if capiCluster.DeletionTimestamp.IsZero() {
		return obj, nil
	}

	cluster, err := h.clusterCache.Get(objMeta.GetNamespace(), capiCluster.Name)
	if err != nil {
		logrus.Errorf("[harvestercleanup] %s: error getting cluster (apiversion=provisioning.cattle.io): %v", key, err)
		return obj, err
	}

	secret, err := machineprovision.GetCloudCredentialSecret(h.secretCache, "", cluster.Spec.CloudCredentialSecretName)
	if err != nil {
		logrus.Errorf("[harvestercleanup] %s: error getting cloud credential secret: %v", key, err)
		return obj, err
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(secret.Data["harvestercredentialConfig-kubeconfigContent"])
	if err != nil {
		logrus.Errorf("[harvestercleanup] %s: error getting REST config from cloud credential secret: %v", key, err)
		return obj, err
	}

	// We need to use the dynamic client here because we do not want to import
	// the kubevirt API into Rancher.
	dynamicClient, err := k8dynamic.NewForConfig(restConfig)
	if err != nil {
		logrus.Errorf("[harvestercleanup] %s: error getting dynamic client: %v", key, err)
		return obj, err
	}

	resourceGVR := schema.GroupVersionResource{
		Group: "kubevirt.io", Version: "v1", Resource: "virtualmachines",
	}
	resourceName := objMeta.GetName()
	resourceNamespace := objData.String("spec", "vmNamespace")

	resource, err := dynamicClient.Resource(resourceGVR).Namespace(resourceNamespace).Get(context.TODO(), resourceName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		logrus.Errorf("[harvestercleanup] %s: error getting VM (gvr=%s, namespace=%s, name=%s): %v",
			key, resourceGVR.String(), resourceNamespace, resourceName, err)
		return obj, err
	}
	if err != nil && apierrors.IsNotFound(err) {
		logrus.Debugf("[harvestercleanup] %s: there is no VM (gvr=%s, namespace=%s, name=%s). Continue ...",
			key, resourceGVR.String(), resourceNamespace, resourceName)
		return obj, nil
	}

	annotations := resource.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if _, ok := annotations[removedAllPVCsAnnotationKey]; !ok {
		annotations[removedAllPVCsAnnotationKey] = "true"
		resource.SetAnnotations(annotations)

		_, err = dynamicClient.Resource(resourceGVR).Namespace(resourceNamespace).Update(context.TODO(), resource, metav1.UpdateOptions{})
		if err != nil {
			logrus.Errorf("[harvestercleanup] %s: error updating VM (gvr=%s, namespace=%s, name=%s): %v",
				key, resourceGVR.String(), resourceNamespace, resourceName, err)
			return obj, err
		}

		logrus.Infof("[harvestercleanup] %s: VM successfully marked for removal (gvr=%s, namespace=%s, name=%s)",
			key, resourceGVR.String(), resourceNamespace, resourceName)
	}

	return obj, nil
}
