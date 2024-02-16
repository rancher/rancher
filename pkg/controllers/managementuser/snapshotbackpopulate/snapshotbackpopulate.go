package snapshotbackpopulate

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	cluster2 "github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	provisioningcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkev1controllers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v2/pkg/name"
	"github.com/rancher/wrangler/v2/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

var (
	configMapNames = map[string]bool{
		"k3s-etcd-snapshots":  true,
		"rke2-etcd-snapshots": true,
	}
	InvalidKeyChars = regexp.MustCompile(`[^-.a-zA-Z0-9]`)
)

const (
	StorageAnnotationKey              = "etcdsnapshot.rke.io/storage"
	SnapshotNameKey                   = "etcdsnapshot.rke.io/snapshot-file-name"
	SnapshotBackpopulateReconciledKey = "etcdsnapshot.rke.io/snapshotbackpopulate-reconciled"
	StorageS3                         = "s3"
	StorageLocal                      = "local"
)

type handler struct {
	clusterName       string
	clusterNamespace  string
	clusterCache      provisioningcontrollers.ClusterCache
	clusters          provisioningcontrollers.ClusterClient
	etcdSnapshotCache rkev1controllers.ETCDSnapshotCache
	etcdSnapshots     rkev1controllers.ETCDSnapshotClient
	machineCache      capicontrollers.MachineCache
	activeConfigMap   string
	v1ClusterName     string
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
	relatedresource.Watch(ctx, "snapshot-reconcile-trigger", func(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
		if snapshot, ok := obj.(*rkev1.ETCDSnapshot); ok && snapshot.Spec.ClusterName == h.v1ClusterName && h.activeConfigMap != "" {
			return []relatedresource.Key{{
				Namespace: "kube-system",
				Name:      h.activeConfigMap,
			}}, nil
		}
		return nil, nil
	}, userContext.Core.ConfigMaps("kube-system").Controller(), userContext.Management.Wrangler.RKE.ETCDSnapshot())
}

func (h *handler) OnChange(key string, configMap *corev1.ConfigMap) (runtime.Object, error) {
	if configMap == nil {
		return nil, nil
	}

	if configMap.Namespace != "kube-system" || !configMapNames[configMap.Name] {
		return configMap, nil
	}

	if h.activeConfigMap == "" {
		h.activeConfigMap = configMap.Name
	}

	clusters, err := h.clusterCache.GetByIndex(cluster2.ByCluster, h.clusterName)
	if err != nil || len(clusters) != 1 {
		return configMap, fmt.Errorf("error while retrieving cluster %s from cache via index: %w", h.clusterName, err)
	}

	cluster := clusters[0]

	if h.v1ClusterName == "" {
		h.v1ClusterName = cluster.Name
	}

	logrus.Infof("[snapshotbackpopulate] rkecluster %s/%s: processing configmap %s/%s", cluster.Namespace, cluster.Name, configMap.Namespace, configMap.Name)

	actualEtcdSnapshots := h.configMapToSnapshots(configMap, cluster)

	ls, err := labels.Parse(fmt.Sprintf("%s=%s", capr.ClusterNameLabel, cluster.Name))
	if err != nil {
		return configMap, err
	}

	currentEtcdSnapshots, err := h.etcdSnapshotCache.List(cluster.Namespace, ls)
	if err != nil {
		return configMap, fmt.Errorf("error while listing existing etcd snapshots for cluster %s: %w", cluster.Name, err)
	}

	// currentEtcdSnapshotsToKeep is a map of etcd snapshots snapshot objects we have retrieved from the management cluster.
	currentEtcdSnapshotsToKeep := map[string]*rkev1.ETCDSnapshot{}

	// iterate over the current etcd snapshots objects retrieved from the management cluster
	// if snapshot object is not found in the etcd snapshot configmap, mark it missing. if it is a local snapshot
	// and no machine can be found for it, go ahead and delete it.
	// if the snapshot object is found in the configmap, add it to the currentEtcdSnapshotsToKeep for reconciliation
	for _, existingSnapshotCR := range currentEtcdSnapshots {
		storageLocation, ok := existingSnapshotCR.GetAnnotations()[StorageAnnotationKey]
		if !ok {
			storageLocation = StorageLocal
			if existingSnapshotCR.SnapshotFile.NodeName == StorageS3 {
				storageLocation = StorageS3
			}
		}
		snapshotKey := existingSnapshotCR.SnapshotFile.Name + storageLocation
		// check to see if the snapshot CR we have is found in the etcd-snapshots configmap, and if it is not found in the configmap, mark it as missing
		if _, ok := actualEtcdSnapshots[snapshotKey]; !ok {
			// the snapshot custom resource we are processing does not exist in the configmap
			if storageLocation != StorageS3 {
				// check to make sure the machine actually exists for the snapshot
				var listSuccessful bool
				var machine *capi.Machine

				if existingSnapshotCR.Labels[capr.MachineIDLabel] == "" {
					logrus.Debugf("[snapshotbackpopulate] rkecluster %s/%s: snapshot %s/%s was missing machine ID label: %s", cluster.Namespace, cluster.Name, existingSnapshotCR.Namespace, existingSnapshotCR.Name, capr.MachineIDLabel)
					// If the machineID label was not set, fall back to looking up the machine by node name, as this may be a snapshot from an earlier version of Rancher that created local snapshots using the snapshotbackpopulate controller, which means the snapshot should not have the machine ID label.
					listSuccessful, machine, err = capr.GetMachineFromNode(h.machineCache, existingSnapshotCR.SnapshotFile.NodeName, cluster)
				} else {
					listSuccessful, machine, err = capr.GetMachineByID(h.machineCache, existingSnapshotCR.Labels[capr.MachineIDLabel], cluster.Namespace, cluster.Name)
				}
				if listSuccessful && machine == nil && err != nil {
					// delete the CR because we don't have a corresponding machine for it
					logrus.Infof("[snapshotbackpopulate] rkecluster %s/%s: deleting snapshot %s as corresponding machine (ID: %s) was not found", cluster.Namespace, cluster.Name, existingSnapshotCR.Name, existingSnapshotCR.Labels[capr.MachineIDLabel])
					if err := h.etcdSnapshots.Delete(existingSnapshotCR.Namespace, existingSnapshotCR.Name, &metav1.DeleteOptions{}); err != nil {
						if !apierrors.IsNotFound(err) {
							return configMap, err
						}
					}
					continue
				}
			}
			// indicate that it should be OK to delete the etcd snapshot object
			// don't delete the snapshots here because our configmap can be outdated. we will reconcile based on the system-agent output via the periodic output
			logrus.Debugf("[snapshotbackpopulate] rkecluster %s/%s: updating status missing=true on etcd snapshot %s/%s as it was not found in the actual snapshot config map", cluster.Namespace, cluster.Name, existingSnapshotCR.Namespace, existingSnapshotCR.Name)
			logrus.Tracef("[snapshotbackpopulate] rkecluster %s/%s: etcd snapshot was %s/%s: %v", cluster.Namespace, cluster.Name, existingSnapshotCR.Namespace, existingSnapshotCR.Name, existingSnapshotCR)
			existingSnapshotCR = existingSnapshotCR.DeepCopy()
			existingSnapshotCR.Status.Missing = true // a missing snapshot indicates that it was not found in the (rke2|k3s)-etcd-snapshots configmap. This could potentially be a transient situation after an etcd snapshot restore to an older/newer datastore, when new snapshots have not been taken.
			if existingSnapshotCR, err = h.etcdSnapshots.UpdateStatus(existingSnapshotCR); err != nil && !apierrors.IsNotFound(err) {
				return configMap, fmt.Errorf("rkecluster %s/%s: error while setting status missing=true on etcd snapshot %s/%s: %w", cluster.Namespace, cluster.Name, existingSnapshotCR.Namespace, existingSnapshotCR.Name, err)
			}
			continue
		}
		currentEtcdSnapshotsToKeep[snapshotKey] = existingSnapshotCR
	}

	// iterate over the snapshots that are listed in the downstream cluster configmap
	// if the snapshot is found in the list of snapshots objects that exist in the management cluster, check to see if the etcdsnapshot object needs to be reconciled with more accurate information.
	// if the snapshot is not found in the list of snapshot objects that exist in the management cluster and the snapshot is an S3 snapshot, create it in the management cluster.
	for snapshotKey, cmGeneratedSnapshot := range actualEtcdSnapshots {
		if snapshot, ok := currentEtcdSnapshotsToKeep[snapshotKey]; ok {
			// the snapshot CR exists in the management cluster. check to see if the configmap data needs to be set on the snapshot.
			logrus.Debugf("[snapshotbackpopulate] rkecluster %s/%s: checking to see if etcdsnapshot %s/%s needs to be updated", cluster.Namespace, cluster.Name, snapshot.Namespace, snapshot.Name)
			logrus.Tracef("[snapshotbackpopulate] rkecluster %s/%s: comparing etcd snapshot %s/%s: %v : %v", cluster.Namespace, cluster.Name, snapshot.Namespace, snapshot.Name, cmGeneratedSnapshot, snapshot)
			snapshot = snapshot.DeepCopy()
			var updated bool
			if !equality.Semantic.DeepEqual(snapshot.SnapshotFile, cmGeneratedSnapshot.SnapshotFile) {
				logrus.Debugf("[snapshotbackpopulate] rkecluster %s/%s: snapshot %s/%s SnapshotFile contents differed from configmap", cluster.Namespace, cluster.Name, snapshot.Namespace, snapshot.Name)
				originalSnapshotFile := snapshot.SnapshotFile
				snapshot.SnapshotFile = cmGeneratedSnapshot.SnapshotFile
				if originalSnapshotFile.Metadata != "" && snapshot.SnapshotFile.Metadata == "" {
					snapshot.SnapshotFile.Metadata = originalSnapshotFile.Metadata
				}
				if originalSnapshotFile.Message != "" && snapshot.SnapshotFile.Message == "" {
					snapshot.SnapshotFile.Message = originalSnapshotFile.Message
				}
				if !equality.Semantic.DeepEqual(snapshot.SnapshotFile, originalSnapshotFile) {
					updated = true
					logrus.Debugf("[snapshotbackpopulate] rkecluster %s/%s: snapshot %s/%s SnapshotFile contents were different, triggering update", cluster.Namespace, cluster.Name, snapshot.Namespace, snapshot.Name)
					logrus.Tracef("[snapshotbackpopulate] rkecluster %s/%s: snapshot %s/%s SnapshotFile contents %v vs %v", cluster.Namespace, cluster.Name, snapshot.Namespace, snapshot.Name, originalSnapshotFile, snapshot.SnapshotFile)
				}
			}
			if snapshot.Spec.ClusterName != cmGeneratedSnapshot.Spec.ClusterName {
				logrus.Debugf("[snapshotbackpopulate] rkecluster %s/%s: snapshot %s/%s clusterName did not match %s vs %s", cluster.Namespace, cluster.Name, snapshot.Namespace, snapshot.Name, snapshot.Spec.ClusterName, cmGeneratedSnapshot.Spec.ClusterName)
				snapshot.Spec.ClusterName = cmGeneratedSnapshot.Spec.ClusterName
				updated = true
			}
			if labelsUpdated := reconcileStringMaps(snapshot.Labels, cmGeneratedSnapshot.Labels, []string{capr.ClusterNameLabel, capr.NodeNameLabel, capr.MachineIDLabel}); labelsUpdated {
				logrus.Debugf("[snapshotbackpopulate] rkecluster %s/%s: snapshot %s/%s labels did not match", cluster.Namespace, cluster.Name, snapshot.Namespace, snapshot.Name)
				updated = true
			}
			if annotationsUpdated := reconcileStringMaps(snapshot.Annotations, cmGeneratedSnapshot.Annotations, []string{SnapshotNameKey, StorageAnnotationKey, SnapshotBackpopulateReconciledKey}); annotationsUpdated {
				logrus.Debugf("[snapshotbackpopulate] rkecluster %s/%s: snapshot %s/%s annotations did not match", cluster.Namespace, cluster.Name, snapshot.Namespace, snapshot.Name)
				updated = true
			}
			if updated {
				logrus.Debugf("[snapshotbackpopulate] rkecluster %s/%s: updating etcdsnapshot %s/%s", cluster.Namespace, cluster.Name, snapshot.Namespace, snapshot.Name)
				snapshot, err = h.etcdSnapshots.Update(snapshot)
				if err != nil {
					return configMap, fmt.Errorf("rkecluster %s/%s: error while updating etcd snapshot %s/%s: %w", cluster.Namespace, cluster.Name, snapshot.Namespace, snapshot.Name, err)
				}
			}
			if snapshot.Status.Missing {
				// if we get to this point and the snapshot object missing status is true, reset it to false as we know it is no longer missing.
				snapshot.Status.Missing = false
				// the kube-apiserver only accepts status updates on deliberate subresource status updates which is why we have to double-call an update here if the missing is set incorrectly
				if _, err := h.etcdSnapshots.UpdateStatus(snapshot); err != nil && !apierrors.IsNotFound(err) {
					return configMap, fmt.Errorf("rkecluster %s/%s: error while setting status missing=false on etcd snapshot %s/%s: %w", cluster.Namespace, cluster.Name, snapshot.Namespace, snapshot.Name, err)
				}
			}
		} else {
			// create the snapshot in the mgmt cluster if it is an s3 snapshot
			// we only create S3 snapshots in the snapshotbackpopulate controller.
			// local snapshots are created by the `plansecret` controller based on real output of the snapshot data, and then updated by this controller.
			if cmGeneratedSnapshot.SnapshotFile.NodeName != "s3" {
				logrus.Debugf("[snapshotbackpopulate] rkecluster %s/%s: not creating etcd snapshot %s/%s as it is on a local node", cluster.Namespace, cluster.Name, cmGeneratedSnapshot.Namespace, cmGeneratedSnapshot.Name)
				continue
			}
			logrus.Debugf("[snapshotbackpopulate] rkecluster %s/%s: creating s3 etcd snapshot %s/%s as it differed from the actual snapshot config map", cluster.Namespace, cluster.Name, cmGeneratedSnapshot.Namespace, cmGeneratedSnapshot.Name)
			logrus.Tracef("[snapshotbackpopulate] rkecluster %s/%s: creating s3 etcd snapshot %s/%s: %v", cluster.Namespace, cluster.Name, cmGeneratedSnapshot.Namespace, cmGeneratedSnapshot.Name, cmGeneratedSnapshot)
			_, err = h.etcdSnapshots.Create(&cmGeneratedSnapshot)
			if err != nil {
				if apierrors.IsAlreadyExists(err) {
					logrus.Debugf("[snapshotbackpopulate] rkecluster %s/%s: duplicate snapshot found when creating s3 snapshot %s/%s", cluster.Namespace, cluster.Name, cmGeneratedSnapshot.Namespace, cmGeneratedSnapshot.Name)
					continue
				}
				logrus.Errorf("rkecluster %s/%s: error while creating s3 etcd snapshot %s/%s: %s", cluster.Namespace, cluster.Name, cmGeneratedSnapshot.Namespace, cmGeneratedSnapshot.Name, err.Error())
			}
		}
	}
	return configMap, nil
}

func reconcileStringMaps(input map[string]string, new map[string]string, keys []string) bool {
	var updated bool
	for _, key := range keys {
		if input[key] == "" && new[key] != "" {
			input[key] = new[key]
			updated = true
		}
	}
	return updated
}

// configMapToSnapshots parses the given configmap and returns a map of etcd snapshots. The snapshotbackpopulate controller will only create snapshots from this map that are located in S3.
// The snapshots will have 2 or 3 labels, 2 if they are S3 snapshots (cluster name and node name), and 3 if they are local snapshots. They will always have 2 annotations.
func (h *handler) configMapToSnapshots(configMap *corev1.ConfigMap, cluster *provv1.Cluster) map[string]rkev1.ETCDSnapshot {
	result := map[string]rkev1.ETCDSnapshot{}
	for k, v := range configMap.Data {
		file := &snapshotFile{}
		if err := json.Unmarshal([]byte(v), file); err != nil {
			logrus.Errorf("rkecluster %s/%s: invalid non-json value in %s/%s for key %s: %v", cluster.Namespace, cluster.Name, configMap.Namespace, configMap.Name, k, err)
			continue
		}
		snapshot := rkev1.ETCDSnapshot{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cluster.Namespace,
				Labels: map[string]string{
					capr.ClusterNameLabel: cluster.Name,
					capr.NodeNameLabel:    file.NodeName,
				},
				Annotations: map[string]string{
					SnapshotNameKey:                   file.Name,
					StorageAnnotationKey:              StorageLocal,
					SnapshotBackpopulateReconciledKey: "true",
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
			// if the snapshot is an S3 snapshot, set the corresponding S3-related figures.
			fileSuffix = StorageS3
			snapshot.SnapshotFile.S3 = &rkev1.ETCDSnapshotS3{
				Endpoint:      file.S3.Endpoint,
				EndpointCA:    file.S3.EndpointCA,
				SkipSSLVerify: file.S3.SkipSSLVerify,
				Bucket:        file.S3.Bucket,
				Region:        file.S3.Region,
				Folder:        file.S3.Folder,
			}
			snapshot.OwnerReferences = []metav1.OwnerReference{{
				APIVersion:         cluster.APIVersion,
				Kind:               cluster.Kind,
				Name:               cluster.Name,
				UID:                cluster.UID,
				Controller:         &[]bool{true}[0],
				BlockOwnerDeletion: &[]bool{true}[0],
			}}
			snapshot.Annotations[StorageAnnotationKey] = StorageS3
		} else {
			listSuccessful, machine, err := capr.GetMachineFromNode(h.machineCache, file.NodeName, cluster)
			if listSuccessful && err != nil {
				logrus.Debugf("[snapshotbackpopulate] rkecluster %s/%s: error getting machine from node (%s) for snapshot (%s/%s): %v", cluster.Namespace, cluster.Name, file.NodeName, snapshot.Namespace, snapshot.Name, err)
			} else if listSuccessful && machine != nil && machine.Labels[capr.MachineIDLabel] == "" {
				logrus.Debugf("[snapshotbackpopulate] rkecluster %s/%s: machine (%s/%s) for snapshot %s on node: %s had empty Machine ID label", cluster.Namespace, cluster.Name, machine.Namespace, machine.Name, file.Name, file.NodeName)
			} else {
				snapshot.Labels[capr.MachineIDLabel] = machine.Labels[capr.MachineIDLabel]
			}
		}
		snapshot.Name = name.SafeConcatName(cluster.Name, strings.ToLower(InvalidKeyChars.ReplaceAllString(snapshot.SnapshotFile.Name, "-")), fileSuffix)
		result[snapshot.SnapshotFile.Name+fileSuffix] = snapshot
	}
	return result
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
