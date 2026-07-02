package snapshotbackpopulate

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	jsonpatch "github.com/evanphx/json-patch/v5"
	k3s "github.com/k3s-io/api/k3s.cattle.io/v1"
	k3scontrollers "github.com/k3s-io/api/pkg/generated/controllers/k3s.cattle.io/v1"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1/snapshotutil"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	provcluster "github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	rkev1controllers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	planapi "github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/types/config"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	StorageAnnotationKey = "etcdsnapshot.rke.io/storage"
	// SnapshotFileNameAnnotationKey is the annotation key used to store the snapshot file resource name.
	SnapshotFileNameAnnotationKey = "etcdsnapshot.rke.io/snapshot-file-name"
	// RestoreModeOptionsAnnotation is the annotation key used to store the available restore modes.
	RestoreModeOptionsAnnotation = "etcdsnapshot.rke.io/restore-mode-options"
)

type Storage string

const (
	S3    Storage = "s3"
	Local Storage = "local"
)

type dynamicClient interface {
	Get(gvk schema.GroupVersionKind, namespace, name string) (runtime.Object, error)
}

type handler struct {
	clusterRef corev1.ObjectReference

	dynamic                dynamicClient
	etcdSnapshotCache      rkev1controllers.ETCDSnapshotCache
	etcdSnapshotController rkev1controllers.ETCDSnapshotController
	beaconCache            plancontrollers.BeaconCache

	nodeCache                  corecontrollers.NodeCache
	etcdSnapshotFileController k3scontrollers.ETCDSnapshotFileController
	etcdSnapshotFileCache      k3scontrollers.ETCDSnapshotFileCache
}

// Register sets up the v2provisioning snapshot backpopulate controller. This controller is responsible for monitoring
// the downstream etcd-snapshots configmap and backpopulating snapshots into etcd snapshot objects in the management cluster.
func Register(ctx context.Context, userContext *config.UserContext, cluster *apimgmtv3.Cluster) {
	logrus.Debugf("[snapshotbackpopulate] Registering controller for cluster %s", userContext.ClusterName)
	h := handler{
		dynamic:                    userContext.Management.Wrangler.Dynamic,
		etcdSnapshotCache:          userContext.Management.Wrangler.RKE.ETCDSnapshot().Cache(),
		etcdSnapshotController:     userContext.Management.Wrangler.RKE.ETCDSnapshot(),
		beaconCache:                userContext.Management.Wrangler.Plan.Beacon().Cache(),
		nodeCache:                  userContext.Corew.Node().Cache(),
		etcdSnapshotFileController: userContext.K3s.V1().ETCDSnapshotFile(),
		etcdSnapshotFileCache:      userContext.K3s.V1().ETCDSnapshotFile().Cache(),
	}

	// todo: find a better way to do this
	if cluster.Annotations["provisioning.cattle.io/administrated"] == "true" {
		provCluster, err := userContext.Management.Wrangler.Provisioning.Cluster().Cache().GetByIndex(provcluster.ByCluster, cluster.Name)
		if err != nil {
			logrus.Errorf("error getting provisioning cluster %s: %v", cluster.Name, err)
			return
		}
		if len(provCluster) != 1 {
			logrus.Errorf("expected 1 provisioning cluster for cluster %s, got %d", cluster.Name, len(provCluster))
			return
		}
		// get provisioning cluster
		h.clusterRef = corev1.ObjectReference{
			APIVersion: provCluster[0].APIVersion,
			Kind:       provCluster[0].Kind,
			Namespace:  provCluster[0].GetNamespace(),
			Name:       provCluster[0].GetName(),
		}
	} else {
		h.clusterRef = corev1.ObjectReference{
			APIVersion: apimgmtv3.SchemeGroupVersion.String(),
			Kind:       apimgmtv3.Kind("Cluster").Kind,
			Name:       cluster.Name,
		}
	}

	userContext.Management.Wrangler.RKE.ETCDSnapshot().OnChange(ctx, "snapshotcleanup", h.OnUpstreamChange)
	userContext.K3s.V1().ETCDSnapshotFile().OnChange(ctx, "snapshotbackpopulate", h.OnDownstreamChange)
}

// OnUpstreamChange will check if the downstream snapshot CR exists for a given snapshot, and if it does not the local
// representation is summarily deleted.
func (h *handler) OnUpstreamChange(_ string, snapshot *rkev1.ETCDSnapshot) (*rkev1.ETCDSnapshot, error) {
	if snapshot == nil {
		return nil, nil
	}

	cluster, err := h.getCluster()
	if err != nil {
		return snapshot, err
	}

	namespace := cluster.GetNamespace()
	if namespace == "" {
		// Assume cluster-scoped resources (e.g. mgmt cluster) use namespace mapping to name of the resource
		namespace = cluster.GetName()
	}

	beacon, err := h.beaconCache.Get(namespace, cluster.GetName())
	if err != nil && !apierrors.IsNotFound(err) {
		return snapshot, err
	}

	// Abort if anything is holding the beacon
	if !planapi.HoldingBeacon(beacon, "") {
		h.etcdSnapshotController.EnqueueAfter(snapshot.Namespace, snapshot.Name, 1*time.Minute)
		return snapshot, nil
	}

	if snapshot.Namespace != namespace || snapshot.Labels == nil || snapshot.Labels[capr.ClusterNameLabel] != cluster.GetName() {
		return snapshot, nil
	}

	logPrefix := getLogPrefix(cluster)

	// Only delete snapshots if the annotation is present: this will allow users to manually create snapshot objects during a DR scenario
	if snapshot.Annotations == nil || snapshot.Annotations[capr.SnapshotNameAnnotation] == "" {
		return snapshot, nil
	}

	_, err = h.etcdSnapshotFileController.Get(snapshot.Annotations[capr.SnapshotNameAnnotation], metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		// If the downstream snapshot does not exist in the downstream cluster, delete the local version
		logrus.Debugf("%s deleting snapshot %s", logPrefix, snapshot.Name)
		return nil, h.etcdSnapshotController.Delete(snapshot.Namespace, snapshot.Name, &metav1.DeleteOptions{})
	} else if err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func (h *handler) OnDownstreamChange(_ string, downstream *k3s.ETCDSnapshotFile) (*k3s.ETCDSnapshotFile, error) {
	if downstream == nil {
		return nil, nil
	}

	cluster, err := h.getCluster()
	if err != nil {
		return downstream, err
	}

	logPrefix := getLogPrefix(cluster)

	if cluster.GetDeletionTimestamp() != nil {
		logrus.Debugf("%s skipping snapshot reconcile as cluster is being deleted", logPrefix)
		return downstream, nil
	}

	if downstream.DeletionTimestamp != nil {
		logrus.Infof("%s downstream snapshot %s was deleted, deleting local snapshot representation", logPrefix, downstream.Name)

		upstreamSnapshots, err := h.getSnapshotsFromSnapshotFile(cluster, downstream)
		if err != nil {
			return downstream, err
		}
		var errs []error
		for _, upstreamSnapshot := range upstreamSnapshots {
			logrus.Infof("%s deleting local snapshot %s", logPrefix, upstreamSnapshot.Name)

			err := h.etcdSnapshotController.Delete(upstreamSnapshot.Namespace, upstreamSnapshot.Name, &metav1.DeleteOptions{})
			if err != nil {
				logrus.Errorf("%s error deleting snapshot %s: %v", logPrefix, upstreamSnapshot.Name, err)
				errs = append(errs, err)
			}
		}
		return downstream, errors.Join(errs...)
	}

	namespace := cluster.GetNamespace()
	if namespace == "" {
		// Assume cluster-scoped resources (e.g. mgmt cluster) use namespace mapping to name of the resource
		namespace = cluster.GetName()
	}

	beacon, err := h.beaconCache.Get(namespace, cluster.GetName())
	if err != nil && !apierrors.IsNotFound(err) {
		return downstream, err
	}

	// Abort if anything is holding the beacon
	if !planapi.HoldingBeacon(beacon, "") {
		h.etcdSnapshotFileController.EnqueueAfter(downstream.Name, 1*time.Minute)
		return downstream, nil
	}

	logrus.Infof("%s processing snapshot %s", logPrefix, downstream.Name)

	// get upstream snapshot object
	// if upstream snapshot object does not exist, create it
	upstreamSnapshots, err := h.getSnapshotsFromSnapshotFile(cluster, downstream)
	if err != nil {
		return downstream, err
	}

	if len(upstreamSnapshots) == 0 {
		// create snapshot
		upstream, err := h.populateUpstreamSnapshotFromDownstream(nil, downstream, cluster)
		if err != nil {
			return downstream, err
		}
		logrus.Debugf("%s creating snapshot %s", logPrefix, upstream.Name)

		_, err = h.etcdSnapshotController.Create(upstream)
		// snapshot may exist on a previous version of Rancher but fail to indexer criteria, update in this case
		if apierrors.IsAlreadyExists(err) {
			upstream, err = h.etcdSnapshotCache.Get(upstream.Namespace, upstream.Name)
			if err != nil {
				return downstream, err
			}

			upstream, err = h.populateUpstreamSnapshotFromDownstream(upstream, downstream, cluster)
			if err != nil {
				return downstream, err
			}

			logrus.Debugf("%s snapshot %s already exists but does not match indexer criteria, updating", logPrefix, upstream.Name)
			_, err = h.etcdSnapshotController.Update(upstream)
		}
		return downstream, err
	} else if len(upstreamSnapshots) > 1 {
		logrus.Warnf("%s multiple snapshots objects found for snapshot %s", logPrefix, downstream.Name)

		var errs []error
		for _, upstreamSnapshot := range upstreamSnapshots {
			logrus.Infof("%s deleting snapshot object %s", logPrefix, upstreamSnapshot.Name)

			err = h.etcdSnapshotController.Delete(upstreamSnapshot.Namespace, upstreamSnapshot.Name, &metav1.DeleteOptions{})
			if err != nil {
				logrus.Errorf("%s error deleting snapshot %s: %v", logPrefix, upstreamSnapshot.Name, err)
				errs = append(errs, err)
			}
		}

		if len(errs) > 0 {
			return downstream, errors.Join(errs...)
		}
		// re-enqueue to ensure the correct snapshot is regenerated
		h.etcdSnapshotFileController.Enqueue(downstream.Name)
		return downstream, nil
	}

	upstream := upstreamSnapshots[0]

	// generate patch
	generated, err := h.populateUpstreamSnapshotFromDownstream(upstream, downstream, cluster)
	if err != nil {
		return downstream, err
	}

	// only patch if something has actually changed
	if reflect.DeepEqual(generated, upstream) {
		return downstream, nil
	}
	logrus.Debugf("%s updating snapshot %s", logPrefix, upstream.Name)

	original, err := json.Marshal(upstream)
	if err != nil {
		return downstream, err
	}

	target, err := json.Marshal(generated)
	if err != nil {
		return downstream, err
	}

	patch, err := jsonpatch.CreateMergePatch(original, target)
	if err != nil {
		return downstream, err
	}

	_, err = h.etcdSnapshotController.Patch(upstream.Namespace, upstream.Name, types.MergePatchType, patch)
	return downstream, err
}

// generateSafeSnapshotName generates a resource-safe name for an etcd snapshot,
// following the same logic as k3s/pkg/etcd/snapshot.(*File).GenerateName
func generateSafeSnapshotName(spec k3s.ETCDSnapshotSpec, createdAt time.Time) string {
	name := strings.ToLower(spec.SnapshotName)

	storage := Local
	if spec.S3 != nil {
		storage = S3
	}

	nodeName := spec.NodeName
	digest := sha256.Sum256([]byte(nodeName + spec.Location))
	hex6 := hex.EncodeToString(digest[:])[:6]

	// reserve space for the non-name parts that will be added:
	// - storage string (worst-case "local" length)
	// - two hyphens around the name
	// - 6-char hex suffix
	reservedSuffixLen := len(string(Local)) + 2 + len(hex6)

	if errs := validation.IsDNS1123Subdomain(name); len(errs) != 0 || len(name)+reservedSuffixLen > validation.DNS1123SubdomainMaxLength {
		shortHost, _, _ := strings.Cut(nodeName, ".")
		name = fmt.Sprintf("etcd-snapshot-%s-%d", shortHost, createdAt.Unix())
	}

	return fmt.Sprintf("%s-%s-%s", storage, name, hex6)
}

// getRestoreModesAnnotation determines the appropriate value for the restore-mode-options annotation
// by checking for a valid, parsable provisioning-cluster-spec and the presence of
// fields required for each restore mode.
func getRestoreModesAnnotation(downstream *k3s.ETCDSnapshotFile, cluster *unstructured.Unstructured) string {
	logPrefix := getLogPrefix(cluster)
	availableModes := []string{rkev1.RestoreRKEConfigNone}

	if downstream.Spec.Metadata == nil {
		logrus.Warnf("%s: downstream snapshot %s/%s has nil metadata, setting restore mode to 'none'",
			logPrefix, downstream.Namespace, downstream.Name)
		return rkev1.RestoreRKEConfigNone
	}

	specPayload, ok := downstream.Spec.Metadata[rkev1.SnapshotMetadataClusterSpecKey]
	if !ok || specPayload == "" {
		logrus.Warnf("%s: downstream snapshot %s/%s is missing '%s' key in metadata or key is empty, setting restore mode to 'none'",
			logPrefix, downstream.Namespace, downstream.Name, rkev1.SnapshotMetadataClusterSpecKey)
		return rkev1.RestoreRKEConfigNone
	}

	clusterSpec, err := snapshotutil.DecompressClusterSpec(specPayload)
	if err != nil {
		logrus.Warnf("%s: downstream snapshot %s/%s contains an unparsable '%s' metadata payload: %v. Setting restore mode to 'none'",
			logPrefix,
			downstream.Namespace,
			downstream.Name,
			rkev1.SnapshotMetadataClusterSpecKey,
			err)
		return rkev1.RestoreRKEConfigNone
	}

	if clusterSpec.KubernetesVersion != "" {
		availableModes = append(availableModes, rkev1.RestoreRKEConfigKubernetesVersion)
	} else {
		logrus.Warnf("%s: downstream snapshot %s/%s has parsable metadata but is missing 'kubernetesVersion', 'kubernetesVersion' restore mode will be unavailable.",
			logPrefix, downstream.Namespace, downstream.Name)
	}

	if clusterSpec.KubernetesVersion != "" && clusterSpec.RKEConfig != nil {
		availableModes = append(availableModes, rkev1.RestoreRKEConfigAll)
	} else {
		logrus.Warnf("%s: downstream snapshot %s/%s is missing 'KubernetesVersion' or 'RKEConfig', 'all' restore mode will be unavailable.",
			logPrefix, downstream.Namespace, downstream.Name)
	}

	return strings.Join(availableModes, ",")
}

// populateUpstreamSnapshotFromDownstream sets the labels, annotations, spec and status fields which are governed by the
// downstream snapshot. Also sets the relevant owner references (machine for local, capi cluster for s3), and
// namespace/name if the snapshot is being created.
func (h *handler) populateUpstreamSnapshotFromDownstream(
	upstream *rkev1.ETCDSnapshot,
	downstream *k3s.ETCDSnapshotFile,
	cluster *unstructured.Unstructured,
) (*rkev1.ETCDSnapshot, error) {
	storage := S3
	if downstream.Spec.S3 == nil {
		storage = Local
	}

	namespace := cluster.GetNamespace()
	if namespace == "" {
		namespace = cluster.GetName()
	}

	genBase := generateSafeSnapshotName(downstream.Spec, downstream.Status.CreationTime.Time)
	snapshotName := name.SafeConcatName(cluster.GetName(), genBase)

	if upstream == nil {
		upstream = &rkev1.ETCDSnapshot{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      snapshotName,
			},
		}
	} else {
		upstream = upstream.DeepCopy()
	}

	if upstream.Labels == nil {
		upstream.Labels = map[string]string{}
	}
	upstream.Labels[capr.ClusterNameLabel] = cluster.GetName()

	if upstream.Annotations == nil {
		upstream.Annotations = map[string]string{}
	}

	upstream.Annotations[RestoreModeOptionsAnnotation] = getRestoreModesAnnotation(downstream, cluster)
	upstream.Annotations[StorageAnnotationKey] = string(storage)
	upstream.Annotations[SnapshotFileNameAnnotationKey] = downstream.Spec.SnapshotName
	upstream.Annotations[capr.SnapshotNameAnnotation] = downstream.Name

	upstream.Spec.ClusterName = cluster.GetName()
	upstream.SnapshotFile = rkev1.ETCDSnapshotFile{
		Name:      downstream.Spec.SnapshotName,
		Location:  downstream.Spec.Location,
		NodeName:  downstream.Spec.NodeName,
		CreatedAt: downstream.Status.CreationTime,
	}

	b, err := json.Marshal(&downstream.Spec.Metadata)
	if err != nil {
		return nil, err
	}
	upstream.SnapshotFile.Metadata = base64.StdEncoding.EncodeToString(b)

	if downstream.Status.Error != nil && downstream.Status.Error.Message != nil {
		upstream.SnapshotFile.Message = *downstream.Status.Error.Message
	}
	if downstream.Status.Size != nil {
		upstream.SnapshotFile.Size, _ = downstream.Status.Size.AsInt64()
	}
	if downstream.Status.ReadyToUse != nil && *downstream.Status.ReadyToUse {
		upstream.SnapshotFile.Status = "successful"
	} else {
		upstream.SnapshotFile.Status = "failed"
	}

	if storage == Local {
		if len(upstream.OwnerReferences) == 0 {
			// make API call to downstream cluster to find node object, and extract lifecycle labels
			node, err := h.nodeCache.Get(downstream.Spec.NodeName)
			if err != nil {
				logrus.Errorf("error getting node %s for snapshot %s/%s: %v", downstream.Spec.NodeName, namespace, snapshotName, err)
				return nil, err
			}

			ref, err := MachineLifecycleLabelsToObjectReference(node)
			if err != nil {
				logrus.Errorf("error getting node reference for snapshot %s/%s: %v", namespace, snapshotName, err)
				return nil, err
			}

			o, err := h.dynamic.Get(ref.GroupVersionKind(), ref.Namespace, ref.Name)
			if err != nil {
				logrus.Errorf("error getting node %s for snapshot %s/%s: %v", downstream.Spec.NodeName, namespace, snapshotName, err)
				return nil, err
			}

			metaObj, err := meta.Accessor(o)
			if err != nil {
				logrus.Errorf("error getting node %s for snapshot %s/%s: %v", downstream.Spec.NodeName, namespace, snapshotName, err)
				return nil, err
			}

			// set owner reference to local cluster machine representation
			upstream.OwnerReferences = []metav1.OwnerReference{
				{
					APIVersion: ref.APIVersion,
					Kind:       ref.Kind,
					Name:       ref.Name,
					UID:        metaObj.GetUID(),
				},
			}
		}
		upstream.Labels[capr.NodeNameLabel] = downstream.Spec.NodeName
	} else {
		upstream.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: cluster.GetAPIVersion(),
				Kind:       cluster.GetKind(),
				Name:       cluster.GetName(),
				UID:        cluster.GetUID(),
			},
		}
		upstream.SnapshotFile.S3 = &rkev1.ETCDSnapshotS3{
			Endpoint:      downstream.Spec.S3.Endpoint,
			EndpointCA:    downstream.Spec.S3.EndpointCA,
			SkipSSLVerify: downstream.Spec.S3.SkipSSLVerify,
			Bucket:        downstream.Spec.S3.Bucket,
			Region:        downstream.Spec.S3.Region,
			Folder:        downstream.Spec.S3.Prefix,
		}
	}

	return upstream, nil
}

// getCluster returns the provisioning cluster associated with the current userContext.
func (h *handler) getCluster() (*unstructured.Unstructured, error) {
	obj, err := h.dynamic.Get(h.clusterRef.GroupVersionKind(), h.clusterRef.Namespace, h.clusterRef.Name)
	if err != nil {
		return nil, err
	}
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	return &unstructured.Unstructured{Object: u}, nil
}

// getSnapshotsFromSnapshotFile returns all snapshots objects for the given cluster for the downstream snapshotfile object.
// During normal operation, this will return either 0 snapshots, indicating that the snapshot has not been reconciled yet,
// or 1 snapshot. While multiple snapshots being returned is possible via user intervention via manually editing snapshots,
// this is an edge case and results in all local snapshot objects being deleted and the downstream snapshot being
// re-enqueued for regeneration.
func (h *handler) getSnapshotsFromSnapshotFile(cluster *unstructured.Unstructured, snapshotFile *k3s.ETCDSnapshotFile) ([]*rkev1.ETCDSnapshot, error) {
	namespace := cluster.GetNamespace()
	if namespace == "" {
		namespace = cluster.GetName()
	}
	snapshots, err := h.etcdSnapshotCache.GetByIndex(provcluster.ByETCDSnapshotName, fmt.Sprintf("%s/%s/%s", namespace, cluster.GetName(), snapshotFile.Name))
	if err != nil {
		return nil, err
	}
	logrus.Debugf("Got snapshots from snapshot file")
	return snapshots, nil
}

func getLogPrefix(cluster *unstructured.Unstructured) string {
	suffix := cluster.GetName()
	if cluster.GetNamespace() != "" {
		suffix = cluster.GetNamespace() + "/" + suffix
	}
	suffix = schema.FromAPIVersionAndKind(cluster.GetAPIVersion(), cluster.GetKind()).String() + "/" + suffix
	return fmt.Sprintf("[snapshotbackpopulate] %s:", suffix)
}

func (h *handler) hasMachineLifecycleLabels(upstream *rkev1.ETCDSnapshot) bool {
	if upstream.Labels == nil {
		return false
	}
	group := upstream.Labels[planv1alpha1.MachineLifecycleGroup]
	if group == "" {
		return false
	}
	version := upstream.Labels[planv1alpha1.MachineLifecycleVersion]
	if version == "" {
		return false
	}
	kind := upstream.Labels[planv1alpha1.MachineLifecycleKind]
	if kind == "" {
		return false
	}
	// theoretically could be a non-namespaced resource, but in practice this doesn't exist
	namespace := upstream.Labels[planv1alpha1.MachineLifecycleNamespace]
	if namespace == "" {
		return false
	}
	name := upstream.Labels[planv1alpha1.MachineLifecycleName]
	return name != ""
}

func MachineLifecycleLabelsToObjectReference(obj metav1.Object) (*corev1.ObjectReference, error) {
	prefix := fmt.Sprintf("object %s", obj.GetName())
	if obj.GetNamespace() != "" {
		prefix = fmt.Sprintf("object %s/%s", obj.GetNamespace(), obj.GetName())
	}

	labels := obj.GetLabels()
	if labels == nil {
		return nil, fmt.Errorf("%s has no labels", prefix)
	}

	group := labels[planv1alpha1.MachineLifecycleGroup]
	if group == "" {
		return nil, fmt.Errorf("%s has no group label", prefix)
	}

	version := labels[planv1alpha1.MachineLifecycleVersion]
	if version == "" {
		return nil, fmt.Errorf("%s has no version label", prefix)
	}

	kind := labels[planv1alpha1.MachineLifecycleKind]
	if kind == "" {
		return nil, fmt.Errorf("%s has no kind label", prefix)
	}

	namespace := labels[planv1alpha1.MachineLifecycleNamespace]
	if namespace == "" {
		return nil, fmt.Errorf("%s has no namespace label", prefix)
	}

	name := labels[planv1alpha1.MachineLifecycleName]
	if name == "" {
		return nil, fmt.Errorf("%s has no name label", prefix)
	}

	gvr := schema.GroupVersionKind{Group: group, Version: version, Kind: kind}
	return &corev1.ObjectReference{
		APIVersion: gvr.GroupVersion().String(),
		Kind:       gvr.Kind,
		Name:       name,
		Namespace:  namespace,
	}, nil
}
