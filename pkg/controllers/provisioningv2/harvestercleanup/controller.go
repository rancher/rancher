package harvestercleanup

import (
	"context"
	"fmt"
	"strconv"

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
	harvMachineProvCleanupHandlerName       = "harvester-machine-provision-cleanup"
	skipHarvMachineProvCleanupAnnotationKey = "harvesterhci.io/skipHarvesterMachineProvisionCleanup"
	removedAllPVCsAnnotationKey             = "harvesterhci.io/removeAllPersistentVolumeClaims"
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

	clients.Dynamic.OnChange(ctx, harvMachineProvCleanupHandlerName, validGVK, h.onChange)
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
	if apierrors.IsNotFound(err) {
		logrus.Warnf("[%s] %s: there is no cluster (apiversion=cluster.x-k8s.io). Skipping ...", harvMachineProvCleanupHandlerName, key)
		return obj, nil
	}
	if err != nil {
		logrus.Errorf("[%s] %s: error getting cluster (apiversion=cluster.x-k8s.io): %v", harvMachineProvCleanupHandlerName, key, err)
		return obj, err
	}

	// Skip if the CAPI cluster (cluster.x-k8s.io) is not being deleted.
	if capiCluster.DeletionTimestamp.IsZero() {
		return obj, nil
	}

	// Skip if forced via annotation that contains a boolean value.
	if value, ok := capiCluster.Annotations[skipHarvMachineProvCleanupAnnotationKey]; ok {
		boolValue, err := strconv.ParseBool(value)
		if boolValue && err == nil {
			return obj, nil
		}
	}

	cluster, err := h.clusterCache.Get(objMeta.GetNamespace(), capiCluster.Name)
	if err != nil {
		logrus.Errorf("[%s] %s: error getting cluster (apiversion=provisioning.cattle.io): %v", harvMachineProvCleanupHandlerName, key, err)
		return obj, err
	}

	// Skip if forced via annotation that contains a boolean value.
	if value, ok := cluster.Annotations[skipHarvMachineProvCleanupAnnotationKey]; ok {
		boolValue, err := strconv.ParseBool(value)
		if boolValue && err == nil {
			return obj, nil
		}
	}

	secret, err := machineprovision.GetCloudCredentialSecret(h.secretCache, "", cluster.Spec.CloudCredentialSecretName)
	if err != nil {
		logrus.Errorf("[%s] %s: error getting cloud credential secret: %v", harvMachineProvCleanupHandlerName, key, err)
		return obj, err
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(secret.Data["harvestercredentialConfig-kubeconfigContent"])
	if err != nil {
		logrus.Errorf("[%s] %s: error getting REST config from cloud credential secret: %v", harvMachineProvCleanupHandlerName, key, err)
		return obj, err
	}

	// We need to use the dynamic client here because we do not want to import
	// the kubevirt API into Rancher.
	dynamicClient, err := k8dynamic.NewForConfig(restConfig)
	if err != nil {
		logrus.Errorf("[%s] %s: error getting dynamic client: %v", harvMachineProvCleanupHandlerName, key, err)
		return obj, err
	}

	resourceGVR := schema.GroupVersionResource{
		Group: "kubevirt.io", Version: "v1", Resource: "virtualmachines",
	}
	resourceName := objMeta.GetName()
	resourceNamespace := objData.String("spec", "vmNamespace")

	resource, err := dynamicClient.Resource(resourceGVR).Namespace(resourceNamespace).Get(context.TODO(), resourceName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		logrus.Warnf("[%s] %s: there is no VM (gvr=%s, namespace=%s, name=%s). Skipping ...",
			harvMachineProvCleanupHandlerName, key, resourceGVR.String(), resourceNamespace, resourceName)
		return obj, nil
	}
	if err != nil {
		logrus.Errorf("[%s] %s: error getting VM (gvr=%s, namespace=%s, name=%s): %v",
			harvMachineProvCleanupHandlerName, key, resourceGVR.String(), resourceNamespace, resourceName, err)
		return obj, err
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
			if apierrors.IsNotFound(err) {
				logrus.Warnf("[%s] %s: there is no VM to update (gvr=%s, namespace=%s, name=%s). Skipping ...",
					harvMachineProvCleanupHandlerName, key, resourceGVR.String(), resourceNamespace, resourceName)
				return obj, nil
			} else {
				err = fmt.Errorf("failed to update VM (gvr=%s, namespace=%s, name=%s): %w", resourceGVR.String(), resourceNamespace, resourceName, err)
				logrus.Errorf("[%s] %s: %v", harvMachineProvCleanupHandlerName, key, err)
				return obj, err
			}
		}

		logrus.Infof("[%s] %s: VM successfully marked for removal (gvr=%s, namespace=%s, name=%s)",
			harvMachineProvCleanupHandlerName, key, resourceGVR.String(), resourceNamespace, resourceName)
	}

	return obj, nil
}
