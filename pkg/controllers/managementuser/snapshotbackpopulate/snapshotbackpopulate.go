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
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta2"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	rkev1controllers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	nodepkg "github.com/rancher/rancher/pkg/node"
	planapi "github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
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
	clusterRef        corev1.ObjectReference
	ownerRef          metav1.OwnerReference
	snapshotNamespace string

	dynamic                dynamicClient
	restMapper             meta.RESTMapper
	etcdSnapshotCache      rkev1controllers.ETCDSnapshotCache
	etcdSnapshotController rkev1controllers.ETCDSnapshotController
	beaconCache            plancontrollers.BeaconCache
	capiClusterCache       capicontrollers.ClusterCache
	capiMachineCache       capicontrollers.MachineCache
	mgmtNodeCache          mgmtcontrollers.NodeCache

	nodeCache                  corecontrollers.NodeCache
	etcdSnapshotFileController k3scontrollers.ETCDSnapshotFileController
	etcdSnapshotFileCache      k3scontrollers.ETCDSnapshotFileCache
}

// Register sets up the v2provisioning snapshot backpopulate controller. This controller is responsible for monitoring
// the downstream etcd-snapshots configmap and backpopulating snapshots into etcd snapshot objects in the management cluster.
func Register(ctx context.Context, userContext *config.UserContext, capiCtx *wrangler.CAPIContext, cluster *apimgmtv3.Cluster) {
	logrus.Debugf("[snapshotbackpopulate] Registering controller for cluster %s", userContext.ClusterName)
	h := handler{
		dynamic:                    userContext.Management.Wrangler.Dynamic,
		restMapper:                 userContext.Management.Wrangler.RESTMapper,
		etcdSnapshotCache:          userContext.Management.Wrangler.RKE.ETCDSnapshot().Cache(),
		etcdSnapshotController:     userContext.Management.Wrangler.RKE.ETCDSnapshot(),
		beaconCache:                userContext.Management.Wrangler.Plan.Beacon().Cache(),
		capiClusterCache:           capiCtx.CAPI.Cluster().Cache(),
		capiMachineCache:           capiCtx.CAPI.Machine().Cache(),
		mgmtNodeCache:              userContext.Management.Wrangler.Mgmt.Node().Cache(),
		nodeCache:                  userContext.Corew.Node().Cache(),
		etcdSnapshotFileController: userContext.K3s.V1().ETCDSnapshotFile(),
		etcdSnapshotFileCache:      userContext.K3s.V1().ETCDSnapshotFile().Cache(),
	}

	switch {
	case cluster.Annotations["provisioning.cattle.io/administrated"] == "true":
		provCluster, err := userContext.Management.Wrangler.Provisioning.Cluster().Cache().GetByIndex(provcluster.ByCluster, cluster.Name)
		if err != nil {
			logrus.Errorf("error getting provisioning cluster %s: %v", cluster.Name, err)
			return
		}
		if len(provCluster) != 1 {
			logrus.Errorf("expected 1 provisioning cluster for cluster %s, got %d", cluster.Name, len(provCluster))
			return
		}
		h.clusterRef = corev1.ObjectReference{
			APIVersion: provCluster[0].APIVersion,
			Kind:       provCluster[0].Kind,
			Namespace:  provCluster[0].GetNamespace(),
			Name:       provCluster[0].GetName(),
		}
		h.snapshotNamespace = h.clusterRef.Namespace
	case cluster.Labels[capr.CAPIClusterOwnerLabel] != "" && cluster.Labels[capr.CAPIClusterOwnerNSLabel] != "":
		// Turtles-imported CAPI cluster: the mgmt cluster is a shell whose labels back-reference
		// the real CAPI Cluster. Use that as the cluster ref so beacon lookups, snapshot owner
		// references, and lifecycle labels resolve against the CAPI-native object graph.
		h.clusterRef = corev1.ObjectReference{
			APIVersion: capi.GroupVersion.String(),
			Kind:       "Cluster",
			Namespace:  cluster.Labels[capr.CAPIClusterOwnerNSLabel],
			Name:       cluster.Labels[capr.CAPIClusterOwnerLabel],
		}
		h.snapshotNamespace = cluster.Name
		h.ownerRef = metav1.OwnerReference{
			APIVersion: cluster.APIVersion,
			Kind:       cluster.Kind,
			Name:       cluster.Name,
			UID:        cluster.UID,
		}
	default:
		h.clusterRef = corev1.ObjectReference{
			APIVersion: apimgmtv3.SchemeGroupVersion.String(),
			Kind:       apimgmtv3.Kind("Cluster").Kind,
			Name:       cluster.Name,
		}
		h.snapshotNamespace = cluster.Name
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
	if !planapi.AuthorizedForBeacon(beacon, "") {
		h.etcdSnapshotController.EnqueueAfter(snapshot.Namespace, snapshot.Name, 1*time.Minute)
		return snapshot, nil
	}

	if snapshot.Namespace != h.snapshotNamespace || snapshot.Labels == nil || snapshot.Labels[capr.ClusterNameLabel] != h.snapshotClusterName(cluster) {
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
	if !planapi.AuthorizedForBeacon(beacon, "") {
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

	clusterName := h.snapshotClusterName(cluster)
	genBase := generateSafeSnapshotName(downstream.Spec, downstream.Status.CreationTime.Time)
	snapshotName := name.SafeConcatName(clusterName, genBase)

	if upstream == nil {
		upstream = &rkev1.ETCDSnapshot{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: h.snapshotNamespace,
				Name:      snapshotName,
			},
		}
	} else {
		upstream = upstream.DeepCopy()
	}

	if upstream.Labels == nil {
		upstream.Labels = map[string]string{}
	}
	upstream.Labels[capr.ClusterNameLabel] = clusterName

	if upstream.Annotations == nil {
		upstream.Annotations = map[string]string{}
	}

	upstream.Annotations[RestoreModeOptionsAnnotation] = getRestoreModesAnnotation(downstream, cluster)
	upstream.Annotations[StorageAnnotationKey] = string(storage)
	upstream.Annotations[SnapshotFileNameAnnotationKey] = downstream.Spec.SnapshotName
	upstream.Annotations[capr.SnapshotNameAnnotation] = downstream.Name

	upstream.Spec.ClusterName = clusterName
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

	var (
		ownerRef        metav1.OwnerReference
		lifecycleLabels map[string]string
	)
	if storage == Local {
		ownerRef, lifecycleLabels, err = h.snapshotOwnerAndLabelsForLocal(downstream.Spec.NodeName)
		if err != nil {
			logrus.Errorf("error resolving snapshot owner for node %s / snapshot %s/%s: %v", downstream.Spec.NodeName, h.snapshotNamespace, snapshotName, err)
			return nil, err
		}
		if len(upstream.OwnerReferences) == 0 {
			upstream.OwnerReferences = []metav1.OwnerReference{ownerRef}
		}
		upstream.Labels[capr.NodeNameLabel] = downstream.Spec.NodeName
	} else {
		ownerRef, lifecycleLabels, err = h.snapshotOwnerAndLabelsForS3(cluster)
		if err != nil {
			logrus.Errorf("error resolving snapshot owner for s3 snapshot %s/%s: %v", h.snapshotNamespace, snapshotName, err)
			return nil, err
		}
		upstream.OwnerReferences = []metav1.OwnerReference{ownerRef}
		upstream.SnapshotFile.S3 = &rkev1.ETCDSnapshotS3{
			Endpoint:      downstream.Spec.S3.Endpoint,
			EndpointCA:    downstream.Spec.S3.EndpointCA,
			SkipSSLVerify: downstream.Spec.S3.SkipSSLVerify,
			Bucket:        downstream.Spec.S3.Bucket,
			Region:        downstream.Spec.S3.Region,
			Folder:        downstream.Spec.S3.Prefix,
		}
	}
	for k, v := range lifecycleLabels {
		upstream.Labels[k] = v
	}

	return upstream, nil
}

// snapshotOwnerAndLabelsForLocal returns the OwnerReference and any extra lifecycle labels that a
// local (non-S3) etcd snapshot from the given downstream node should carry.
//
//   - CAPI-native / CAPRKE2 (h.clusterRef is a cluster.x-k8s.io Cluster): the owner is the mgmt v3
//     Node whose LabelNodeName matches nodeName. Additionally, ClusterLifecycle labels (from the
//     CAPI Cluster) and MachineLifecycle labels (from the CAPI Machine whose NodeRef matches
//     nodeName) are stamped so reconcileRestore can correlate against machine-plan secrets — plan
//     secrets on CAPRKE2 clusters are labelled with the CAPI Machine's identity, not the v3 Node's.
//
//   - Otherwise (v2prov / imported RKE2/K3s): read the downstream Node's MachineLifecycle labels
//     and dereference to whatever machine object they name (CAPI Machine for v2prov, mgmt v3 Node
//     for imported). No extra labels are needed — the plan-secret and snapshot correlation already
//     works via the OwnerReferences path.
func (h *handler) snapshotOwnerAndLabelsForLocal(nodeName string) (metav1.OwnerReference, map[string]string, error) {
	if h.clusterRef.APIVersion == capi.GroupVersion.String() && h.clusterRef.Kind == "Cluster" {
		machines, err := h.capiMachineCache.List(h.clusterRef.Namespace, labels.SelectorFromSet(labels.Set{
			capi.ClusterNameLabel: h.clusterRef.Name,
		}))
		if err != nil {
			return metav1.OwnerReference{}, nil, err
		}
		var capiMachine *capi.Machine
		for _, m := range machines {
			if m.Status.NodeRef.IsDefined() && m.Status.NodeRef.Name == nodeName {
				capiMachine = m
				break
			}
		}
		if capiMachine == nil {
			return metav1.OwnerReference{}, nil, fmt.Errorf("no CAPI Machine in %s (cluster %s) has NodeRef.Name=%s",
				h.clusterRef.Namespace, h.clusterRef.Name, nodeName)
		}

		mgmtNodes, err := h.mgmtNodeCache.List(h.snapshotNamespace, labels.SelectorFromSet(labels.Set{
			nodepkg.LabelNodeName: nodeName,
		}))
		if err != nil {
			return metav1.OwnerReference{}, nil, err
		}
		if len(mgmtNodes) == 0 {
			return metav1.OwnerReference{}, nil, fmt.Errorf("no mgmt v3 Node with %s=%s in namespace %s",
				nodepkg.LabelNodeName, nodeName, h.snapshotNamespace)
		}
		mgmtNode := mgmtNodes[0]

		lifecycleLabels, err := h.caprke2LifecycleLabels(capiMachine)
		if err != nil {
			return metav1.OwnerReference{}, nil, err
		}
		return metav1.OwnerReference{
			APIVersion: apimgmtv3.SchemeGroupVersion.String(),
			Kind:       "Node",
			Name:       mgmtNode.Name,
			UID:        mgmtNode.UID,
		}, lifecycleLabels, nil
	}

	node, err := h.nodeCache.Get(nodeName)
	if err != nil {
		return metav1.OwnerReference{}, nil, err
	}
	// Derive the mgmt-side namespace from the clusterRef. For imported RKE2/K3s the clusterRef
	// points at a cluster-scoped mgmt v3 Cluster, and its mgmt v3 Node lives in the namespace
	// named after the cluster. For v2prov the clusterRef is namespace-scoped and the CAPI
	// Machine lives alongside it. (The CAPI-native branch is handled above and never reaches here.)
	machineNamespace := h.clusterRef.Namespace
	if machineNamespace == "" {
		machineNamespace = h.clusterRef.Name
	}
	ref, err := planv1alpha1.MachineLifecycleLabelsToObjectReference(node, machineNamespace, h.restMapper)
	if err != nil {
		return metav1.OwnerReference{}, nil, err
	}
	o, err := h.dynamic.Get(ref.GroupVersionKind(), ref.Namespace, ref.Name)
	if err != nil {
		return metav1.OwnerReference{}, nil, err
	}
	metaObj, err := meta.Accessor(o)
	if err != nil {
		return metav1.OwnerReference{}, nil, err
	}
	return metav1.OwnerReference{
		APIVersion: ref.APIVersion,
		Kind:       ref.Kind,
		Name:       ref.Name,
		UID:        metaObj.GetUID(),
	}, nil, nil
}

// snapshotOwnerAndLabelsForS3 returns the OwnerReference and any extra lifecycle labels that an
// S3 snapshot should carry. For turtles-imported CAPRKE2 clusters the owner is the mgmt v3
// Cluster (populated on h.ownerRef during Register) and ClusterLifecycle labels are stamped so
// reconcileRestore can locate the snapshot regardless of its namespace. For every other cluster
// type the owner is the object returned by getCluster (provv1.Cluster for v2prov, mgmt v3 Cluster
// for imported) and no extra labels are needed.
func (h *handler) snapshotOwnerAndLabelsForS3(cluster *unstructured.Unstructured) (metav1.OwnerReference, map[string]string, error) {
	if h.ownerRef.Name != "" {
		clusterLabels, err := h.caprke2ClusterLifecycleLabels()
		if err != nil {
			return metav1.OwnerReference{}, nil, err
		}
		return h.ownerRef, clusterLabels, nil
	}
	return metav1.OwnerReference{
		APIVersion: cluster.GetAPIVersion(),
		Kind:       cluster.GetKind(),
		Name:       cluster.GetName(),
		UID:        cluster.GetUID(),
	}, nil, nil
}

// snapshotClusterName returns the user-facing cluster name that should be stamped on the
// snapshot (via capr.ClusterNameLabel and Spec.ClusterName) and used for indexer keys. For
// turtles-imported CAPRKE2 clusters this is the mgmt v3 Cluster's name — the UI keys its
// day-2 views off the mgmt cluster, so the CAPI Cluster name (returned by getCluster) is not
// visible to the user. For every other cluster type the caller's cluster object already carries
// the user-facing name.
func (h *handler) snapshotClusterName(cluster *unstructured.Unstructured) string {
	if h.ownerRef.Name != "" {
		return h.ownerRef.Name
	}
	return cluster.GetName()
}

// caprke2ClusterLifecycleLabels resolves the CAPI Cluster referenced by h.clusterRef and returns
// its lifecycle labels. Reserved for the CAPRKE2 dispatch path where h.clusterRef points at a
// CAPI Cluster.
func (h *handler) caprke2ClusterLifecycleLabels() (map[string]string, error) {
	capiCluster, err := h.capiClusterCache.Get(h.clusterRef.Namespace, h.clusterRef.Name)
	if err != nil {
		return nil, err
	}
	capiCluster = capiCluster.DeepCopy()
	capiCluster.TypeMeta = metav1.TypeMeta{Kind: "Cluster", APIVersion: capi.GroupVersion.String()}
	return planv1alpha1.ObjToClusterLifecycleLabels(capiCluster)
}

// caprke2LifecycleLabels returns the merged Cluster+Machine lifecycle labels used to stamp a
// CAPRKE2 local snapshot. The Machine labels are what reconcileRestore matches against
// machine-plan secret labels; the Cluster labels are stamped for symmetry with plan secrets.
func (h *handler) caprke2LifecycleLabels(capiMachine *capi.Machine) (map[string]string, error) {
	clusterLabels, err := h.caprke2ClusterLifecycleLabels()
	if err != nil {
		return nil, err
	}
	machine := capiMachine.DeepCopy()
	machine.TypeMeta = metav1.TypeMeta{Kind: "Machine", APIVersion: capi.GroupVersion.String()}
	machineLabels, err := planv1alpha1.ObjToMachineLifecycleLabels(machine)
	if err != nil {
		return nil, err
	}
	merged := make(map[string]string, len(clusterLabels)+len(machineLabels))
	for k, v := range clusterLabels {
		merged[k] = v
	}
	for k, v := range machineLabels {
		merged[k] = v
	}
	return merged, nil
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
	// The index key must match the namespace snapshots are actually written to, which is
	// h.snapshotNamespace — for CAPRKE2 this is the mgmt cluster ns, not the CAPI Cluster's ns
	// (cluster.GetNamespace()).
	snapshots, err := h.etcdSnapshotCache.GetByIndex(provcluster.ByETCDSnapshotName, fmt.Sprintf("%s/%s/%s", h.snapshotNamespace, h.snapshotClusterName(cluster), snapshotFile.Name))
	if err != nil {
		return nil, err
	}
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
