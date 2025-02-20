package snapshotbackpopulate

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	jsonpatch "github.com/evanphx/json-patch/v5"
	k3s "github.com/rancher/rancher/pkg/apis/k3s.cattle.io/v1"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	cluster2 "github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	k3scontrollers "github.com/rancher/rancher/pkg/generated/controllers/k3s.cattle.io/v1"
	managementcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	provisioningcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkev1controllers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	normancorev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"regexp"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"strings"
	"time"
)

var (
	InvalidKeyChars = regexp.MustCompile(`[^-.a-zA-Z0-9]`)
)

const (
	StorageAnnotationKey = "etcdsnapshot.rke.io/storage"
)

type Storage string

const (
	S3    Storage = "s3"
	Local Storage = "local"
)

type handler struct {
	clusterName                string
	clusterCache               provisioningcontrollers.ClusterCache
	clusters                   provisioningcontrollers.ClusterClient
	mgmtClusterCache           managementcontrollers.ClusterCache
	controlPlaneCache          rkev1controllers.RKEControlPlaneCache
	etcdSnapshotCache          rkev1controllers.ETCDSnapshotCache
	etcdSnapshotController     rkev1controllers.ETCDSnapshotController
	machineCache               capicontrollers.MachineCache
	capiClusterCache           capicontrollers.ClusterCache
	nodeController             normancorev1.NodeInterface
	etcdSnapshotFileController k3scontrollers.ETCDSnapshotFileController
	etcdSnapshotFileCache      k3scontrollers.ETCDSnapshotFileCache
	v1ClusterNamespace         string
	v1ClusterName              string
}

// Register sets up the v2provisioning snapshot backpopulate controller. This controller is responsible for monitoring
// the downstream etcd-snapshots configmap and backpopulating snapshots into etcd snapshot objects in the management cluster.
func Register(ctx context.Context, userContext *config.UserContext) {
	h := handler{
		clusterName:                userContext.ClusterName,
		clusterCache:               userContext.Management.Wrangler.Provisioning.Cluster().Cache(),
		clusters:                   userContext.Management.Wrangler.Provisioning.Cluster(),
		mgmtClusterCache:           userContext.Management.Wrangler.Mgmt.Cluster().Cache(),
		controlPlaneCache:          userContext.Management.Wrangler.RKE.RKEControlPlane().Cache(),
		etcdSnapshotCache:          userContext.Management.Wrangler.RKE.ETCDSnapshot().Cache(),
		etcdSnapshotController:     userContext.Management.Wrangler.RKE.ETCDSnapshot(),
		machineCache:               userContext.Management.Wrangler.CAPI.Machine().Cache(),
		capiClusterCache:           userContext.Management.Wrangler.CAPI.Cluster().Cache(),
		nodeController:             userContext.Core.Nodes(""),
		etcdSnapshotFileController: userContext.K3s.V1().ETCDSnapshotFile(),
		etcdSnapshotFileCache:      userContext.K3s.V1().ETCDSnapshotFile().Cache(),
	}

	userContext.Management.Wrangler.RKE.ETCDSnapshot().OnChange(ctx, "snapshotbackpopulate", h.OnUpstreamChange)
	userContext.K3s.V1().ETCDSnapshotFile().OnChange(ctx, "downstreamsnapshotbackpopulate", h.OnDownstreamChange)

	relatedresource.Watch(ctx, "snapshot-reconcile-downstream-deletion", func(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
		if snapshot, ok := obj.(*k3s.ETCDSnapshotFile); ok {
			if snapshot.DeletionTimestamp == nil {
				return nil, nil
			}
			upstreamSnapshots, err := h.etcdSnapshotCache.List(h.v1ClusterNamespace, labels.SelectorFromSet(labels.Set{capr.ClusterNameLabel: h.v1ClusterName}))
			if err != nil {
				return nil, err
			}
			for _, upstreamSnapshot := range upstreamSnapshots {
				if upstreamSnapshot.Annotations != nil && upstreamSnapshot.Annotations[capr.SnapshotNameAnnotation] == snapshot.Name {
					return []relatedresource.Key{
						{
							Namespace: upstreamSnapshot.Namespace,
							Name:      upstreamSnapshot.Name,
						},
					}, nil
				}
			}
		}
		return nil, nil
	}, userContext.Management.Wrangler.RKE.ETCDSnapshot(), userContext.K3s.V1().ETCDSnapshotFile())

	//relatedresource.WatchClusterScoped(ctx, "", func(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
	//	if snapshot, ok := obj.(*rkev1.ETCDSnapshot); ok {
	//		if snapshot.Annotations != nil && snapshot.Annotations[capr.SnapshotNameAnnotation] != "" {
	//			return []relatedresource.Key{
	//				{
	//					Name: snapshot.Annotations[capr.SnapshotNameAnnotation],
	//				},
	//			}, nil
	//		}
	//	}
	//	return nil, nil
	//}, userContext.K3s.V1().ETCDSnapshotFile(), userContext.Management.Wrangler.RKE.ETCDSnapshot())
}

func getSnapshotStorageNodeLabel(controlPlane *rkev1.RKEControlPlane) string {
	return fmt.Sprintf("etcd.%s.cattle.io/snapshot/storage-node", capr.GetRuntime(controlPlane.Spec.KubernetesVersion))
}

func (h *handler) OnUpstreamChange(_ string, snapshot *rkev1.ETCDSnapshot) (*rkev1.ETCDSnapshot, error) {
	if snapshot == nil {
		return nil, nil
	}

	clusters, err := h.clusterCache.GetByIndex(cluster2.ByCluster, h.clusterName)
	if err != nil || len(clusters) != 1 {
		return snapshot, fmt.Errorf("error while retrieving cluster %s from cache via index: %w", h.clusterName, err)
	}

	cluster := clusters[0]
	if h.v1ClusterName == "" {
		h.v1ClusterName = cluster.Name
	}

	//logPrefix := fmt.Sprintf("[snapshotbackpopulate] rkecluster %s/%s:", cluster.Namespace, cluster.Name)

	controlPlane, err := h.controlPlaneCache.Get(cluster.Namespace, cluster.Name)
	if err != nil {
		return snapshot, err
	}

	// if controlplane is not currently performing a restore, and the node responsible for the snapshot has reconciled since
	if controlPlane.Spec.ETCDSnapshotRestore != nil && controlPlane.Status.ETCDSnapshotRestore != nil &&
		controlPlane.Spec.ETCDSnapshotRestore.Generation != controlPlane.Status.ETCDSnapshotRestore.Generation {
		h.etcdSnapshotController.EnqueueAfter(snapshot.Namespace, snapshot.Name, 1*time.Minute)
		return snapshot, nil
	}

	if snapshot.Annotations == nil || snapshot.Annotations[capr.SnapshotNameAnnotation] == "" {
		return nil, h.etcdSnapshotController.Delete(snapshot.Namespace, snapshot.Name, &metav1.DeleteOptions{})
	}

	_, err = h.etcdSnapshotFileCache.Get(snapshot.Annotations[capr.SnapshotNameAnnotation])
	if apierrors.IsNotFound(err) {
		// If the downstream snapshot does not exist in the downstream cluster, delete the local version
		return nil, h.etcdSnapshotController.Delete(snapshot.Namespace, snapshot.Name, &metav1.DeleteOptions{})
	} else if err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func (h *handler) OnDownstreamChange(_ string, snapshot *k3s.ETCDSnapshotFile) (*k3s.ETCDSnapshotFile, error) {
	if snapshot == nil {
		return nil, nil
	}

	clusters, err := h.clusterCache.GetByIndex(cluster2.ByCluster, h.clusterName)
	if err != nil || len(clusters) != 1 {
		return snapshot, fmt.Errorf("error while retrieving cluster %s from cache via index: %w", h.clusterName, err)
	}

	cluster := clusters[0]
	if h.v1ClusterName == "" {
		h.v1ClusterName = cluster.Name
	}

	logPrefix := fmt.Sprintf("[snapshotbackpopulate] rkecluster %s/%s:", cluster.Namespace, cluster.Name)

	controlPlane, err := h.controlPlaneCache.Get(cluster.Namespace, cluster.Name)
	if err != nil {
		return snapshot, err
	}
	// if controlplane is not currently performing a restore, and the node responsible for the snapshot has reconciled since
	if controlPlane.Spec.ETCDSnapshotRestore != nil && controlPlane.Status.ETCDSnapshotRestore != nil &&
		controlPlane.Spec.ETCDSnapshotRestore.Generation != controlPlane.Status.ETCDSnapshotRestore.Generation {
		h.etcdSnapshotController.EnqueueAfter(snapshot.Namespace, snapshot.Name, 1*time.Minute)
		return snapshot, nil
	}

	capiCluster, err := capr.GetCAPIClusterFromLabel(controlPlane, h.capiClusterCache)
	if err != nil {
		return snapshot, err
	}

	logrus.Infof("%s processing snapshot %s", logPrefix, snapshot.Name)

	var machine *capi.Machine
	labelSet := labels.Set{capr.ClusterNameLabel: cluster.Name}

	if snapshot.Spec.S3 == nil {
		machine, err = capr.GetMachineFromNode(h.machineCache, snapshot.Spec.NodeName, cluster)
		if err != nil {
			return snapshot, err
		}
		labelSet[capr.NodeNameLabel] = machine.Name
	}

	// get upstream snapshot object
	// if upstream snapshot object does not exist, create it
	upstreamSnapshots, err := h.etcdSnapshotCache.List(cluster.Namespace, labels.SelectorFromSet(labelSet))
	if err != nil {
		return snapshot, err
	}

	var upstreamSnapshot *rkev1.ETCDSnapshot
	for _, u := range upstreamSnapshots {
		if u.Annotations[capr.SnapshotFileNameAnnotation] == snapshot.Spec.SnapshotName {
			if upstreamSnapshot != nil {
				return snapshot, fmt.Errorf("found multiple upstream snapshots for downstream snapshot %s", snapshot.Name)
			}
			upstreamSnapshot = u
		}
	}

	if upstreamSnapshot == nil {
		// create snapshot
		name := name.SafeConcatName(cluster.Name, strings.ToLower(InvalidKeyChars.ReplaceAllString(snapshot.Spec.SnapshotName, "-")), string(S3))
		upstream := rkev1.NewETCDSnapshot(cluster.Namespace, name, rkev1.ETCDSnapshot{})
		upstream, err = updateSnapshot(upstream, snapshot, cluster, capiCluster, machine)
		if err != nil {
			return snapshot, err
		}

		_, err = h.etcdSnapshotController.Create(upstream)
		return snapshot, err
	}

	// generate patch
	generated, err := updateSnapshot(upstreamSnapshot.DeepCopy(), snapshot, cluster, capiCluster, machine)
	if err != nil {
		return snapshot, err
	}

	original, err := json.Marshal(upstreamSnapshot)
	if err != nil {
		return snapshot, err
	}

	target, err := json.Marshal(generated)
	if err != nil {
		return snapshot, err
	}

	patch, err := jsonpatch.CreateMergePatch(original, target)
	if err != nil {
		return snapshot, err
	}

	_, err = h.etcdSnapshotController.Patch(upstreamSnapshot.Namespace, upstreamSnapshot.Name, types.MergePatchType, patch)
	return snapshot, err
}

func updateSnapshot(upstream *rkev1.ETCDSnapshot, snapshot *k3s.ETCDSnapshotFile, cluster *provv1.Cluster, capiCluster *capi.Cluster, machine *capi.Machine) (*rkev1.ETCDSnapshot, error) {
	// create snapshot
	b, err := json.Marshal(&snapshot.Spec.Metadata)
	if err != nil {
		return nil, err
	}

	metadata := base64.StdEncoding.EncodeToString(b)
	message := ""
	if snapshot.Status.Error != nil && snapshot.Status.Error.Message != nil {
		message = *snapshot.Status.Error.Message
	}

	size := int64(0)
	if snapshot.Status.Size != nil {
		size, _ = snapshot.Status.Size.AsInt64()
	}

	status := ""
	if snapshot.Status.ReadyToUse != nil && *snapshot.Status.ReadyToUse {
		status = "successful"
	} else {
		status = "failed"
	}

	storage := Local
	if snapshot.Spec.S3 != nil {
		storage = S3
	}

	var s3 *rkev1.ETCDSnapshotS3
	var owner metav1.OwnerReference
	if storage == Local {
		owner, err = capr.ToOwnerReference(machine)
		if err != nil {
			return nil, err
		}
	} else {
		owner, err = capr.ToOwnerReference(capiCluster)
		if err != nil {
			return nil, err
		}
		s3 = &rkev1.ETCDSnapshotS3{
			Endpoint:      snapshot.Spec.S3.Endpoint,
			EndpointCA:    snapshot.Spec.S3.EndpointCA,
			SkipSSLVerify: snapshot.Spec.S3.SkipSSLVerify,
			Bucket:        snapshot.Spec.S3.Bucket,
			Region:        snapshot.Spec.S3.Region,
			Folder:        snapshot.Spec.S3.Prefix,
		}
	}
	if upstream.Labels == nil {
		upstream.Labels = map[string]string{}
	}
	upstream.Labels[capr.ClusterNameLabel] = cluster.Name
	if machine != nil {
		upstream.Labels[capr.MachineIDLabel] = machine.Labels[capr.MachineIDLabel]
	}
	if snapshot.Spec.NodeName != "s3" {
		upstream.Labels[capr.NodeNameLabel] = snapshot.Spec.NodeName
	}

	if upstream.Annotations == nil {
		upstream.Annotations = map[string]string{}
	}
	upstream.Annotations[StorageAnnotationKey] = string(storage)
	upstream.Annotations[capr.SnapshotFileNameAnnotation] = snapshot.Spec.SnapshotName
	upstream.Annotations[capr.SnapshotNameAnnotation] = snapshot.Name

	upstream.OwnerReferences = []metav1.OwnerReference{owner}

	upstream.Spec.ClusterName = cluster.Name
	upstream.SnapshotFile = rkev1.ETCDSnapshotFile{
		Name:      snapshot.Spec.SnapshotName,
		Location:  snapshot.Spec.Location,
		Metadata:  metadata,
		Message:   message,
		NodeName:  snapshot.Spec.NodeName,
		CreatedAt: snapshot.Status.CreationTime,
		Size:      size,
		Status:    status,
		S3:        s3,
	}

	return upstream, nil
}

// ETCDSnapshotS3 holds information about the S3 storage system holding the snapshot.
type ETCDSnapshotS3 struct {
	// Endpoint is the host or host:port of the S3 service
	Endpoint string `json:"endpoint,omitempty"`
	// EndpointCA is the path on disk to the S3 service's trusted CA list. Leave empty to use the OS CA bundle.
	EndpointCA string `json:"endpointCA,omitempty"`
	// SkipSSLVerify is true if TLS certificate verification is disabled
	SkipSSLVerify bool `json:"skipSSLVerify,omitempty"`
	// Bucket is the bucket holding the snapshot
	Bucket string `json:"bucket,omitempty"`
	// Region is the region of the S3 service
	Region string `json:"region,omitempty"`
	// Prefix is the prefix in which the snapshot file is stored.
	Prefix string `json:"prefix,omitempty"`
	// Insecure is true if the S3 service uses HTTP instead of HTTPS
	Insecure bool `json:"insecure,omitempty"`
}
