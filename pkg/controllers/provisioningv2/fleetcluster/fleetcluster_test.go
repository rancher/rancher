package fleetcluster

import (
	"fmt"
	"testing"

	mgmt "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	errNotImplemented = fmt.Errorf("unimplemented")
	errNotFound       = fmt.Errorf("not found")
)

func TestCreateCluster(t *testing.T) {
	h := &handler{
		clustersCache:     &fakeClusterCache{},
		getPrivateRepoURL: func(*v1.Cluster, *mgmt.Cluster) string { return "" },
	}

	tests := []struct {
		name          string
		cluster       *v1.Cluster
		status        v1.ClusterStatus
		clustersCache mgmtv3.ClusterCache
		wantLen       int
	}{
		{
			"cluster-has-no-cg",
			&v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: v1.ClusterSpec{},
			},
			v1.ClusterStatus{
				ClusterName:      "cluster-name",
				ClientSecretName: "client-secret-name",
			},

			newClusterCache(map[string]*mgmt.Cluster{
				"cluster-name": newMgmtCluster(
					"cluster-name",
					map[string]string{
						"cluster-group": "cluster-group-name",
					},
					false,
				),
			}),
			1,
		},
		{
			"local-cluster-has-cg-has-label",
			&v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "local-cluster",
					Namespace: "fleet-local",
				},
				Spec: v1.ClusterSpec{},
			},
			v1.ClusterStatus{
				ClusterName:      "local-cluster",
				ClientSecretName: "local-kubeconfig",
			},
			newClusterCache(map[string]*mgmt.Cluster{
				"local-cluster": newMgmtCluster(
					"local-cluster",
					map[string]string{
						"cluster-group": "default",
					},
					true,
				),
			}),
			2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h.clustersCache = tt.clustersCache
			objs, _, err := h.createCluster(tt.cluster, tt.status)

			if objs == nil {
				t.Errorf("Expected non-nil objs: %v", err)
			}

			if err != nil {
				t.Errorf("Expected nil err")
			}

			if len(objs) != tt.wantLen {
				t.Errorf("Expected %d objects, got %d", tt.wantLen, len(objs))
			}
		})
	}

}

func newMgmtCluster(name string, labels map[string]string, internal bool) *mgmt.Cluster {
	mgmtCluster := &mgmt.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: mgmt.ClusterSpec{
			DisplayName: name,
			Internal:    internal,
		},
	}
	mgmt.ClusterConditionReady.SetStatus(mgmtCluster, "True")
	return mgmtCluster

}

// implements v3.ClusterCache
func newClusterCache(clusters map[string]*mgmt.Cluster) mgmtv3.ClusterCache {
	return &fakeClusterCache{
		clusters: clusters,
	}
}

type fakeClusterCache struct {
	clusters map[string]*mgmt.Cluster
}

func (f *fakeClusterCache) Get(name string) (*mgmt.Cluster, error) {
	if c, ok := f.clusters[name]; ok {
		return c, nil
	}
	return nil, errNotFound
}
func (f *fakeClusterCache) List(selector labels.Selector) ([]*mgmt.Cluster, error) {
	return nil, errNotImplemented
}
func (f *fakeClusterCache) AddIndexer(indexName string, indexer mgmtv3.ClusterIndexer) {}
func (f *fakeClusterCache) GetByIndex(indexName, key string) ([]*mgmt.Cluster, error) {
	return nil, errNotImplemented
}
