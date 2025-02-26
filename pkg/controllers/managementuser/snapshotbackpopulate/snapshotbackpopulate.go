package snapshotbackpopulate

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	jsonpatch "github.com/evanphx/json-patch/v5"
	k3s "github.com/k3s-io/api/k3s.cattle.io/v1"
	k3scontrollers "github.com/k3s-io/api/pkg/generated/controllers/k3s.cattle.io/v1"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	cluster2 "github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	provisioningcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkev1controllers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
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
	controlPlaneCache          rkev1controllers.RKEControlPlaneCache
	etcdSnapshotCache          rkev1controllers.ETCDSnapshotCache
	etcdSnapshotController     rkev1controllers.ETCDSnapshotController
	machineCache               capicontrollers.MachineCache
	capiClusterCache           capicontrollers.ClusterCache
	etcdSnapshotFileController k3scontrollers.ETCDSnapshotFileController
	etcdSnapshotFileCache      k3scontrollers.ETCDSnapshotFileCache
}

// Register sets up the v2provisioning snapshot backpopulate controller. This controller is responsible for monitoring
// the downstream etcd-snapshots configmap and backpopulating snapshots into etcd snapshot objects in the management cluster.
func Register(ctx context.Context, userContext *config.UserContext) {
	h := handler{
		clusterName:                userContext.ClusterName,
		clusterCache:               userContext.Management.Wrangler.Provisioning.Cluster().Cache(),
		controlPlaneCache:          userContext.Management.Wrangler.RKE.RKEControlPlane().Cache(),
		etcdSnapshotCache:          userContext.Management.Wrangler.RKE.ETCDSnapshot().Cache(),
		etcdSnapshotController:     userContext.Management.Wrangler.RKE.ETCDSnapshot(),
		machineCache:               userContext.Management.Wrangler.CAPI.Machine().Cache(),
		capiClusterCache:           userContext.Management.Wrangler.CAPI.Cluster().Cache(),
		etcdSnapshotFileController: userContext.K3s.V1().ETCDSnapshotFile(),
		etcdSnapshotFileCache:      userContext.K3s.V1().ETCDSnapshotFile().Cache(),
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

	if snapshot.Namespace != cluster.Namespace || snapshot.Labels == nil || snapshot.Labels[capr.ClusterNameLabel] != cluster.Name {
		return snapshot, nil
	}

	logPrefix := getLogPrefix(cluster)

	controlPlane, err := h.controlPlaneCache.Get(cluster.Namespace, cluster.Name)
	if err != nil {
		return snapshot, err
	}

	// if controlplane is currently performing a restore, reconciling snapshots will be postponed until post restore
	if controlPlane.Spec.ETCDSnapshotRestore != nil && controlPlane.Status.ETCDSnapshotRestore != nil &&
		controlPlane.Spec.ETCDSnapshotRestore.Generation != controlPlane.Status.ETCDSnapshotRestore.Generation {
		h.etcdSnapshotController.EnqueueAfter(snapshot.Namespace, snapshot.Name, 1*time.Minute)
		return snapshot, nil
	}

	// Only delete snapshots if the annotation is present: this will allow users to manually create snapshot objects during a DR scenario
	if snapshot.Annotations == nil || snapshot.Annotations[capr.SnapshotNameAnnotation] == "" {
		logrus.Debugf("%s local snapshot %s will not be deleted due to missing annotation %s", logPrefix, snapshot.Name, capr.SnapshotNameAnnotation)
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

	if cluster.DeletionTimestamp != nil {
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

	controlPlane, err := h.controlPlaneCache.Get(cluster.Namespace, cluster.Name)
	if err != nil {
		return downstream, err
	}

	// if controlplane is currently performing a restore, reconciling snapshots will be postponed until post restore
	if controlPlane.Spec.ETCDSnapshotRestore != nil && controlPlane.Status.ETCDSnapshotRestore != nil &&
		controlPlane.Spec.ETCDSnapshotRestore.Generation != controlPlane.Status.ETCDSnapshotRestore.Generation {
		logrus.Debugf("%s skipping snapshot reconcile as cluster is being restored", logPrefix)

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
		upstream, err := h.populateUpstreamSnapshotFromDownstream(nil, downstream, cluster, controlPlane)
		if err != nil {
			return downstream, err
		}
		logrus.Debugf("%s creating snapshot %s", logPrefix, upstream.Name)

		_, err = h.etcdSnapshotController.Create(upstream)
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
	generated, err := h.populateUpstreamSnapshotFromDownstream(upstream, downstream, cluster, controlPlane)
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

// populateUpstreamSnapshotFromDownstream sets the labels, annotations, spec and status fields which are governed by the
// downstream snapshot. Also sets the relevant owner references (machine for local, capi cluster for s3), and
// namespace/name if the snapshot is being created.
func (h *handler) populateUpstreamSnapshotFromDownstream(upstream *rkev1.ETCDSnapshot, downstream *k3s.ETCDSnapshotFile, cluster *provv1.Cluster, controlPlane *rkev1.RKEControlPlane) (*rkev1.ETCDSnapshot, error) {
	storage := S3
	if downstream.Spec.S3 == nil {
		storage = Local
	}

	if upstream == nil {
		name := name.SafeConcatName(cluster.Name, strings.ToLower(InvalidKeyChars.ReplaceAllString(downstream.Spec.SnapshotName, "-")), string(storage))
		upstream = &rkev1.ETCDSnapshot{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cluster.Namespace,
				Name:      name,
			},
		}
	} else {
		upstream = upstream.DeepCopy()
	}

	if upstream.Labels == nil {
		upstream.Labels = map[string]string{}
	}
	upstream.Labels[capr.ClusterNameLabel] = cluster.Name

	if upstream.Annotations == nil {
		upstream.Annotations = map[string]string{}
	}
	upstream.Annotations[StorageAnnotationKey] = string(storage)
	upstream.Annotations[capr.SnapshotNameAnnotation] = downstream.Name

	upstream.Spec.ClusterName = cluster.Name
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
		var machine *capi.Machine
		var err error
		if upstream.Labels != nil && upstream.Labels[capr.MachineIDLabel] != "" {
			machine, err = h.getMachineByID(upstream.Labels[capr.MachineIDLabel], cluster.Name, cluster.Namespace)
			if err != nil {
				logrus.Errorf("%s error getting machine by id for snapshot %s: %v", getLogPrefix(cluster), upstream.Name, err)
			}
		}
		// fallback to getting by node name, also used on snapshot create
		if machine == nil {
			machine, err = h.getMachineFromNode(downstream.Spec.NodeName, cluster.Name, cluster.Namespace)
			if err != nil {
				return upstream, err
			}
		}
		upstream.Labels[capr.MachineIDLabel] = machine.Labels[capr.MachineIDLabel]
		upstream.Labels[capr.NodeNameLabel] = downstream.Spec.NodeName
		upstream.OwnerReferences = []metav1.OwnerReference{capr.ToOwnerReference(machine.TypeMeta, machine.ObjectMeta)}
	} else {
		capiCluster, err := capr.GetCAPIClusterFromLabel(controlPlane, h.capiClusterCache)
		if err != nil {
			return upstream, err
		}
		upstream.OwnerReferences = []metav1.OwnerReference{capr.ToOwnerReference(capiCluster.TypeMeta, capiCluster.ObjectMeta)}
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
func (h *handler) getCluster() (*provv1.Cluster, error) {
	clusters, err := h.clusterCache.GetByIndex(cluster2.ByCluster, h.clusterName)
	if err != nil || len(clusters) != 1 {
		return nil, fmt.Errorf("error while retrieving cluster %s from cache via index: %w", h.clusterName, err)
	}
	return clusters[0], nil
}

// getSnapshotsFromSnapshotFile returns all snapshots objects for the given cluster for the downstream snapshotfile object.
// During normal operation, this will return either 0 snapshots, indicating that the snapshot has not been reconciled yet,
// or 1 snapshot. While multiple snapshots being returned is possible via user intervention via manually editing snapshots,
// this is an edge case and results in all local snapshot objects being deleted and the downstream snapshot being
// re-enqueued for regeneration.
func (h *handler) getSnapshotsFromSnapshotFile(cluster *provv1.Cluster, snapshotFile *k3s.ETCDSnapshotFile) ([]*rkev1.ETCDSnapshot, error) {
	snapshots, err := h.etcdSnapshotCache.GetByIndex(cluster2.ByETCDSnapshotName, fmt.Sprintf("%s/%s/%s", cluster.Namespace, cluster.Name, snapshotFile.Name))
	if err != nil {
		return nil, err
	}
	return snapshots, nil
}

// getMachineFromNode attempts to find the corresponding machine for an etcd snapshot that is found in the configmap. If the machine list is successful, it will return true on the boolean, otherwise, it can be assumed that a false, nil, and defined error indicate the machine does not exist.
func (h *handler) getMachineFromNode(nodeName string, clusterName, namespace string) (*capi.Machine, error) {
	ls, err := labels.Parse(fmt.Sprintf("%s=%s", capi.ClusterNameLabel, clusterName))
	if err != nil {
		return nil, err
	}
	machines, err := h.machineCache.List(namespace, ls)
	if err != nil {
		return nil, err
	}
	for _, machine := range machines {
		if machine.Status.NodeRef != nil && machine.Status.NodeRef.Name == nodeName {
			return machine, nil
		}
	}
	return nil, fmt.Errorf("unable to find node %s in machines", nodeName)
}

// getMachineByID attempts to find the corresponding machine for an etcd snapshot that is found in the configmap. If the machine list is successful, it will return true on the boolean, otherwise, it can be assumed that a false, nil, and defined error indicate the machine does not exist.
func (h *handler) getMachineByID(machineID string, clusterName, namespace string) (*capi.Machine, error) {
	machines, err := h.machineCache.List(namespace, labels.SelectorFromSet(labels.Set{
		capr.ClusterNameLabel: clusterName,
		capr.MachineIDLabel:   machineID,
	}))
	if err != nil {
		return nil, err
	}
	if len(machines) > 1 {
		return nil, fmt.Errorf("found multiple machines in cluster with machine ID %s", machineID)
	}
	if len(machines) == 0 {
		return nil, fmt.Errorf("found no machines in cluster with machine ID %s", machineID)
	}
	return machines[0], nil
}

func getLogPrefix(cluster *provv1.Cluster) string {
	return fmt.Sprintf("[snapshotbackpopulate] rkecluster %s/%s:", cluster.Namespace, cluster.Name)
}
