package snapshotbackpopulate

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	k3s "github.com/k3s-io/api/k3s.cattle.io/v1"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	cluster2 "github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/utils/ptr"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

func TestOnUpstreamChange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string

		snapshot *rkev1.ETCDSnapshot

		handlerFunc func(ctrl *gomock.Controller) handler
		expectErr   bool
	}{
		{
			name:     "nil snapshot",
			snapshot: nil,
			handlerFunc: func(_ *gomock.Controller) handler {
				return handler{}
			},
			expectErr: false,
		},
		{
			name:     "failed to get cluster",
			snapshot: &rkev1.ETCDSnapshot{},
			handlerFunc: func(ctrl *gomock.Controller) handler {
				clusterCache := fake.NewMockCacheInterface[*provv1.Cluster](ctrl)
				clusterCache.EXPECT().GetByIndex(cluster2.ByCluster, "test-cluster").Return(nil, errors.New("failed to get cluster"))
				h := handler{
					clusterName:  "test-cluster",
					clusterCache: clusterCache,
				}
				return h
			},
			expectErr: true,
		},
		{
			name:     "snapshot from different namespace",
			snapshot: rkev1.NewETCDSnapshot("other-namespace", "test-snapshot", rkev1.ETCDSnapshot{}),
			handlerFunc: func(ctrl *gomock.Controller) handler {
				clusterCache := fake.NewMockCacheInterface[*provv1.Cluster](ctrl)
				cluster := &provv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-namespace",
						Name:      "test-cluster",
					},
					Status: provv1.ClusterStatus{
						ClusterName: "test-mgmt-cluster",
					},
				}
				clusterCache.EXPECT().GetByIndex(cluster2.ByCluster, "test-mgmt-cluster").Return([]*provv1.Cluster{cluster}, nil)
				h := handler{
					clusterName:  cluster.Status.ClusterName,
					clusterCache: clusterCache,
				}
				return h
			},
			expectErr: false,
		},
		{
			name:     "snapshot has no labels",
			snapshot: rkev1.NewETCDSnapshot("test-namespace", "test-snapshot", rkev1.ETCDSnapshot{}),
			handlerFunc: func(ctrl *gomock.Controller) handler {
				clusterCache := fake.NewMockCacheInterface[*provv1.Cluster](ctrl)
				cluster := &provv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-namespace",
						Name:      "test-cluster",
					},
					Status: provv1.ClusterStatus{
						ClusterName: "test-mgmt-cluster",
					},
				}
				clusterCache.EXPECT().GetByIndex(cluster2.ByCluster, "test-mgmt-cluster").Return([]*provv1.Cluster{cluster}, nil)
				h := handler{
					clusterName:  cluster.Status.ClusterName,
					clusterCache: clusterCache,
				}
				return h
			},
			expectErr: false,
		},
		{
			name: "snapshot from other cluster",
			snapshot: rkev1.NewETCDSnapshot("test-namespace", "test-snapshot", rkev1.ETCDSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						capr.ClusterNameLabel: "not-this-cluster",
					},
				},
			}),
			handlerFunc: func(ctrl *gomock.Controller) handler {
				clusterCache := fake.NewMockCacheInterface[*provv1.Cluster](ctrl)
				cluster := &provv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-namespace",
						Name:      "test-cluster",
					},
					Status: provv1.ClusterStatus{
						ClusterName: "test-mgmt-cluster",
					},
				}
				clusterCache.EXPECT().GetByIndex(cluster2.ByCluster, "test-mgmt-cluster").Return([]*provv1.Cluster{cluster}, nil)
				h := handler{
					clusterName:  cluster.Status.ClusterName,
					clusterCache: clusterCache,
				}
				return h
			},
			expectErr: false,
		},
		{
			name: "no matching controlplane",
			snapshot: rkev1.NewETCDSnapshot("test-namespace", "test-snapshot", rkev1.ETCDSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						capr.ClusterNameLabel: "test-cluster",
					},
				},
			}),
			handlerFunc: func(ctrl *gomock.Controller) handler {
				clusterCache := fake.NewMockCacheInterface[*provv1.Cluster](ctrl)
				cluster := &provv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-namespace",
						Name:      "test-cluster",
					},
					Status: provv1.ClusterStatus{
						ClusterName: "test-mgmt-cluster",
					},
				}
				clusterCache.EXPECT().GetByIndex(cluster2.ByCluster, cluster.Status.ClusterName).Return([]*provv1.Cluster{cluster}, nil)
				controlPlaneCache := fake.NewMockCacheInterface[*rkev1.RKEControlPlane](ctrl)
				controlPlaneCache.EXPECT().Get(cluster.Namespace, cluster.Name).Return(nil, errors.New("not found"))
				h := handler{
					clusterName:       cluster.Status.ClusterName,
					clusterCache:      clusterCache,
					controlPlaneCache: controlPlaneCache,
				}
				return h
			},
			expectErr: true,
		},
		{
			name: "controlplane mid restore",
			snapshot: rkev1.NewETCDSnapshot("test-namespace", "test-snapshot", rkev1.ETCDSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						capr.ClusterNameLabel: "test-cluster",
					},
				},
			}),
			handlerFunc: func(ctrl *gomock.Controller) handler {
				clusterCache := fake.NewMockCacheInterface[*provv1.Cluster](ctrl)
				cluster := &provv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-namespace",
						Name:      "test-cluster",
					},
					Status: provv1.ClusterStatus{
						ClusterName: "test-mgmt-cluster",
					},
				}
				clusterCache.EXPECT().GetByIndex(cluster2.ByCluster, cluster.Status.ClusterName).Return([]*provv1.Cluster{cluster}, nil)
				controlPlaneCache := fake.NewMockCacheInterface[*rkev1.RKEControlPlane](ctrl)
				controlPlane := &rkev1.RKEControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-namespace",
						Name:      "test-cluster",
					},
					Spec: rkev1.RKEControlPlaneSpec{
						ETCDSnapshotRestore: &rkev1.ETCDSnapshotRestore{
							Generation: 1,
						},
					},
					Status: rkev1.RKEControlPlaneStatus{
						ETCDSnapshotRestore: &rkev1.ETCDSnapshotRestore{
							Generation: 0,
						},
					},
				}
				controlPlaneCache.EXPECT().Get(cluster.Namespace, cluster.Name).Return(controlPlane, nil)
				etcdSnapshotController := fake.NewMockControllerInterface[*rkev1.ETCDSnapshot, *rkev1.ETCDSnapshotList](ctrl)
				etcdSnapshotController.EXPECT().EnqueueAfter(cluster.Namespace, "test-snapshot", gomock.Any())
				h := handler{
					clusterName:            cluster.Status.ClusterName,
					clusterCache:           clusterCache,
					controlPlaneCache:      controlPlaneCache,
					etcdSnapshotController: etcdSnapshotController,
				}
				return h
			},
			expectErr: false,
		},
		{
			name: "snapshot has no annotations",
			snapshot: rkev1.NewETCDSnapshot("test-namespace", "test-snapshot", rkev1.ETCDSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						capr.ClusterNameLabel: "test-cluster",
					},
				},
			}),
			handlerFunc: func(ctrl *gomock.Controller) handler {
				clusterCache := fake.NewMockCacheInterface[*provv1.Cluster](ctrl)
				cluster := &provv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-namespace",
						Name:      "test-cluster",
					},
					Status: provv1.ClusterStatus{
						ClusterName: "test-mgmt-cluster",
					},
				}
				clusterCache.EXPECT().GetByIndex(cluster2.ByCluster, cluster.Status.ClusterName).Return([]*provv1.Cluster{cluster}, nil)
				controlPlaneCache := fake.NewMockCacheInterface[*rkev1.RKEControlPlane](ctrl)
				controlPlane := &rkev1.RKEControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-namespace",
						Name:      "test-cluster",
					},
				}
				controlPlaneCache.EXPECT().Get(cluster.Namespace, cluster.Name).Return(controlPlane, nil)
				h := handler{
					clusterName:       cluster.Status.ClusterName,
					clusterCache:      clusterCache,
					controlPlaneCache: controlPlaneCache,
				}
				return h
			},
			expectErr: false,
		},
		{
			name: "downstream snapshot exists",
			snapshot: rkev1.NewETCDSnapshot("test-namespace", "test-snapshot", rkev1.ETCDSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						capr.SnapshotNameAnnotation: "test-snapshot-downstream",
					},
					Labels: map[string]string{
						capr.ClusterNameLabel: "test-cluster",
					},
				},
			}),
			handlerFunc: func(ctrl *gomock.Controller) handler {
				clusterCache := fake.NewMockCacheInterface[*provv1.Cluster](ctrl)
				cluster := &provv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-namespace",
						Name:      "test-cluster",
					},
					Status: provv1.ClusterStatus{
						ClusterName: "test-mgmt-cluster",
					},
				}
				clusterCache.EXPECT().GetByIndex(cluster2.ByCluster, cluster.Status.ClusterName).Return([]*provv1.Cluster{cluster}, nil)
				controlPlaneCache := fake.NewMockCacheInterface[*rkev1.RKEControlPlane](ctrl)
				controlPlane := &rkev1.RKEControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-namespace",
						Name:      "test-cluster",
					},
				}
				controlPlaneCache.EXPECT().Get(cluster.Namespace, cluster.Name).Return(controlPlane, nil)
				etcdSnapshotFileController := fake.NewMockNonNamespacedControllerInterface[*k3s.ETCDSnapshotFile, *k3s.ETCDSnapshotFileList](ctrl)
				etcdSnapshotFileController.EXPECT().Get("test-snapshot-downstream", gomock.Any()).Return(&k3s.ETCDSnapshotFile{}, nil)
				h := handler{
					clusterName:                cluster.Status.ClusterName,
					clusterCache:               clusterCache,
					controlPlaneCache:          controlPlaneCache,
					etcdSnapshotFileController: etcdSnapshotFileController,
				}
				return h
			},
			expectErr: false,
		},
		{
			name: "downstream snapshot does not exist",
			snapshot: rkev1.NewETCDSnapshot("test-namespace", "test-snapshot", rkev1.ETCDSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						capr.SnapshotNameAnnotation: "test-snapshot-downstream",
					},
					Labels: map[string]string{
						capr.ClusterNameLabel: "test-cluster",
					},
				},
			}),
			handlerFunc: func(ctrl *gomock.Controller) handler {
				clusterCache := fake.NewMockCacheInterface[*provv1.Cluster](ctrl)
				cluster := &provv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-namespace",
						Name:      "test-cluster",
					},
					Status: provv1.ClusterStatus{
						ClusterName: "test-mgmt-cluster",
					},
				}
				clusterCache.EXPECT().GetByIndex(cluster2.ByCluster, cluster.Status.ClusterName).Return([]*provv1.Cluster{cluster}, nil)
				controlPlaneCache := fake.NewMockCacheInterface[*rkev1.RKEControlPlane](ctrl)
				controlPlane := &rkev1.RKEControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-namespace",
						Name:      "test-cluster",
					},
				}
				controlPlaneCache.EXPECT().Get(cluster.Namespace, cluster.Name).Return(controlPlane, nil)
				etcdSnapshotFileController := fake.NewMockNonNamespacedControllerInterface[*k3s.ETCDSnapshotFile, *k3s.ETCDSnapshotFileList](ctrl)
				etcdSnapshotFileController.EXPECT().Get("test-snapshot-downstream", gomock.Any()).Return(nil, apierrors.NewNotFound(schema.GroupResource{}, "test-snapshot-downstream"))
				etcdSnapshotController := fake.NewMockControllerInterface[*rkev1.ETCDSnapshot, *rkev1.ETCDSnapshotList](ctrl)
				etcdSnapshotController.EXPECT().Delete(cluster.Namespace, "test-snapshot", gomock.Any()).Return(nil)
				h := handler{
					clusterName:                cluster.Status.ClusterName,
					clusterCache:               clusterCache,
					controlPlaneCache:          controlPlaneCache,
					etcdSnapshotFileController: etcdSnapshotFileController,
					etcdSnapshotController:     etcdSnapshotController,
				}
				return h
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			h := tt.handlerFunc(ctrl)
			snapshotCopy := tt.snapshot.DeepCopy()
			snapshot, err := h.OnUpstreamChange("", snapshotCopy)
			assert.Equal(t, snapshotCopy, tt.snapshot, "OnUpstreamChange should not modify the snapshot parameter")
			if snapshot != nil {
				assert.Equal(t, snapshotCopy, snapshot, "OnUpstreamChange should return the same input")
			}
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOnDownstreamChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	clusterCache := fake.NewMockCacheInterface[*provv1.Cluster](ctrl)
	controlPlaneCache := fake.NewMockCacheInterface[*rkev1.RKEControlPlane](ctrl)
	etcdSnapshotCache := fake.NewMockCacheInterface[*rkev1.ETCDSnapshot](ctrl)
	etcdSnapshotController := fake.NewMockControllerInterface[*rkev1.ETCDSnapshot, *rkev1.ETCDSnapshotList](ctrl)
	etcdSnapshotFileController := fake.NewMockNonNamespacedControllerInterface[*k3s.ETCDSnapshotFile, *k3s.ETCDSnapshotFileList](ctrl)
	machineCache := fake.NewMockCacheInterface[*capi.Machine](ctrl)
	capiClusterCache := fake.NewMockCacheInterface[*capi.Cluster](ctrl)

	h := handler{
		clusterName:                "test-mgmt-cluster",
		clusterCache:               clusterCache,
		controlPlaneCache:          controlPlaneCache,
		etcdSnapshotCache:          etcdSnapshotCache,
		etcdSnapshotController:     etcdSnapshotController,
		machineCache:               machineCache,
		capiClusterCache:           capiClusterCache,
		etcdSnapshotFileController: etcdSnapshotFileController,
	}
	var snapshot *k3s.ETCDSnapshotFile

	cluster := &provv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:         "test-namespace",
			Name:              "test-cluster",
			DeletionTimestamp: &metav1.Time{Time: time.Now()},
		},
	}
	controlPlane := &rkev1.RKEControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      "test-cluster",
		},
	}
	capiCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      "test-cluster",
		},
	}
	capiMachine := &capi.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      "test-machine",
		},
	}
	upstreamSnapshot := &rkev1.ETCDSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      "test-snapshot-upstream",
		},
	}

	// Nil snapshot

	_, err := h.OnDownstreamChange("", snapshot)
	assert.NoError(t, err, "A nil snapshot should return immediately")

	// Provisioning cluster not found

	clusterCache.EXPECT().GetByIndex(cluster2.ByCluster, h.clusterName).Return(nil, errors.New("not found")).Times(1)

	snapshot = &k3s.ETCDSnapshotFile{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      "test-snapshot-downstream",
		},
	}

	_, err = h.OnDownstreamChange("", snapshot)
	assert.Error(t, err, "An error is expected if the provisioning cluster cannot be found")

	// Provisioning cluster being deleted

	clusterCache.EXPECT().GetByIndex(cluster2.ByCluster, h.clusterName).Return([]*provv1.Cluster{cluster}, nil).AnyTimes()

	_, err = h.OnDownstreamChange("", snapshot)
	assert.NoError(t, err, "It should return early if the provisioning cluster is being deleted")

	// No matching snapshots

	cluster.DeletionTimestamp = nil
	snapshot.DeletionTimestamp = &metav1.Time{Time: time.Now()}

	etcdSnapshotCache.EXPECT().GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/test-snapshot-downstream").Return([]*rkev1.ETCDSnapshot{}, nil).Times(1)

	_, err = h.OnDownstreamChange("", snapshot)
	assert.NoError(t, err, "It should not return an error if the snapshot is being deleted and there are no upstream snapshots")

	// One matching snapshot

	etcdSnapshotCache.EXPECT().GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/test-snapshot-downstream").Return([]*rkev1.ETCDSnapshot{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test-namespace",
				Name:      "test-snapshot-upstream",
			},
		},
	}, nil).Times(1)
	etcdSnapshotController.EXPECT().Delete(cluster.Namespace, "test-snapshot-upstream", gomock.Any()).Return(nil).Times(1)

	_, err = h.OnDownstreamChange("", snapshot)
	assert.NoError(t, err, "It should not return an error if the snapshot is being deleted and there is 1 upstream snapshot")

	// Multiple matching snapshots

	etcdSnapshotCache.EXPECT().GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/test-snapshot-downstream").Return([]*rkev1.ETCDSnapshot{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test-namespace",
				Name:      "test-snapshot-upstream-0",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test-namespace",
				Name:      "test-snapshot-upstream-1",
			},
		},
	}, nil).Times(1)
	etcdSnapshotController.EXPECT().Delete(cluster.Namespace, "test-snapshot-upstream-0", gomock.Any()).Return(nil).Times(1)
	etcdSnapshotController.EXPECT().Delete(cluster.Namespace, "test-snapshot-upstream-1", gomock.Any()).Return(nil).Times(1)

	_, err = h.OnDownstreamChange("", snapshot)
	assert.NoError(t, err, "It should not return an error if the snapshot is being deleted and there are 2 upstream snapshots")

	// Controlplane not found

	snapshot.DeletionTimestamp = nil
	controlPlaneCache.EXPECT().Get(cluster.Namespace, cluster.Name).Return(nil, errors.New("not found")).Times(1)

	_, err = h.OnDownstreamChange("", snapshot)
	assert.Error(t, err, "It should return an error if the controlplane cannot be found")

	// Mid ETCD Restore

	controlPlane.Spec.ETCDSnapshotRestore = &rkev1.ETCDSnapshotRestore{
		Generation: 1,
	}
	controlPlane.Status.ETCDSnapshotRestore = &rkev1.ETCDSnapshotRestore{
		Generation: 0,
	}
	controlPlaneCache.EXPECT().Get(cluster.Namespace, cluster.Name).Return(controlPlane, nil).AnyTimes()
	etcdSnapshotFileController.EXPECT().EnqueueAfter(snapshot.Name, gomock.Any()).Times(1)

	_, err = h.OnDownstreamChange("", snapshot)
	assert.NoError(t, err, "It should return early if the controlplane is mid restore")

	// Error getting snapshots from index

	controlPlane.Labels = map[string]string{
		capi.ClusterNameLabel: cluster.Name,
	}
	controlPlane.Spec = rkev1.RKEControlPlaneSpec{}
	controlPlane.Status = rkev1.RKEControlPlaneStatus{}
	capiClusterCache.EXPECT().Get(cluster.Namespace, cluster.Name).Return(capiCluster, nil).AnyTimes()
	etcdSnapshotCache.EXPECT().GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/test-snapshot-downstream").Return(nil, errors.New("error")).Times(1)

	_, err = h.OnDownstreamChange("", snapshot)
	assert.Error(t, err, "It should return an error if getting snapshots from the index fails")

	// no snapshots and no machines

	machineCache.EXPECT().List(cluster.Namespace, labels.SelectorFromSet(labels.Set{capi.ClusterNameLabel: cluster.Name})).Return(nil, nil)
	etcdSnapshotCache.EXPECT().GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/test-snapshot-downstream").Return([]*rkev1.ETCDSnapshot{}, nil).Times(2)

	_, err = h.OnDownstreamChange("", snapshot)
	assert.Error(t, err, "It should return an error if no machine match the node")

	// create upstream snapshot

	machineCache.EXPECT().List(cluster.Namespace, labels.SelectorFromSet(labels.Set{capi.ClusterNameLabel: cluster.Name})).Return([]*capi.Machine{capiMachine}, nil).AnyTimes()
	etcdSnapshotController.EXPECT().Create(gomock.Any()).Return(nil, nil).Times(1)

	snapshot.Spec.NodeName = "test-node"
	capiMachine.Status.NodeRef = &corev1.ObjectReference{
		Name: "test-node",
	}

	_, err = h.OnDownstreamChange("", snapshot)
	assert.NoError(t, err, "It should not return an error when creating the machine")

	// No matching snapshots but exists upstream

	etcdSnapshotCache.EXPECT().GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/test-snapshot-downstream").Return([]*rkev1.ETCDSnapshot{}, nil).Times(1)

	etcdSnapshotController.EXPECT().Create(gomock.Any()).Return(nil, apierrors.NewAlreadyExists(schema.GroupResource{}, "error")).Times(1)

	existingSnapshot := upstreamSnapshot.DeepCopy()
	existingSnapshot.ResourceVersion = "1"

	etcdSnapshotCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(existingSnapshot, nil).Times(1)
	etcdSnapshotController.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *rkev1.ETCDSnapshot) (*rkev1.ETCDSnapshot, error) {
		assert.Equal(t, "1", s.ResourceVersion)
		s.ResourceVersion = "2"
		return s, nil
	})

	_, err = h.OnDownstreamChange("", snapshot)
	assert.NoError(t, err, "It should update the snapshot if one exists but could not be found by the indexer")

	// No matching snapshots but does not exist upstream

	etcdSnapshotCache.EXPECT().GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/test-snapshot-downstream").Return([]*rkev1.ETCDSnapshot{}, nil).Times(1)

	etcdSnapshotController.EXPECT().Create(gomock.Any()).DoAndReturn(func(s *rkev1.ETCDSnapshot) (*rkev1.ETCDSnapshot, error) {
		assert.Equal(t, "", s.ResourceVersion)
		s.ResourceVersion = "1"
		return s, nil
	})

	_, err = h.OnDownstreamChange("", snapshot)
	assert.NoError(t, err, "It should create the snapshot if one does not exist")

	// delete multiple snapshots

	toDelete := []*rkev1.ETCDSnapshot{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test-namespace",
				Name:      "test-snapshot-upstream-0",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test-namespace",
				Name:      "test-snapshot-upstream-1",
			},
		},
	}
	etcdSnapshotCache.EXPECT().GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/test-snapshot-downstream").Return(toDelete, nil).Times(1)

	for _, e := range toDelete {
		etcdSnapshotController.EXPECT().Delete(e.Namespace, e.Name, gomock.Any()).Return(nil).Times(1)
	}
	etcdSnapshotFileController.EXPECT().Enqueue(snapshot.Name).Times(1)

	_, err = h.OnDownstreamChange("", snapshot)
	assert.NoError(t, err, "It should not return an error when deleting the snapshot")

	// update existing snapshots
	etcdSnapshotCache.EXPECT().GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/test-snapshot-downstream").Return([]*rkev1.ETCDSnapshot{upstreamSnapshot}, nil).AnyTimes()

	etcdSnapshotController.EXPECT().Patch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(upstreamSnapshot, nil).Times(1)

	_, err = h.OnDownstreamChange("", snapshot)
	assert.NoError(t, err, "It should not return an error when update the snapshot")
}

func TestOnDownstreamChange_S3FallbackNameGeneration_Metadata_Is_Empty(t *testing.T) {
	controller := gomock.NewController(t)

	clusterCache := fake.NewMockCacheInterface[*provv1.Cluster](controller)
	controlPlaneCache := fake.NewMockCacheInterface[*rkev1.RKEControlPlane](controller)
	etcdSnapshotCache := fake.NewMockCacheInterface[*rkev1.ETCDSnapshot](controller)
	etcdSnapshotController := fake.NewMockControllerInterface[*rkev1.ETCDSnapshot, *rkev1.ETCDSnapshotList](controller)
	machineCache := fake.NewMockCacheInterface[*capi.Machine](controller)
	capiClusterCache := fake.NewMockCacheInterface[*capi.Cluster](controller)
	etcdSnapshotFileController := fake.NewMockNonNamespacedControllerInterface[*k3s.ETCDSnapshotFile, *k3s.ETCDSnapshotFileList](controller)

	handlerUnderTest := handler{
		clusterName:                "test-mgmt-cluster",
		clusterCache:               clusterCache,
		controlPlaneCache:          controlPlaneCache,
		etcdSnapshotCache:          etcdSnapshotCache,
		etcdSnapshotController:     etcdSnapshotController,
		machineCache:               machineCache,
		capiClusterCache:           capiClusterCache,
		etcdSnapshotFileController: etcdSnapshotFileController,
	}

	provisioningCluster := &provv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      "test-cluster",
		},
		Status: provv1.ClusterStatus{
			ClusterName: "test-mgmt-cluster",
		},
	}
	controlPlane := &rkev1.RKEControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      "test-cluster",
			Labels: map[string]string{
				capi.ClusterNameLabel: provisioningCluster.Name,
			},
		},
	}
	capiCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      "test-cluster",
		},
	}
	machine := &capi.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      "test-machine",
			Labels: map[string]string{
				capi.ClusterNameLabel: provisioningCluster.Name,
			},
		},
		Status: capi.MachineStatus{
			NodeRef: &corev1.ObjectReference{Name: "cp-0"},
		},
	}
	downstreamSnapshot := &k3s.ETCDSnapshotFile{
		ObjectMeta: metav1.ObjectMeta{
			Name: "downstream-bad",
		},
		Spec: k3s.ETCDSnapshotSpec{
			SnapshotName: "something.something.-.s3-.com",
			NodeName:     "cp-0",
			Location:     "s3://bucket/prefix/key",
			S3:           &k3s.ETCDSnapshotS3{},
		},
		Status: k3s.ETCDSnapshotStatus{
			CreationTime: &metav1.Time{Time: time.Now()},
			ReadyToUse:   ptr.To(true),
		},
	}

	clusterCache.EXPECT().
		GetByIndex(cluster2.ByCluster, handlerUnderTest.clusterName).
		Return([]*provv1.Cluster{provisioningCluster}, nil).
		Times(1)

	controlPlaneCache.EXPECT().
		Get(provisioningCluster.Namespace, provisioningCluster.Name).
		Return(controlPlane, nil).
		Times(1)

	capiClusterCache.EXPECT().
		Get(provisioningCluster.Namespace, provisioningCluster.Name).
		Return(capiCluster, nil).
		AnyTimes()

	etcdSnapshotCache.EXPECT().
		GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/downstream-bad").
		Return([]*rkev1.ETCDSnapshot{}, nil).
		Times(1)

	machineCache.EXPECT().
		List(provisioningCluster.Namespace, gomock.AssignableToTypeOf(labels.Selector(nil))).
		Return([]*capi.Machine{machine}, nil).
		AnyTimes()

	var callbackErr error
	etcdSnapshotController.EXPECT().
		Create(gomock.Any()).
		DoAndReturn(func(obj *rkev1.ETCDSnapshot) (*rkev1.ETCDSnapshot, error) {
			if errs := validation.IsDNS1123Subdomain(obj.Name); len(errs) != 0 {
				callbackErr = fmt.Errorf("dns1123 failed: %v", errs)
				return obj, nil
			}

			if len(obj.Name)+13 > validation.DNS1123SubdomainMaxLength {
				callbackErr = errors.New("dns1123 failed: too long")
				return obj, nil
			}

			annotations := obj.GetAnnotations()
			require.NotNil(t, annotations)
			require.Equal(t, downstreamSnapshot.Spec.SnapshotName, annotations[SnapshotOriginalNameKey])
			require.Equal(t, FallbackGeneratedStrategyName, annotations[SnapshotNameStrategyKey])

			require.Equal(t, "failed", obj.SnapshotFile.Status)
			require.Contains(t, obj.SnapshotFile.Message, EncodedMetadataIsEmptyMessage)

			return obj, nil
		}).
		Times(1)

	_, err := handlerUnderTest.OnDownstreamChange("", downstreamSnapshot)
	require.NoError(t, err)
	require.NoError(t, callbackErr)
}

func TestOnDownstreamChange_Local_Metadata_Is_Empty(t *testing.T) {
	ctrl := gomock.NewController(t)

	clusterCache := fake.NewMockCacheInterface[*provv1.Cluster](ctrl)
	controlPlaneCache := fake.NewMockCacheInterface[*rkev1.RKEControlPlane](ctrl)
	etcdSnapshotCache := fake.NewMockCacheInterface[*rkev1.ETCDSnapshot](ctrl)
	etcdSnapshotController := fake.NewMockControllerInterface[*rkev1.ETCDSnapshot, *rkev1.ETCDSnapshotList](ctrl)
	machineCache := fake.NewMockCacheInterface[*capi.Machine](ctrl)
	capiClusterCache := fake.NewMockCacheInterface[*capi.Cluster](ctrl)
	etcdSnapshotFileController := fake.NewMockNonNamespacedControllerInterface[*k3s.ETCDSnapshotFile, *k3s.ETCDSnapshotFileList](ctrl)

	h := handler{
		clusterName:                "test-mgmt-cluster",
		clusterCache:               clusterCache,
		controlPlaneCache:          controlPlaneCache,
		etcdSnapshotCache:          etcdSnapshotCache,
		etcdSnapshotController:     etcdSnapshotController,
		machineCache:               machineCache,
		capiClusterCache:           capiClusterCache,
		etcdSnapshotFileController: etcdSnapshotFileController,
	}

	cluster := &provv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test-namespace", Name: "test-cluster"},
		Status:     provv1.ClusterStatus{ClusterName: "test-mgmt-cluster"},
	}
	controlPlane := &rkev1.RKEControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace", Name: "test-cluster",
			Labels: map[string]string{capi.ClusterNameLabel: cluster.Name},
		},
	}

	// Local snapshot, empty metadata, ReadyToUse true
	snap := &k3s.ETCDSnapshotFile{
		ObjectMeta: metav1.ObjectMeta{Name: "local-empty-metadata"},
		Spec: k3s.ETCDSnapshotSpec{
			SnapshotName: "etcd-snapshot-cp-0-1700000000",
			NodeName:     "cp-0",
			Location:     "file:///var/lib/rancher/etcd",
			Metadata:     nil,
		},
		Status: k3s.ETCDSnapshotStatus{
			CreationTime: &metav1.Time{Time: time.Now()},
			ReadyToUse:   ptr.To(true),
		},
	}

	clusterCache.EXPECT().GetByIndex(cluster2.ByCluster, h.clusterName).
		Return([]*provv1.Cluster{cluster}, nil)
	controlPlaneCache.EXPECT().Get(cluster.Namespace, cluster.Name).
		Return(controlPlane, nil)
	etcdSnapshotCache.EXPECT().
		GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/local-empty-metadata").
		Return([]*rkev1.ETCDSnapshot{}, nil)

	// Machine lookup for local
	machineCache.EXPECT().
		List(cluster.Namespace, labels.SelectorFromSet(labels.Set{capi.ClusterNameLabel: cluster.Name})).
		Return([]*capi.Machine{
			{Status: capi.MachineStatus{NodeRef: &corev1.ObjectReference{Name: "cp-0"}}},
		}, nil).
		AnyTimes()

	etcdSnapshotController.EXPECT().
		Create(gomock.Any()).
		DoAndReturn(func(obj *rkev1.ETCDSnapshot) (*rkev1.ETCDSnapshot, error) {
			require.Equal(t, "failed", obj.SnapshotFile.Status)
			require.Contains(t, obj.SnapshotFile.Message, EncodedMetadataIsEmptyMessage)
			return obj, nil
		})

	_, err := h.OnDownstreamChange("", snap)
	require.NoError(t, err)
}

func TestOnDownstreamChange_FallbackNameGeneration_Metadata_Not_Empty(t *testing.T) {
	controller := gomock.NewController(t)

	clusterCache := fake.NewMockCacheInterface[*provv1.Cluster](controller)
	controlPlaneCache := fake.NewMockCacheInterface[*rkev1.RKEControlPlane](controller)
	etcdSnapshotCache := fake.NewMockCacheInterface[*rkev1.ETCDSnapshot](controller)
	etcdSnapshotController := fake.NewMockControllerInterface[*rkev1.ETCDSnapshot, *rkev1.ETCDSnapshotList](controller)
	machineCache := fake.NewMockCacheInterface[*capi.Machine](controller)
	capiClusterCache := fake.NewMockCacheInterface[*capi.Cluster](controller)
	etcdSnapshotFileController := fake.NewMockNonNamespacedControllerInterface[*k3s.ETCDSnapshotFile, *k3s.ETCDSnapshotFileList](controller)

	h := handler{
		clusterName:                "test-mgmt-cluster",
		clusterCache:               clusterCache,
		controlPlaneCache:          controlPlaneCache,
		etcdSnapshotCache:          etcdSnapshotCache,
		etcdSnapshotController:     etcdSnapshotController,
		machineCache:               machineCache,
		capiClusterCache:           capiClusterCache,
		etcdSnapshotFileController: etcdSnapshotFileController,
	}

	provisioningCluster := &provv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test-namespace", Name: "test-cluster"},
		Status:     provv1.ClusterStatus{ClusterName: "test-mgmt-cluster"},
	}
	controlPlane := &rkev1.RKEControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace", Name: "test-cluster",
			Labels: map[string]string{capi.ClusterNameLabel: provisioningCluster.Name},
		},
	}
	capiCluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test-namespace", Name: "test-cluster"},
	}

	// Downstream S3 snapshot WITH metadata
	downstreamSnapshot := &k3s.ETCDSnapshotFile{
		ObjectMeta: metav1.ObjectMeta{
			Name: "downstream-bad",
		},
		Spec: k3s.ETCDSnapshotSpec{
			SnapshotName: "something.something.-.s3-.com",
			NodeName:     "cp-0",
			Location:     "s3://bucket/prefix/key",
			S3:           &k3s.ETCDSnapshotS3{},
			Metadata:     map[string]string{"provisioning-cluster-spec": "compressed-blob"},
		},
		Status: k3s.ETCDSnapshotStatus{
			CreationTime: &metav1.Time{Time: time.Now()},
			ReadyToUse:   ptr.To(true),
		},
	}

	clusterCache.EXPECT().GetByIndex(cluster2.ByCluster, h.clusterName).
		Return([]*provv1.Cluster{provisioningCluster}, nil)
	controlPlaneCache.EXPECT().Get(provisioningCluster.Namespace, provisioningCluster.Name).
		Return(controlPlane, nil)
	capiClusterCache.EXPECT().Get(provisioningCluster.Namespace, provisioningCluster.Name).
		Return(capiCluster, nil).
		AnyTimes()
	etcdSnapshotCache.EXPECT().
		GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/downstream-bad").
		Return([]*rkev1.ETCDSnapshot{}, nil)
	machineCache.EXPECT().
		List(provisioningCluster.Namespace, gomock.AssignableToTypeOf(labels.Selector(nil))).
		Return([]*capi.Machine{
			{Status: capi.MachineStatus{NodeRef: &corev1.ObjectReference{Name: "cp-0"}}},
		}, nil).
		AnyTimes()

	etcdSnapshotController.EXPECT().
		Create(gomock.Any()).
		DoAndReturn(func(obj *rkev1.ETCDSnapshot) (*rkev1.ETCDSnapshot, error) {
			require.Equal(t, "successful", obj.SnapshotFile.Status)
			return obj, nil
		})

	_, err := h.OnDownstreamChange("", downstreamSnapshot)
	require.NoError(t, err)
}

func TestGetCluster(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string

		clusterName string

		cacheFunc func(cache *fake.MockCacheInterface[*provv1.Cluster])

		expectedCluster *provv1.Cluster
		expectErr       bool
	}{
		{
			name:        "nil result",
			clusterName: "test-cluster",
			cacheFunc: func(cache *fake.MockCacheInterface[*provv1.Cluster]) {
				cache.EXPECT().GetByIndex(cluster2.ByCluster, "test-cluster").Return(nil, nil)
			},
			expectedCluster: nil,
			expectErr:       true,
		},
		{
			name:        "empty result",
			clusterName: "test-cluster",
			cacheFunc: func(cache *fake.MockCacheInterface[*provv1.Cluster]) {
				cache.EXPECT().GetByIndex(cluster2.ByCluster, "test-cluster").Return([]*provv1.Cluster{}, nil)
			},
			expectedCluster: nil,
			expectErr:       true,
		},
		{
			name:        "error from index",
			clusterName: "test-cluster",
			cacheFunc: func(cache *fake.MockCacheInterface[*provv1.Cluster]) {
				cache.EXPECT().GetByIndex(cluster2.ByCluster, "test-cluster").Return(nil, errors.New("error from index"))
			},
			expectedCluster: nil,
			expectErr:       true,
		},
		{
			name:        "match from index",
			clusterName: "test-cluster",
			cacheFunc: func(cache *fake.MockCacheInterface[*provv1.Cluster]) {
				cache.EXPECT().GetByIndex(cluster2.ByCluster, "test-cluster").Return([]*provv1.Cluster{
					{
						Status: provv1.ClusterStatus{
							ClusterName: "test-cluster",
						},
					},
				}, nil)
			},
			expectedCluster: &provv1.Cluster{
				Status: provv1.ClusterStatus{
					ClusterName: "test-cluster",
				},
			},
			expectErr: false,
		},
		{
			name:        "multiple matches from index",
			clusterName: "test-cluster",
			cacheFunc: func(cache *fake.MockCacheInterface[*provv1.Cluster]) {
				cache.EXPECT().GetByIndex(cluster2.ByCluster, "test-cluster").Return([]*provv1.Cluster{
					{
						Status: provv1.ClusterStatus{
							ClusterName: "test-cluster",
						},
					},
					{
						Status: provv1.ClusterStatus{
							ClusterName: "test-cluster",
						},
					},
				}, nil)
			},
			expectedCluster: nil,
			expectErr:       true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			cache := fake.NewMockCacheInterface[*provv1.Cluster](ctrl)
			tt.cacheFunc(cache)
			h := handler{
				clusterName: tt.clusterName,
			}
			h.clusterCache = cache
			cluster, err := h.getCluster()
			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, cluster)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedCluster, cluster)
			}
		})
	}
}

func TestGetSnapshotsFromSnapshotFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string

		cluster      *provv1.Cluster
		snapshotFile *k3s.ETCDSnapshotFile

		cacheFunc func(cache *fake.MockCacheInterface[*rkev1.ETCDSnapshot])

		expectedSnapshots []*rkev1.ETCDSnapshot
		expectErr         bool
	}{
		{
			name:         "nil result",
			snapshotFile: k3s.NewETCDSnapshotFile("", "test-snapshot", k3s.ETCDSnapshotFile{}),
			cluster:      provv1.NewCluster("test-namespace", "test-cluster", provv1.Cluster{}),
			cacheFunc: func(cache *fake.MockCacheInterface[*rkev1.ETCDSnapshot]) {
				cache.EXPECT().GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/test-snapshot").Return(nil, nil)
			},
			expectedSnapshots: nil,
			expectErr:         false,
		},
		{
			name:         "empty result",
			snapshotFile: k3s.NewETCDSnapshotFile("", "test-snapshot", k3s.ETCDSnapshotFile{}),
			cluster:      provv1.NewCluster("test-namespace", "test-cluster", provv1.Cluster{}),
			cacheFunc: func(cache *fake.MockCacheInterface[*rkev1.ETCDSnapshot]) {
				cache.EXPECT().GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/test-snapshot").Return([]*rkev1.ETCDSnapshot{}, nil)
			},
			expectedSnapshots: []*rkev1.ETCDSnapshot{},
			expectErr:         false,
		},
		{
			name:         "error from index",
			snapshotFile: k3s.NewETCDSnapshotFile("", "test-snapshot", k3s.ETCDSnapshotFile{}),
			cluster:      provv1.NewCluster("test-namespace", "test-cluster", provv1.Cluster{}),
			cacheFunc: func(cache *fake.MockCacheInterface[*rkev1.ETCDSnapshot]) {
				cache.EXPECT().GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/test-snapshot").Return(nil, errors.New("error from index"))
			},
			expectedSnapshots: []*rkev1.ETCDSnapshot{},
			expectErr:         true,
		},
		{
			name:         "match from index",
			snapshotFile: k3s.NewETCDSnapshotFile("", "test-snapshot", k3s.ETCDSnapshotFile{}),
			cluster:      provv1.NewCluster("test-namespace", "test-cluster", provv1.Cluster{}),
			cacheFunc: func(cache *fake.MockCacheInterface[*rkev1.ETCDSnapshot]) {
				cache.EXPECT().GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/test-snapshot").Return([]*rkev1.ETCDSnapshot{{}}, nil)
			},
			expectedSnapshots: []*rkev1.ETCDSnapshot{{}},
			expectErr:         false,
		},
		{
			// it's valid for multiple objects to be returned from the index if a user creates them, although the controller will delete any duplicates and recreate a single one.
			name:         "multiple matches from index",
			snapshotFile: k3s.NewETCDSnapshotFile("", "test-snapshot", k3s.ETCDSnapshotFile{}),
			cluster:      provv1.NewCluster("test-namespace", "test-cluster", provv1.Cluster{}),
			cacheFunc: func(cache *fake.MockCacheInterface[*rkev1.ETCDSnapshot]) {
				cache.EXPECT().GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/test-snapshot").Return([]*rkev1.ETCDSnapshot{{}, {}}, nil)
			},
			expectedSnapshots: []*rkev1.ETCDSnapshot{{}, {}},
			expectErr:         false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			cache := fake.NewMockCacheInterface[*rkev1.ETCDSnapshot](ctrl)
			tt.cacheFunc(cache)
			h := handler{}
			h.etcdSnapshotCache = cache
			snapshots, err := h.getSnapshotsFromSnapshotFile(tt.cluster, tt.snapshotFile)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Empty(t, snapshots)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedSnapshots, snapshots)
			}
		})
	}
}

func TestGetMachineFromNode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string

		nodeName         string
		clusterName      string
		clusterNamespace string

		cacheFunc func(cache *fake.MockCacheInterface[*capi.Machine])

		expectedMachine *capi.Machine
		expectErr       bool
	}{
		{
			name:             "no machines",
			nodeName:         "test-node",
			clusterName:      "test-cluster",
			clusterNamespace: "test-namespace",
			cacheFunc: func(cache *fake.MockCacheInterface[*capi.Machine]) {
				cache.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*capi.Machine{}, nil)
			},
			expectedMachine: nil,
			expectErr:       true,
		},
		{
			name:             "error from list",
			nodeName:         "test-node",
			clusterName:      "test-cluster",
			clusterNamespace: "test-namespace",
			cacheFunc: func(cache *fake.MockCacheInterface[*capi.Machine]) {
				cache.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, errors.New("list error"))
			},
			expectedMachine: nil,
			expectErr:       true,
		},
		{
			name:             "nil node ref",
			nodeName:         "test-node",
			clusterName:      "test-cluster",
			clusterNamespace: "test-namespace",
			cacheFunc: func(cache *fake.MockCacheInterface[*capi.Machine]) {
				cache.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*capi.Machine{
					{},
				}, nil)
			},
			expectedMachine: nil,
			expectErr:       true,
		},
		{
			name:             "no matching node ref",
			nodeName:         "test-node",
			clusterName:      "test-cluster",
			clusterNamespace: "test-namespace",
			cacheFunc: func(cache *fake.MockCacheInterface[*capi.Machine]) {
				cache.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*capi.Machine{
					{
						Status: capi.MachineStatus{
							NodeRef: &corev1.ObjectReference{
								Name: "not-test-node",
							},
						},
					},
				}, nil)
			},
			expectedMachine: nil,
			expectErr:       true,
		},
		{
			name:             "matching node ref",
			nodeName:         "test-node",
			clusterName:      "test-cluster",
			clusterNamespace: "test-namespace",
			cacheFunc: func(cache *fake.MockCacheInterface[*capi.Machine]) {
				cache.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*capi.Machine{
					{
						Status: capi.MachineStatus{
							NodeRef: &corev1.ObjectReference{
								Name: "test-node",
							},
						},
					},
				}, nil)
			},
			expectedMachine: &capi.Machine{
				Status: capi.MachineStatus{
					NodeRef: &corev1.ObjectReference{
						Name: "test-node",
					},
				},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			machineCache := fake.NewMockCacheInterface[*capi.Machine](ctrl)
			tt.cacheFunc(machineCache)
			h := handler{}
			h.machineCache = machineCache
			machine, err := h.getMachineFromNode(tt.nodeName, tt.clusterName, tt.clusterNamespace)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, machine)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedMachine, machine)
			}
		})
	}
}

func TestGetMachineByID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string

		machineID        string
		clusterName      string
		clusterNamespace string

		cacheFunc func(cache *fake.MockCacheInterface[*capi.Machine])

		expectedMachine *capi.Machine
		expectErr       bool
	}{
		{
			name:             "no machines",
			machineID:        "abcdefg",
			clusterName:      "test-cluster",
			clusterNamespace: "test-namespace",
			cacheFunc: func(cache *fake.MockCacheInterface[*capi.Machine]) {
				cache.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*capi.Machine{}, nil)
			},
			expectedMachine: nil,
			expectErr:       true,
		},
		{
			name:             "multiple matching machines",
			machineID:        "abcdefg",
			clusterName:      "test-cluster",
			clusterNamespace: "test-namespace",
			cacheFunc: func(cache *fake.MockCacheInterface[*capi.Machine]) {
				cache.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*capi.Machine{{}, {}}, nil)
			},
			expectedMachine: nil,
			expectErr:       true,
		},
		{
			name:             "error from list",
			machineID:        "abcdefg",
			clusterName:      "test-cluster",
			clusterNamespace: "test-namespace",
			cacheFunc: func(cache *fake.MockCacheInterface[*capi.Machine]) {
				cache.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, errors.New("list error"))
			},
			expectedMachine: nil,
			expectErr:       true,
		},
		{
			name:             "matching machine",
			machineID:        "abcdefg",
			clusterName:      "test-cluster",
			clusterNamespace: "test-namespace",
			cacheFunc: func(cache *fake.MockCacheInterface[*capi.Machine]) {
				cache.EXPECT().List("test-namespace", labels.SelectorFromSet(labels.Set{
					capr.ClusterNameLabel: "test-cluster",
					capr.MachineIDLabel:   "abcdefg",
				})).Return([]*capi.Machine{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "test-namespace",
							Name:      "test-machine",
						},
					},
				}, nil)
			},
			expectedMachine: &capi.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-machine",
				},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			machineCache := fake.NewMockCacheInterface[*capi.Machine](ctrl)
			tt.cacheFunc(machineCache)
			h := handler{}
			h.machineCache = machineCache
			machine, err := h.getMachineByID(tt.machineID, tt.clusterName, tt.clusterNamespace)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, machine)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedMachine, machine)
			}
		})
	}
}

func TestGetLogPrefix(t *testing.T) {
	assert.Equal(t, "[snapshotbackpopulate] rkecluster test-namespace/test-cluster:", getLogPrefix(&provv1.Cluster{ObjectMeta: metav1.ObjectMeta{Namespace: "test-namespace", Name: "test-cluster"}}))
}

func TestGenerateSafeSnapshotName(t *testing.T) {
	ts := time.Unix(1_700_000_000, 0)

	createEtcdEnapshot := func(name, node, loc string, s3 bool) k3s.ETCDSnapshotSpec {
		var s3ptr *k3s.ETCDSnapshotS3
		if s3 {
			s3ptr = &k3s.ETCDSnapshotS3{}
		}
		return k3s.ETCDSnapshotSpec{
			SnapshotName: name,
			NodeName:     node,
			Location:     loc,
			S3:           s3ptr,
		}
	}

	cases := []struct {
		name             string
		etcdSnapshotSpec k3s.ETCDSnapshotSpec
		wantPrefix       string
	}{
		{
			name:             "Local snapshot: invalid chars, node has domain",
			etcdSnapshotSpec: createEtcdEnapshot("something.something.-.s3-.com", "cp-0.example.com", "file:///var/lib", false),
			wantPrefix:       "local-etcd-snapshot-cp-0-1700000000-",
		},
		{
			name:             "S3 snapshot: overlength basename",
			etcdSnapshotSpec: createEtcdEnapshot(strings.Repeat("a", 250), "cp-0", "s3://bucket/prefix", true),
			wantPrefix:       "s3-etcd-snapshot-cp-0-1700000000-",
		},
		{
			name:             "S3 snapshot: digest depends on location (case A)",
			etcdSnapshotSpec: createEtcdEnapshot("snapshot", "cp-0", "s3://bucket/prefix/A", true),
			wantPrefix:       "s3-etcd-snapshot-cp-0-1700000000-",
		},
	}

	hex6re := regexp.MustCompile(`^[a-f0-9]{6}$`)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := generateSafeSnapshotName(&tc.etcdSnapshotSpec, ts)

			require.True(t, strings.HasPrefix(got, tc.wantPrefix), got)
			require.Equal(t, strings.ToLower(got), got, "name must be lowercase")

			parts := strings.Split(got, "-")
			require.GreaterOrEqual(t, len(parts), 2)
			suffix := parts[len(parts)-1]
			require.Regexp(t, hex6re, suffix, "suffix must be 6 hex chars")

			got2 := generateSafeSnapshotName(&tc.etcdSnapshotSpec, ts)
			assert.Equal(t, got, got2)
		})
	}

	t.Run("Digest differs for different locations", func(t *testing.T) {
		a := createEtcdEnapshot("snapshot", "cp-0", "s3://bucket/prefix/A", true)
		b := createEtcdEnapshot("snapshot", "cp-0", "s3://bucket/prefix/B", true)

		gotA := generateSafeSnapshotName(&a, ts)
		gotB := generateSafeSnapshotName(&b, ts)

		pref := "s3-etcd-snapshot-cp-0-1700000000-"
		require.True(t, strings.HasPrefix(gotA, pref))
		require.True(t, strings.HasPrefix(gotB, pref))

		assert.NotEqual(t, gotA, gotB)
	})
}
