package cluster

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/generated/mocks"
	"github.com/rancher/wrangler/pkg/generic"
)

func TestRegexp(t *testing.T) {
	assert.True(t, mgmtNameRegexp.MatchString("local"))
	assert.False(t, mgmtNameRegexp.MatchString("alocal"))
	assert.False(t, mgmtNameRegexp.MatchString("localb"))
	assert.True(t, mgmtNameRegexp.MatchString("c-12345"))
	assert.False(t, mgmtNameRegexp.MatchString("ac-12345"))
	assert.False(t, mgmtNameRegexp.MatchString("c-12345b"))
	assert.False(t, mgmtNameRegexp.MatchString("ac-12345b"))
}

func TestOnClusterRemove_CAPI_WithOwned(t *testing.T) {
	name := "test"
	namespace := "default"
	rancherCluster := createRancherCluster(name, namespace)
	capiCluster := createCAPICluster(name, namespace, rancherCluster)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	clusterControllerMock := mocks.NewMockClusterController(mockCtrl)
	capiClusterCache := mocks.NewMockClusterCache(mockCtrl)
	capiClusterClient := mocks.NewMockClusterClient(mockCtrl)
	rkeControlPlaneCache := mocks.NewMockRKEControlPlaneCache(mockCtrl)

	capiClusterCache.EXPECT().Get(rancherCluster.Namespace, rancherCluster.Name).Return(capiCluster, nil)
	clusterControllerMock.EXPECT().UpdateStatus(gomock.AssignableToTypeOf(rancherCluster)).Return(rancherCluster, nil)
	capiClusterClient.EXPECT().Delete(rancherCluster.Namespace, rancherCluster.Name, &metav1.DeleteOptions{}).Return(nil)
	rkeControlPlaneCache.EXPECT().Get(rancherCluster.Namespace, rancherCluster.Name).Return(nil, nil)

	h := &handler{
		clusters:              clusterControllerMock,
		capiClustersCache:     capiClusterCache,
		capiClusters:          capiClusterClient,
		rkeControlPlanesCache: rkeControlPlaneCache,
	}
	_, err := h.OnClusterRemove("", rancherCluster)
	assert.Equal(t, generic.ErrSkip, err)
}

func TestOnClusterRemove_CAPI_NotOwned(t *testing.T) {
	name := "test"
	namespace := "default"

	rancherCluster := createRancherCluster(name, namespace)
	capiCluster := createCAPICluster(name, namespace, nil)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	clusterControllerMock := mocks.NewMockClusterController(mockCtrl)
	capiClusterCache := mocks.NewMockClusterCache(mockCtrl)

	capiClusterCache.EXPECT().Get(rancherCluster.Namespace, rancherCluster.Name).Return(capiCluster, nil)
	clusterControllerMock.EXPECT().UpdateStatus(gomock.AssignableToTypeOf(rancherCluster)).Return(rancherCluster, nil)

	h := &handler{
		clusters:          clusterControllerMock,
		capiClustersCache: capiClusterCache,
	}
	_, err := h.OnClusterRemove("", rancherCluster)
	assert.Nil(t, err)
}

func createRancherCluster(name, namespace string) *provv1.Cluster {
	return &provv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec:   provv1.ClusterSpec{},
		Status: provv1.ClusterStatus{},
	}
}

func createCAPICluster(name, namespace string, ownedBy *provv1.Cluster) *capi.Cluster {
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec:   capi.ClusterSpec{},
		Status: capi.ClusterStatus{},
	}

	if ownedBy != nil {
		cluster.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: provv1.SchemeGroupVersion.Identifier(),
				Kind:       "Cluster",
				Name:       ownedBy.Name,
			},
		}
	}

	return cluster
}
