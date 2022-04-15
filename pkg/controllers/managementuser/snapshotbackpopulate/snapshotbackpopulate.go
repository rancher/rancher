package snapshotbackpopulate

import (
	"context"
	"encoding/json"
	"fmt"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	cluster2 "github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	provisioningcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkev1controllers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	configMapNames = map[string]bool{
		"k3s-etcd-snapshots":  true,
		"rke2-etcd-snapshots": true,
	}
)

const (
	StorageLabelKey = "etcdsnapshot.rke.io/storage"
	SnapshotNameKey = "etcdsnapshot.rke.io/snapshot-file-name"
	StorageS3       = "s3"
	StorageLocal    = "local"
)

type handler struct {
	clusterName       string
	clusterNamespace  string
	clusterCache      provisioningcontrollers.ClusterCache
	clusters          provisioningcontrollers.ClusterClient
	etcdSnapshotCache rkev1controllers.ETCDSnapshotCache
	etcdSnapshots     rkev1controllers.ETCDSnapshotClient
	machineCache      capicontrollers.MachineCache
}

// Register sets up the v2provisioning snapshot backpopulate controller. This controller is responsible for monitoring
// the downstream etcd-snapshots configmap and backpopulating snapshots into etcd snapshot objects in the management cluster.
func Register(ctx context.Context, userContext *config.UserContext) {
	h := handler{
		clusterName:       userContext.ClusterName,
		clusterCache:      userContext.Management.Wrangler.Provisioning.Cluster().Cache(),
		clusters:          userContext.Management.Wrangler.Provisioning.Cluster(),
		etcdSnapshotCache: userContext.Management.Wrangler.RKE.ETCDSnapshot().Cache(),
		etcdSnapshots:     userContext.Management.Wrangler.RKE.ETCDSnapshot(),
		machineCache:      userContext.Management.Wrangler.CAPI.Machine().Cache(),
	}

	userContext.Core.ConfigMaps("kube-system").Controller().AddHandler(ctx, "snapshotbackpopulate", h.OnChange)
}

func (h *handler) OnChange(key string, configMap *corev1.ConfigMap) (runtime.Object, error) {
	if configMap == nil {
		return nil, nil
	}

	if configMap.Namespace != "kube-system" || !configMapNames[configMap.Name] {
		return configMap, nil
	}

	clusters, err := h.clusterCache.GetByIndex(cluster2.ByCluster, h.clusterName)
	if err != nil || len(clusters) != 1 {
		return configMap, fmt.Errorf("error while retrieving cluster %s from cache via index: %w", h.clusterName, err)
	}

	cluster := clusters[0]

	logrus.Infof("[snapshotbackpopulate] rkecluster %s/%s: processing configmap %s/%s", cluster.Namespace, cluster.Name, configMap.Namespace, configMap.Name)

	actualEtcdSnapshots, err := h.configMapToSnapshots(configMap, cluster)
	if err != nil {
		return configMap, fmt.Errorf("error while converting configmap to snapshot map for cluster %s: %w", cluster.Name, err)
	}

	ls, err := labels.Parse(fmt.Sprintf("%s=%s", rke2.ClusterNameLabel, cluster.Name))
	if err != nil {
		return configMap, err
	}

	currentEtcdSnapshots, err := h.etcdSnapshotCache.List(cluster.Namespace, ls)
	if err != nil {
		return configMap, fmt.Errorf("error while listing existing etcd snapshots for cluster %s: %w", cluster.Name, err)
	}

	currentEtcdSnapshotsToKeep := map[string]*rkev1.ETCDSnapshot{}
	// iterate over the current etcd snapshots
	// if the current etcd snapshot is NOT found in the actual etcd snapshots list, we delete it
	// otherwise, we add it to the desired etcd snapshots map
	for _, existingSnapshotCR := range currentEtcdSnapshots {
		storageLocation, ok := existingSnapshotCR.GetAnnotations()[StorageLabelKey]
		if !ok {
			storageLocation = StorageLocal
			if existingSnapshotCR.SnapshotFile.NodeName == StorageS3 {
				storageLocation = StorageS3
			}
		}
		snapshotKey := existingSnapshotCR.SnapshotFile.Name + storageLocation
		if _, ok := actualEtcdSnapshots[snapshotKey]; !ok {
			if storageLocation != StorageS3 {
				// check to make sure the machine actually exists for the snapshot
				listSuccessful, machine, err := rke2.GetMachineFromNode(h.machineCache, existingSnapshotCR.SnapshotFile.NodeName, cluster)
				if listSuccessful && machine == nil && err != nil {
					// delete the CR because we don't have a corresponding machine for it
					logrus.Infof("[snapshotbackpopulate] rkecluster %s/%s: deleting snapshot %s as corresponding machine was not found", cluster.Namespace, cluster.Name, existingSnapshotCR.Name)
					if err := h.etcdSnapshots.Delete(existingSnapshotCR.Namespace, existingSnapshotCR.Name, &metav1.DeleteOptions{}); err != nil {
						if !apierrors.IsNotFound(err) {
							return configMap, err
						}
					}
					continue
				}
			}
			// indicate that it should be OK to delete the etcd object
			// don't delete the snapshots here because our configmap can be outdated. we will reconcile based on the system-agent output via the periodic output
			logrus.Debugf("[snapshotbackpopulate] rkecluster %s/%s: updating status missing=true on etcd snapshot %s/%s as it was not found in the actual snapshot config map", cluster.Namespace, cluster.Name, existingSnapshotCR.Namespace, existingSnapshotCR.Name)
			logrus.Tracef("[snapshotbackpopulate] rkecluster %s/%s: etcd snapshot was %s/%s: %+v", cluster.Namespace, cluster.Name, existingSnapshotCR.Namespace, existingSnapshotCR.Name, existingSnapshotCR)
			existingSnapshotCR = existingSnapshotCR.DeepCopy()
			existingSnapshotCR.Status.Missing = true // a missing snapshot indicates that it was not found in the (rke2|k3s)-etcd-snapshots configmap. This could potentially be a transient situation after an etcd snapshot restore to an older/newer datastore, when new snapshots have not been taken.
			if existingSnapshotCR, err = h.etcdSnapshots.UpdateStatus(existingSnapshotCR); err != nil && !apierrors.IsNotFound(err) {
				return configMap, fmt.Errorf("rkecluster %s/%s: error while setting status missing=true on etcd snapshot %s/%s: %w", cluster.Namespace, cluster.Name, existingSnapshotCR.Namespace, existingSnapshotCR.Name, err)
			}
			continue
		}
		currentEtcdSnapshotsToKeep[snapshotKey] = existingSnapshotCR
	}

	// iterate over the actual etcd snapshots that are in the management cluster
	// if the snapshot is found in the desired etcd snapshots, check to see if an update needs to be made
	// otherwise, create the etcd snapshot CR
	for snapshotKey, cmGeneratedSnapshot := range actualEtcdSnapshots {
		if snapshot, ok := currentEtcdSnapshotsToKeep[snapshotKey]; ok {
			if !equality.Semantic.DeepEqual(cmGeneratedSnapshot.SnapshotFile, snapshot.SnapshotFile) || !equality.Semantic.DeepEqual(cmGeneratedSnapshot.Status, snapshot.Status) {
				logrus.Debugf("[snapshotbackpopulate] rkecluster %s/%s: updating etcd snapshot %s/%s as it differed from the actual snapshot config map %v vs %v", cluster.Namespace, cluster.Name, snapshot.Namespace, snapshot.Name, cmGeneratedSnapshot.SnapshotFile, snapshot.SnapshotFile)
				logrus.Tracef("[snapshotbackpopulate] rkecluster %s/%s: updating etcd snapshot %s/%s: %+v", cluster.Namespace, cluster.Name, cmGeneratedSnapshot.Namespace, cmGeneratedSnapshot.Name, cmGeneratedSnapshot)
				snapshot = snapshot.DeepCopy()
				// keep a copy of the metadata and message
				md := snapshot.SnapshotFile.Metadata
				msg := snapshot.SnapshotFile.Message
				snapshot.SnapshotFile = cmGeneratedSnapshot.SnapshotFile
				// restore the metadata and message as those may have been lost
				snapshot.SnapshotFile.Metadata = md
				snapshot.SnapshotFile.Message = msg
				snapshot, err = h.etcdSnapshots.Update(snapshot)
				if err != nil {
					return configMap, fmt.Errorf("rkecluster %s/%s: error while updating etcd snapshot %s/%s: %w", cluster.Namespace, cluster.Name, snapshot.Namespace, snapshot.Name, err)
				}
				if snapshot.Status.Missing {
					snapshot.Status.Missing = false
					// the kube-apiserver only accepts status updates on deliberate subresource status updates which is why we have to double-call an update here if the missing is set incorrectly
					if _, err := h.etcdSnapshots.UpdateStatus(snapshot); err != nil && !apierrors.IsNotFound(err) {
						return configMap, fmt.Errorf("rkecluster %s/%s: error while setting status missing=false on etcd snapshot %s/%s: %w", cluster.Namespace, cluster.Name, snapshot.Namespace, snapshot.Name, err)
					}
				}
			}
		} else {
			// create the snapshot in the mgmt cluster
			logrus.Debugf("[snapshotbackpopulate] rkecluster %s/%s: creating etcd snapshot %s/%s as it differed from the actual snapshot config map", cluster.Namespace, cluster.Name, cmGeneratedSnapshot.Namespace, cmGeneratedSnapshot.Name)
			logrus.Tracef("[snapshotbackpopulate] rkecluster %s/%s: creating etcd snapshot %s/%s: %+v", cluster.Namespace, cluster.Name, cmGeneratedSnapshot.Namespace, cmGeneratedSnapshot.Name, cmGeneratedSnapshot)
			_, err = h.etcdSnapshots.Create(&cmGeneratedSnapshot)
			if err != nil {
				if apierrors.IsAlreadyExists(err) {
					logrus.Debugf("[snapshotbackpopulate] rkecluster %s/%s: duplicate snapshot found when creating snapshot %s/%s", cluster.Namespace, cluster.Name, cmGeneratedSnapshot.Namespace, cmGeneratedSnapshot.Name)
					continue
				}
				return configMap, fmt.Errorf("rkecluster %s/%s: error while creating etcd snapshot %s/%s: %w", cluster.Namespace, cluster.Name, cmGeneratedSnapshot.Namespace, cmGeneratedSnapshot.Name, err)
			}
		}
	}
	return configMap, nil
}

func (h *handler) configMapToSnapshots(configMap *corev1.ConfigMap, cluster *provv1.Cluster) (map[string]rkev1.ETCDSnapshot, error) {
	result := map[string]rkev1.ETCDSnapshot{}
	for k, v := range configMap.Data {
		file := &snapshotFile{}
		if err := json.Unmarshal([]byte(v), file); err != nil {
			logrus.Errorf("invalid non-json value in %s/%s for key %s in cluster %s", configMap.Namespace, configMap.Name, k, h.clusterName)
			return nil, nil
		}
		// Validate that the corresponding machine for the node exists before creating the snapshot
		snapshot := rkev1.ETCDSnapshot{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cluster.Namespace,
				Labels: map[string]string{
					rke2.ClusterNameLabel: cluster.Name,
					rke2.NodeNameLabel:    file.NodeName,
				},
				Annotations: map[string]string{
					SnapshotNameKey: file.Name,
					StorageLabelKey: StorageLocal,
				},
				OwnerReferences: []metav1.OwnerReference{},
			},
			Spec: rkev1.ETCDSnapshotSpec{
				ClusterName: cluster.Name,
			},
			SnapshotFile: rkev1.ETCDSnapshotFile{
				Name:      file.Name,
				Location:  file.Location,
				Metadata:  file.Metadata,
				Message:   file.Message,
				NodeName:  file.NodeName,
				CreatedAt: file.CreatedAt,
				Size:      file.Size,
				Status:    file.Status,
			},
		}
		fileSuffix := StorageLocal
		if file.S3 != nil {
			fileSuffix = StorageS3
			snapshot.SnapshotFile.S3 = &rkev1.ETCDSnapshotS3{
				Endpoint:      file.S3.Endpoint,
				EndpointCA:    file.S3.EndpointCA,
				SkipSSLVerify: file.S3.SkipSSLVerify,
				Bucket:        file.S3.Bucket,
				Region:        file.S3.Region,
				Folder:        file.S3.Folder,
			}
			snapshot.Annotations[StorageLabelKey] = StorageS3
		} else {
			listSuccessful, machine, err := rke2.GetMachineFromNode(h.machineCache, file.NodeName, cluster)
			if listSuccessful && err != nil {
				logrus.Errorf("error getting machine from node (%s) for snapshot (%s/%s): %v", file.NodeName, snapshot.Namespace, snapshot.Name, err)
				continue // don't add this snapshot to the list as we can't actually correlate it to an existing node
			}
			snapshot.OwnerReferences = append(snapshot.OwnerReferences, metav1.OwnerReference{
				APIVersion:         machine.APIVersion,
				Kind:               machine.Kind,
				Name:               machine.Name,
				UID:                machine.UID,
				Controller:         &[]bool{true}[0],
				BlockOwnerDeletion: &[]bool{true}[0],
			})
		}
		if len(snapshot.OwnerReferences) == 0 {
			snapshot.OwnerReferences = []metav1.OwnerReference{{
				APIVersion:         cluster.APIVersion,
				Kind:               cluster.Kind,
				Name:               cluster.Name,
				UID:                cluster.UID,
				Controller:         &[]bool{true}[0],
				BlockOwnerDeletion: &[]bool{true}[0],
			}}
		}
		snapshot.Name = name.SafeConcatName(cluster.Name, snapshot.SnapshotFile.Name, fileSuffix)
		result[snapshot.SnapshotFile.Name+fileSuffix] = snapshot
	}
	return result, nil
}

type s3Config struct {
	Endpoint      string `json:"endpoint,omitempty"`
	EndpointCA    string `json:"endpointCA,omitempty"`
	SkipSSLVerify bool   `json:"skipSSLVerify,omitempty"`
	Bucket        string `json:"bucket,omitempty"`
	Region        string `json:"region,omitempty"`
	Folder        string `json:"folder,omitempty"`
}

// snapshotFile represents a single snapshot and it's
// metadata, and is used to unmarshal snapshot data from
// the downstream config map.
type snapshotFile struct {
	Name      string       `json:"name"`
	Location  string       `json:"location,omitempty"`
	NodeName  string       `json:"nodeName,omitempty"`
	Metadata  string       `json:"metadata,omitempty"`
	Message   string       `json:"message,omitempty"`
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`
	Size      int64        `json:"size,omitempty"`
	Status    string       `json:"status,omitempty"`
	S3        *s3Config    `json:"s3Config,omitempty"`
}
