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
	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1/snapshotutil"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	cluster2 "github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
)

// testRESTMapper returns a RESTMapper that resolves the handful of GroupKinds these tests need.
// Namespaced by default; management.cattle.io/v3 Cluster is registered as cluster-scoped so the
// cluster-lifecycle tests can assert the scope-based namespace override.
func testRESTMapper() meta.RESTMapper {
	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{
		{Group: "cluster.x-k8s.io", Version: "v1beta2"},
		{Group: "provisioning.cattle.io", Version: "v1"},
		{Group: "management.cattle.io", Version: "v3"},
	})
	m.Add(schema.GroupVersionKind{Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "Machine"}, meta.RESTScopeNamespace)
	m.Add(schema.GroupVersionKind{Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "Cluster"}, meta.RESTScopeNamespace)
	m.Add(schema.GroupVersionKind{Group: "provisioning.cattle.io", Version: "v1", Kind: "Cluster"}, meta.RESTScopeNamespace)
	m.Add(schema.GroupVersionKind{Group: "management.cattle.io", Version: "v3", Kind: "Node"}, meta.RESTScopeNamespace)
	m.Add(schema.GroupVersionKind{Group: "management.cattle.io", Version: "v3", Kind: "Cluster"}, meta.RESTScopeRoot)
	return m
}

type dynamicClientFake struct {
	obj runtime.Object
	err error
}

func (d *dynamicClientFake) Get(_ schema.GroupVersionKind, _, _ string) (runtime.Object, error) {
	return d.obj, d.err
}

func newUnstructuredCluster(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(apiVersion)
	obj.SetKind(kind)
	obj.SetNamespace(namespace)
	obj.SetName(name)
	return obj
}

func newProvisioningClusterUnstructured(namespace, name string) *unstructured.Unstructured {
	return newUnstructuredCluster("provisioning.cattle.io/v1", "Cluster", namespace, name)
}

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
			handlerFunc: func(_ *gomock.Controller) handler {
				return handler{
					dynamic: &dynamicClientFake{err: errors.New("failed to get cluster")},
					clusterRef: corev1.ObjectReference{
						APIVersion: "provisioning.cattle.io/v1",
						Kind:       "Cluster",
						Namespace:  "test-namespace",
						Name:       "test-cluster",
					},
				}
			},
			expectErr: true,
		},
		{
			name:     "snapshot from different namespace",
			snapshot: rkev1.NewETCDSnapshot("other-namespace", "test-snapshot", rkev1.ETCDSnapshot{}),
			handlerFunc: func(ctrl *gomock.Controller) handler {
				cluster := newProvisioningClusterUnstructured("test-namespace", "test-cluster")
				beaconCache := fake.NewMockCacheInterface[*planv1alpha1.Beacon](ctrl)
				beacon := &planv1alpha1.Beacon{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-namespace",
						Name:      "test-cluster",
					},
				}
				beaconCache.EXPECT().Get(cluster.GetNamespace(), cluster.GetName()).Return(beacon, nil).AnyTimes()
				return handler{
					beaconCache: beaconCache,
					dynamic:     &dynamicClientFake{obj: cluster},
					clusterRef: corev1.ObjectReference{
						APIVersion: "provisioning.cattle.io/v1",
						Kind:       "Cluster",
						Namespace:  "test-namespace",
						Name:       "test-cluster",
					},
				}
			},
			expectErr: false,
		},
		{
			name:     "snapshot has no labels",
			snapshot: rkev1.NewETCDSnapshot("test-namespace", "test-snapshot", rkev1.ETCDSnapshot{}),
			handlerFunc: func(ctrl *gomock.Controller) handler {
				cluster := newProvisioningClusterUnstructured("test-namespace", "test-cluster")
				beaconCache := fake.NewMockCacheInterface[*planv1alpha1.Beacon](ctrl)
				beacon := &planv1alpha1.Beacon{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-namespace",
						Name:      "test-cluster",
					},
				}
				beaconCache.EXPECT().Get(cluster.GetNamespace(), cluster.GetName()).Return(beacon, nil).AnyTimes()
				return handler{
					beaconCache: beaconCache,
					dynamic:     &dynamicClientFake{obj: cluster},
					clusterRef: corev1.ObjectReference{
						APIVersion: "provisioning.cattle.io/v1",
						Kind:       "Cluster",
						Namespace:  "test-namespace",
						Name:       "test-cluster",
					},
				}
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
				cluster := newProvisioningClusterUnstructured("test-namespace", "test-cluster")
				beaconCache := fake.NewMockCacheInterface[*planv1alpha1.Beacon](ctrl)
				beacon := &planv1alpha1.Beacon{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-namespace",
						Name:      "test-cluster",
					},
				}
				beaconCache.EXPECT().Get(cluster.GetNamespace(), cluster.GetName()).Return(beacon, nil).AnyTimes()
				return handler{
					beaconCache: beaconCache,
					dynamic:     &dynamicClientFake{obj: cluster},
					clusterRef: corev1.ObjectReference{
						APIVersion: "provisioning.cattle.io/v1",
						Kind:       "Cluster",
						Namespace:  "test-namespace",
						Name:       "test-cluster",
					},
				}
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
				cluster := newProvisioningClusterUnstructured("test-namespace", "test-cluster")
				beaconCache := fake.NewMockCacheInterface[*planv1alpha1.Beacon](ctrl)
				beacon := &planv1alpha1.Beacon{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-namespace",
						Name:      "test-cluster",
					},
				}
				beaconCache.EXPECT().Get(cluster.GetNamespace(), cluster.GetName()).Return(beacon, nil).AnyTimes()
				return handler{
					beaconCache: beaconCache,
					dynamic:     &dynamicClientFake{obj: cluster},
					clusterRef: corev1.ObjectReference{
						APIVersion: "provisioning.cattle.io/v1",
						Kind:       "Cluster",
						Namespace:  "test-namespace",
						Name:       "test-cluster",
					},
				}
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
				cluster := newProvisioningClusterUnstructured("test-namespace", "test-cluster")
				beaconCache := fake.NewMockCacheInterface[*planv1alpha1.Beacon](ctrl)
				beacon := &planv1alpha1.Beacon{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-namespace",
						Name:      "test-cluster",
					},
				}
				beaconCache.EXPECT().Get(cluster.GetNamespace(), cluster.GetName()).Return(beacon, nil).AnyTimes()
				etcdSnapshotFileController := fake.NewMockNonNamespacedControllerInterface[*k3s.ETCDSnapshotFile, *k3s.ETCDSnapshotFileList](ctrl)
				etcdSnapshotFileController.EXPECT().Get("test-snapshot-downstream", gomock.Any()).Return(&k3s.ETCDSnapshotFile{}, nil)
				return handler{
					beaconCache: beaconCache,
					dynamic:     &dynamicClientFake{obj: cluster},
					clusterRef: corev1.ObjectReference{
						APIVersion: "provisioning.cattle.io/v1",
						Kind:       "Cluster",
						Namespace:  "test-namespace",
						Name:       "test-cluster",
					},
					snapshotNamespace:          "test-namespace",
					etcdSnapshotFileController: etcdSnapshotFileController,
				}
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
				cluster := newProvisioningClusterUnstructured("test-namespace", "test-cluster")
				etcdSnapshotFileController := fake.NewMockNonNamespacedControllerInterface[*k3s.ETCDSnapshotFile, *k3s.ETCDSnapshotFileList](ctrl)
				etcdSnapshotFileController.EXPECT().Get("test-snapshot-downstream", gomock.Any()).Return(nil, apierrors.NewNotFound(schema.GroupResource{}, "test-snapshot-downstream"))
				beaconCache := fake.NewMockCacheInterface[*planv1alpha1.Beacon](ctrl)
				beacon := &planv1alpha1.Beacon{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-namespace",
						Name:      "test-cluster",
					},
				}
				beaconCache.EXPECT().Get(cluster.GetNamespace(), cluster.GetName()).Return(beacon, nil).AnyTimes()
				etcdSnapshotController := fake.NewMockControllerInterface[*rkev1.ETCDSnapshot, *rkev1.ETCDSnapshotList](ctrl)
				etcdSnapshotController.EXPECT().Delete("test-namespace", "test-snapshot", gomock.Any()).Return(nil)
				return handler{
					beaconCache: beaconCache,
					dynamic:     &dynamicClientFake{obj: cluster},
					clusterRef: corev1.ObjectReference{
						APIVersion: "provisioning.cattle.io/v1",
						Kind:       "Cluster",
						Namespace:  "test-namespace",
						Name:       "test-cluster",
					},
					snapshotNamespace:          "test-namespace",
					etcdSnapshotFileController: etcdSnapshotFileController,
					etcdSnapshotController:     etcdSnapshotController,
				}
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
	etcdSnapshotCache := fake.NewMockCacheInterface[*rkev1.ETCDSnapshot](ctrl)
	etcdSnapshotController := fake.NewMockControllerInterface[*rkev1.ETCDSnapshot, *rkev1.ETCDSnapshotList](ctrl)
	beaconCache := fake.NewMockCacheInterface[*planv1alpha1.Beacon](ctrl)
	etcdSnapshotFileController := fake.NewMockNonNamespacedControllerInterface[*k3s.ETCDSnapshotFile, *k3s.ETCDSnapshotFileList](ctrl)
	nodeCache := fake.NewMockNonNamespacedCacheInterface[*corev1.Node](ctrl)

	cluster := newProvisioningClusterUnstructured("test-namespace", "test-cluster")
	deletedCluster := newProvisioningClusterUnstructured("test-namespace", "test-cluster")
	deletedCluster.SetDeletionTimestamp(&metav1.Time{Time: time.Now()})

	dynamicSuccess := &dynamicClientFake{obj: cluster}
	dynamicDeleted := &dynamicClientFake{obj: deletedCluster}
	dynamicError := &dynamicClientFake{err: errors.New("not found")}

	h := handler{
		clusterRef: corev1.ObjectReference{
			APIVersion: "provisioning.cattle.io/v1",
			Kind:       "Cluster",
			Namespace:  "test-namespace",
			Name:       "test-cluster",
		},
		snapshotNamespace:          "test-namespace",
		dynamic:                    dynamicSuccess,
		restMapper:                 testRESTMapper(),
		etcdSnapshotCache:          etcdSnapshotCache,
		etcdSnapshotController:     etcdSnapshotController,
		beaconCache:                beaconCache,
		nodeCache:                  nodeCache,
		etcdSnapshotFileController: etcdSnapshotFileController,
	}

	upstreamSnapshot := &rkev1.ETCDSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      "test-snapshot-upstream",
		},
	}

	var snapshot *k3s.ETCDSnapshotFile

	// Nil snapshot

	_, err := h.OnDownstreamChange("", snapshot)
	assert.NoError(t, err, "A nil snapshot should return immediately")

	// Provisioning cluster not found

	h.dynamic = dynamicError

	snapshot = &k3s.ETCDSnapshotFile{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-snapshot-downstream",
		},
		Status: k3s.ETCDSnapshotStatus{
			CreationTime: &metav1.Time{Time: time.Now()},
			ReadyToUse:   ptr.To(true),
		},
	}

	beacon := &planv1alpha1.Beacon{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      "test-cluster",
		},
	}

	_, err = h.OnDownstreamChange("", snapshot)
	assert.Error(t, err, "An error is expected if the cluster cannot be found")

	// Cluster being deleted

	h.dynamic = dynamicDeleted

	_, err = h.OnDownstreamChange("", snapshot)
	assert.NoError(t, err, "It should return early if the cluster is being deleted")

	// Downstream snapshot deleted, no matching upstream snapshots

	h.dynamic = dynamicSuccess
	snapshot.DeletionTimestamp = &metav1.Time{Time: time.Now()}

	etcdSnapshotCache.EXPECT().GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/test-snapshot-downstream").Return([]*rkev1.ETCDSnapshot{}, nil).Times(1)

	_, err = h.OnDownstreamChange("", snapshot)
	assert.NoError(t, err, "It should not return an error if the snapshot is being deleted and there are no upstream snapshots")

	// Downstream snapshot deleted, one matching upstream snapshot

	etcdSnapshotCache.EXPECT().GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/test-snapshot-downstream").Return([]*rkev1.ETCDSnapshot{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test-namespace",
				Name:      "test-snapshot-upstream",
			},
		},
	}, nil).Times(1)
	etcdSnapshotController.EXPECT().Delete(cluster.GetNamespace(), "test-snapshot-upstream", gomock.Any()).Return(nil).Times(1)

	_, err = h.OnDownstreamChange("", snapshot)
	assert.NoError(t, err, "It should not return an error if the snapshot is being deleted and there is 1 upstream snapshot")

	// Downstream snapshot deleted, multiple matching upstream snapshots

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
	etcdSnapshotController.EXPECT().Delete(cluster.GetNamespace(), "test-snapshot-upstream-0", gomock.Any()).Return(nil).Times(1)
	etcdSnapshotController.EXPECT().Delete(cluster.GetNamespace(), "test-snapshot-upstream-1", gomock.Any()).Return(nil).Times(1)

	_, err = h.OnDownstreamChange("", snapshot)
	assert.NoError(t, err, "It should not return an error if the snapshot is being deleted and there are 2 upstream snapshots")

	// Error getting snapshots from index

	snapshot.DeletionTimestamp = nil

	etcdSnapshotCache.EXPECT().GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/test-snapshot-downstream").Return(nil, errors.New("error")).Times(1)
	beaconCache.EXPECT().Get(cluster.GetNamespace(), cluster.GetName()).Return(beacon, nil).AnyTimes()

	_, err = h.OnDownstreamChange("", snapshot)
	assert.Error(t, err, "It should return an error if getting snapshots from the index fails")

	// No upstream snapshots, create new - node has lifecycle labels

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Labels: map[string]string{
				planv1alpha1.MachineLifecycleGroupLabel: "cluster.x-k8s.io",
				planv1alpha1.MachineLifecycleKindLabel:  "Machine",
				planv1alpha1.MachineLifecycleNameLabel:  "test-machine",
			},
		},
	}

	snapshot.Spec.NodeName = "test-node"
	etcdSnapshotCache.EXPECT().GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/test-snapshot-downstream").Return([]*rkev1.ETCDSnapshot{}, nil).Times(1)
	nodeCache.EXPECT().Get("test-node").Return(node, nil).Times(1)
	etcdSnapshotController.EXPECT().Create(gomock.Any()).Return(nil, nil).Times(1)

	_, err = h.OnDownstreamChange("", snapshot)
	assert.NoError(t, err, "It should not return an error when creating the snapshot")

	// No matching snapshots from index but already exists upstream (AlreadyExists)

	etcdSnapshotCache.EXPECT().GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/test-snapshot-downstream").Return([]*rkev1.ETCDSnapshot{}, nil).Times(1)
	nodeCache.EXPECT().Get("test-node").Return(node, nil).Times(1)

	etcdSnapshotController.EXPECT().Create(gomock.Any()).Return(nil, apierrors.NewAlreadyExists(schema.GroupResource{}, "error")).Times(1)

	existingSnapshot := upstreamSnapshot.DeepCopy()
	existingSnapshot.ResourceVersion = "1"

	etcdSnapshotCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(existingSnapshot, nil).Times(1)
	nodeCache.EXPECT().Get("test-node").Return(node, nil).Times(1)
	etcdSnapshotController.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *rkev1.ETCDSnapshot) (*rkev1.ETCDSnapshot, error) {
		assert.Equal(t, "1", s.ResourceVersion)
		s.ResourceVersion = "2"
		return s, nil
	})

	_, err = h.OnDownstreamChange("", snapshot)
	assert.NoError(t, err, "It should update the snapshot if one exists but could not be found by the indexer")

	// No matching snapshots, create succeeds

	etcdSnapshotCache.EXPECT().GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/test-snapshot-downstream").Return([]*rkev1.ETCDSnapshot{}, nil).Times(1)
	nodeCache.EXPECT().Get("test-node").Return(node, nil).Times(1)

	etcdSnapshotController.EXPECT().Create(gomock.Any()).DoAndReturn(func(s *rkev1.ETCDSnapshot) (*rkev1.ETCDSnapshot, error) {
		assert.Equal(t, "", s.ResourceVersion)
		s.ResourceVersion = "1"
		return s, nil
	})

	_, err = h.OnDownstreamChange("", snapshot)
	assert.NoError(t, err, "It should create the snapshot if one does not exist")

	// Delete multiple duplicate upstream snapshots and re-enqueue

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

	// Update existing snapshot

	etcdSnapshotCache.EXPECT().GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/test-snapshot-downstream").Return([]*rkev1.ETCDSnapshot{upstreamSnapshot}, nil).AnyTimes()
	nodeCache.EXPECT().Get("test-node").Return(node, nil).AnyTimes()

	etcdSnapshotController.EXPECT().Patch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(upstreamSnapshot, nil).Times(1)

	_, err = h.OnDownstreamChange("", snapshot)
	assert.NoError(t, err, "It should not return an error when updating the snapshot")
}

func TestOnDownstreamChange_RestoreModeAnnotationIsSetCorrectly(t *testing.T) {
	compressSpec := func(t *testing.T, spec *provv1.ClusterSpec) string {
		payload, err := snapshotutil.CompressInterface(spec)
		require.NoError(t, err)
		return payload
	}

	validSpecFull := &provv1.ClusterSpec{
		KubernetesVersion: "v1.34.1+rke2r1",
		RKEConfig:         &provv1.RKEConfig{},
	}
	validSpecNoRKEConfig := &provv1.ClusterSpec{
		KubernetesVersion: "v1.34.1+rke2r1",
	}
	validSpecNoK8sVersion := &provv1.ClusterSpec{
		RKEConfig: &provv1.RKEConfig{},
	}

	testCases := []struct {
		name               string
		metadata           map[string]string
		expectedAnnotation string
	}{
		{
			name:               "metadata is nil",
			metadata:           nil,
			expectedAnnotation: "none",
		},
		{
			name:               "metadata is empty map",
			metadata:           map[string]string{},
			expectedAnnotation: "none",
		},
		{
			name: "metadata key is present but payload is corrupt",
			metadata: map[string]string{
				rkev1.SnapshotMetadataClusterSpecKey: "not-base64-or-gzip-corrupt-data",
			},
			expectedAnnotation: "none",
		},
		{
			name: "spec is valid but missing k8s version",
			metadata: map[string]string{
				rkev1.SnapshotMetadataClusterSpecKey: compressSpec(t, validSpecNoK8sVersion),
			},
			expectedAnnotation: "none",
		},
		{
			name: "spec has k8s version but no RKEConfig",
			metadata: map[string]string{
				rkev1.SnapshotMetadataClusterSpecKey: compressSpec(t, validSpecNoRKEConfig),
			},
			expectedAnnotation: "none,kubernetesVersion",
		},
		{
			name: "spec has k8s version and RKEConfig",
			metadata: map[string]string{
				rkev1.SnapshotMetadataClusterSpecKey: compressSpec(t, validSpecFull),
			},
			expectedAnnotation: "none,kubernetesVersion,all",
		},
	}

	ctrl := gomock.NewController(t)

	etcdSnapshotCache := fake.NewMockCacheInterface[*rkev1.ETCDSnapshot](ctrl)
	etcdSnapshotController := fake.NewMockControllerInterface[*rkev1.ETCDSnapshot, *rkev1.ETCDSnapshotList](ctrl)
	beaconCache := fake.NewMockCacheInterface[*planv1alpha1.Beacon](ctrl)
	etcdSnapshotFileController := fake.NewMockNonNamespacedControllerInterface[*k3s.ETCDSnapshotFile, *k3s.ETCDSnapshotFileList](ctrl)
	nodeCache := fake.NewMockNonNamespacedCacheInterface[*corev1.Node](ctrl)

	cluster := newProvisioningClusterUnstructured("fleet-default", "example")

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cp-0",
			Labels: map[string]string{
				planv1alpha1.MachineLifecycleGroupLabel: "cluster.x-k8s.io",
				planv1alpha1.MachineLifecycleKindLabel:  "Machine",
				planv1alpha1.MachineLifecycleNameLabel:  "machine-0",
			},
		},
	}

	handlerUnderTest := handler{
		clusterRef: corev1.ObjectReference{
			APIVersion: "provisioning.cattle.io/v1",
			Kind:       "Cluster",
			Namespace:  "fleet-default",
			Name:       "example",
		},
		snapshotNamespace:          "fleet-default",
		dynamic:                    &dynamicClientFake{obj: cluster},
		restMapper:                 testRESTMapper(),
		etcdSnapshotCache:          etcdSnapshotCache,
		etcdSnapshotController:     etcdSnapshotController,
		beaconCache:                beaconCache,
		nodeCache:                  nodeCache,
		etcdSnapshotFileController: etcdSnapshotFileController,
	}

	beacon := &planv1alpha1.Beacon{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fleet-default",
			Name:      "example",
		},
	}

	beaconCache.EXPECT().Get(cluster.GetNamespace(), cluster.GetName()).Return(beacon, nil).AnyTimes()

	makeDownstream := func(isS3Storage bool, metadata map[string]string, name string) *k3s.ETCDSnapshotFile {
		ds := &k3s.ETCDSnapshotFile{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec: k3s.ETCDSnapshotSpec{
				SnapshotName: "etcdsnapshot-name",
				NodeName:     "cp-0",
				Location:     "file:///var/lib/rancher/etcd",
				Metadata:     metadata,
			},
			Status: k3s.ETCDSnapshotStatus{
				CreationTime: &metav1.Time{Time: time.Now()},
				ReadyToUse:   ptr.To(true),
			},
		}
		if isS3Storage {
			ds.Spec.S3 = &k3s.ETCDSnapshotS3{
				Bucket:   "bucket-name",
				Region:   "us-east-1",
				Prefix:   "etcd-snaps",
				Endpoint: "s3.amazonaws.com",
			}
			ds.Spec.Location = "s3://bucket-name/etcd-snaps/etcdsnapshot-name"
		}
		return ds
	}

	storageTypes := []struct {
		name        string
		isS3Storage bool
	}{
		{name: "Local Storage", isS3Storage: false},
		{name: "S3 Storage", isS3Storage: true},
	}

	for _, storage := range storageTypes {
		for _, tc := range testCases {

			downstreamSnapshotName := strings.ReplaceAll(tc.name, " ", "-") + "-" + strings.ToLower(storage.name)

			t.Run(storage.name+": "+tc.name, func(t *testing.T) {
				downstreamFile := makeDownstream(storage.isS3Storage, tc.metadata, downstreamSnapshotName)

				etcdSnapshotCache.EXPECT().
					GetByIndex(cluster2.ByETCDSnapshotName, "fleet-default/example/"+downstreamSnapshotName).
					Return([]*rkev1.ETCDSnapshot{}, nil).
					Times(1)

				if !storage.isS3Storage {
					nodeCache.EXPECT().Get("cp-0").Return(node, nil).Times(1)
				}

				etcdSnapshotController.EXPECT().
					Create(gomock.Any()).
					DoAndReturn(func(created *rkev1.ETCDSnapshot) (*rkev1.ETCDSnapshot, error) {
						annotations := created.GetAnnotations()
						require.NotNil(t, annotations)
						assert.Equal(t, tc.expectedAnnotation, annotations[RestoreModeOptionsAnnotation], "Annotation should be set correctly")

						assert.Equal(t, "successful", created.SnapshotFile.Status, "Status should be successful because ReadyToUse is true")

						require.Equal(t, downstreamFile.Spec.SnapshotName, annotations[SnapshotFileNameAnnotationKey])
						return created, nil
					}).Times(1)

				_, err := handlerUnderTest.OnDownstreamChange("", downstreamFile)
				require.NoError(t, err)
			})
		}
	}
}

func TestGetCluster(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string

		clusterRef corev1.ObjectReference
		dynamicObj runtime.Object
		dynamicErr error

		expectNamespace string
		expectName      string
		expectErr       bool
	}{
		{
			name: "dynamic returns error",
			clusterRef: corev1.ObjectReference{
				APIVersion: "provisioning.cattle.io/v1",
				Kind:       "Cluster",
				Namespace:  "test-namespace",
				Name:       "test-cluster",
			},
			dynamicErr: errors.New("not found"),
			expectErr:  true,
		},
		{
			name: "success",
			clusterRef: corev1.ObjectReference{
				APIVersion: "provisioning.cattle.io/v1",
				Kind:       "Cluster",
				Namespace:  "test-namespace",
				Name:       "test-cluster",
			},
			dynamicObj:      newProvisioningClusterUnstructured("test-namespace", "test-cluster"),
			expectNamespace: "test-namespace",
			expectName:      "test-cluster",
			expectErr:       false,
		},
		{
			name: "cluster-scoped resource",
			clusterRef: corev1.ObjectReference{
				APIVersion: "management.cattle.io/v3",
				Kind:       "Cluster",
				Name:       "c-m-12345",
			},
			dynamicObj:      newUnstructuredCluster("management.cattle.io/v3", "Cluster", "", "c-m-12345"),
			expectNamespace: "",
			expectName:      "c-m-12345",
			expectErr:       false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			h := handler{
				clusterRef: tt.clusterRef,
				dynamic:    &dynamicClientFake{obj: tt.dynamicObj, err: tt.dynamicErr},
			}
			cluster, err := h.getCluster()
			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, cluster)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, cluster)
				assert.Equal(t, tt.expectNamespace, cluster.GetNamespace())
				assert.Equal(t, tt.expectName, cluster.GetName())
			}
		})
	}
}

func TestGetSnapshotsFromSnapshotFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string

		cluster      *unstructured.Unstructured
		snapshotFile *k3s.ETCDSnapshotFile

		cacheFunc func(cache *fake.MockCacheInterface[*rkev1.ETCDSnapshot])

		expectedSnapshots []*rkev1.ETCDSnapshot
		expectErr         bool
	}{
		{
			name:         "nil result",
			snapshotFile: k3s.NewETCDSnapshotFile("", "test-snapshot", k3s.ETCDSnapshotFile{}),
			cluster:      newProvisioningClusterUnstructured("test-namespace", "test-cluster"),
			cacheFunc: func(cache *fake.MockCacheInterface[*rkev1.ETCDSnapshot]) {
				cache.EXPECT().GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/test-snapshot").Return(nil, nil)
			},
			expectedSnapshots: nil,
			expectErr:         false,
		},
		{
			name:         "empty result",
			snapshotFile: k3s.NewETCDSnapshotFile("", "test-snapshot", k3s.ETCDSnapshotFile{}),
			cluster:      newProvisioningClusterUnstructured("test-namespace", "test-cluster"),
			cacheFunc: func(cache *fake.MockCacheInterface[*rkev1.ETCDSnapshot]) {
				cache.EXPECT().GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/test-snapshot").Return([]*rkev1.ETCDSnapshot{}, nil)
			},
			expectedSnapshots: []*rkev1.ETCDSnapshot{},
			expectErr:         false,
		},
		{
			name:         "error from index",
			snapshotFile: k3s.NewETCDSnapshotFile("", "test-snapshot", k3s.ETCDSnapshotFile{}),
			cluster:      newProvisioningClusterUnstructured("test-namespace", "test-cluster"),
			cacheFunc: func(cache *fake.MockCacheInterface[*rkev1.ETCDSnapshot]) {
				cache.EXPECT().GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/test-snapshot").Return(nil, errors.New("error from index"))
			},
			expectedSnapshots: []*rkev1.ETCDSnapshot{},
			expectErr:         true,
		},
		{
			name:         "match from index",
			snapshotFile: k3s.NewETCDSnapshotFile("", "test-snapshot", k3s.ETCDSnapshotFile{}),
			cluster:      newProvisioningClusterUnstructured("test-namespace", "test-cluster"),
			cacheFunc: func(cache *fake.MockCacheInterface[*rkev1.ETCDSnapshot]) {
				cache.EXPECT().GetByIndex(cluster2.ByETCDSnapshotName, "test-namespace/test-cluster/test-snapshot").Return([]*rkev1.ETCDSnapshot{{}}, nil)
			},
			expectedSnapshots: []*rkev1.ETCDSnapshot{{}},
			expectErr:         false,
		},
		{
			name:         "multiple matches from index",
			snapshotFile: k3s.NewETCDSnapshotFile("", "test-snapshot", k3s.ETCDSnapshotFile{}),
			cluster:      newProvisioningClusterUnstructured("test-namespace", "test-cluster"),
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
			// snapshotNamespace mirrors the namespace snapshots are written to. All fixtures use
			// "test-namespace" for the cluster's own ns, and for the non-CAPRKE2 paths in this
			// test file that ns equals h.snapshotNamespace.
			h := handler{snapshotNamespace: tt.cluster.GetNamespace()}
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

func TestGetLogPrefix(t *testing.T) {
	cluster := newProvisioningClusterUnstructured("test-namespace", "test-cluster")
	expected := fmt.Sprintf("[snapshotbackpopulate] %s/test-namespace/test-cluster:",
		schema.FromAPIVersionAndKind("provisioning.cattle.io/v1", "Cluster").String())
	assert.Equal(t, expected, getLogPrefix(cluster))
}

func TestMachineLifecycleLabelsToObjectReference(t *testing.T) {
	t.Parallel()

	allLabels := map[string]string{
		planv1alpha1.MachineLifecycleGroupLabel: "cluster.x-k8s.io",
		planv1alpha1.MachineLifecycleKindLabel:  "Machine",
		planv1alpha1.MachineLifecycleNameLabel:  "test-machine",
	}

	drop := func(key string) map[string]string {
		l := make(map[string]string, len(allLabels))
		for k, v := range allLabels {
			l[k] = v
		}
		delete(l, key)
		return l
	}

	tests := []struct {
		name             string
		labels           map[string]string
		contextNamespace string
		expectRef        *corev1.ObjectReference
		expectErr        bool
	}{
		{
			name:      "no labels",
			labels:    nil,
			expectErr: true,
		},
		{
			name:      "missing kind",
			labels:    drop(planv1alpha1.MachineLifecycleKindLabel),
			expectErr: true,
		},
		{
			name:      "missing name",
			labels:    drop(planv1alpha1.MachineLifecycleNameLabel),
			expectErr: true,
		},
		{
			name: "unknown group",
			labels: map[string]string{
				planv1alpha1.MachineLifecycleGroupLabel: "not.a.real.group",
				planv1alpha1.MachineLifecycleKindLabel:  "Machine",
				planv1alpha1.MachineLifecycleNameLabel:  "test-machine",
			},
			contextNamespace: "fleet-default",
			expectErr:        true,
		},
		{
			name:             "namespace comes from context, not label",
			labels:           allLabels,
			contextNamespace: "fleet-default",
			expectRef: &corev1.ObjectReference{
				APIVersion: "cluster.x-k8s.io/v1beta2",
				Kind:       "Machine",
				Name:       "test-machine",
				Namespace:  "fleet-default",
			},
		},
		{
			name: "stale namespace label is ignored (spoofing check)",
			labels: map[string]string{
				planv1alpha1.MachineLifecycleGroupLabel:                   "cluster.x-k8s.io",
				planv1alpha1.MachineLifecycleKindLabel:                    "Machine",
				planv1alpha1.MachineLifecycleNameLabel:                    "test-machine",
				"plan.cattle.io/machine-namespace-legacy-ignored-by-code": "other-tenant-ns",
			},
			contextNamespace: "fleet-default",
			expectRef: &corev1.ObjectReference{
				APIVersion: "cluster.x-k8s.io/v1beta2",
				Kind:       "Machine",
				Name:       "test-machine",
				Namespace:  "fleet-default",
			},
		},
	}

	mapper := testRESTMapper()
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			obj := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-node",
					Labels: tt.labels,
				},
			}
			ref, err := planv1alpha1.MachineLifecycleLabelsToObjectReference(obj, tt.contextNamespace, mapper)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, ref)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectRef, ref)
			}
		})
	}
}

func TestGenerateSafeSnapshotName(t *testing.T) {
	callTime := time.Unix(1_700_000_000, 0)

	newSpec := func(name, node, loc string, s3 bool) k3s.ETCDSnapshotSpec {
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

	hex6re := regexp.MustCompile(`^[a-f0-9]{6}$`)

	tests := []struct {
		name       string
		storage    Storage
		spec       k3s.ETCDSnapshotSpec
		wantPrefix string
	}{
		{
			name:       "Local valid filename is reused as base",
			spec:       newSpec("valid-filename", "cp-0", "file:///var/lib/etcd", false),
			wantPrefix: "local-valid-filename-",
		},
		{
			name:       "S3 valid filename is reused as base",
			spec:       newSpec("ok.valid.name", "cp-0", "s3://bucket/prefix/key", true),
			wantPrefix: "s3-ok.valid.name-",
		},
		{
			name:       "Local invalid filename falls back to host+unix",
			spec:       newSpec("something.something.-.s3-.com", "cp-0.example.com", "file:///var/lib", false),
			wantPrefix: "local-etcd-snapshot-cp-0-1700000000-",
		},
		{
			name:       "S3 overly long filename falls back to host+unix",
			spec:       newSpec(strings.Repeat("a", 250), "cp-0", "s3://bucket/prefix", true),
			wantPrefix: "s3-etcd-snapshot-cp-0-1700000000-",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := generateSafeSnapshotName(tc.spec, callTime)
			require.Equal(t, strings.ToLower(got), got)
			require.True(t, strings.HasPrefix(got, tc.wantPrefix), got)

			parts := strings.Split(got, "-")
			require.GreaterOrEqual(t, len(parts), 2)
			suffix := parts[len(parts)-1]
			require.Regexp(t, hex6re, suffix)

			got2 := generateSafeSnapshotName(tc.spec, callTime)
			require.Equal(t, got, got2)
		})
	}

	t.Run("Digest changes when location changes", func(t *testing.T) {
		a := newSpec("ok", "cp-0", "s3://bucket/prefix/A", true)
		b := newSpec("ok", "cp-0", "s3://bucket/prefix/B", true)

		ga := generateSafeSnapshotName(a, callTime)
		gb := generateSafeSnapshotName(b, callTime)

		require.True(t, strings.HasPrefix(ga, "s3-ok-"))
		require.True(t, strings.HasPrefix(gb, "s3-ok-"))
		require.NotEqual(t, ga, gb, "digest suffix must differ when location differs")
	})
}
