package prebootstrap

import (
	"encoding/base64"
	"path"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/capr/planner"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGeneratePreBootstrapClusterAgentManifestWithoutSyncBootstrapSecrets(t *testing.T) {
	originalServerURL := settings.ServerURL.Get()
	t.Cleanup(func() {
		_ = settings.ServerURL.Set(originalServerURL)
	})
	assert.NoError(t, settings.ServerURL.Set("https://rancher.example.com"))

	ctrl := gomock.NewController(t)
	mgmtClusterCache := fake.NewMockNonNamespacedCacheInterface[*apimgmtv3.Cluster](ctrl)
	clusterRegistrationTokenCache := fake.NewMockCacheInterface[*apimgmtv3.ClusterRegistrationToken](ctrl)
	secretCache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)

	controlPlane := &rkev1.RKEControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-control-plane",
			Namespace: "fleet-default",
		},
		Spec: rkev1.RKEControlPlaneSpec{
			KubernetesVersion:     "v1.28.8+rke2r1",
			ManagementClusterName: "c-m-test",
		},
	}
	mgmtCluster := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "c-m-test",
		},
		Spec: apimgmtv3.ClusterSpec{
			DisplayName:        "test-cluster",
			FleetWorkspaceName: "fleet-default",
		},
	}
	token := &apimgmtv3.ClusterRegistrationToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-token",
			Namespace: "fleet-default",
		},
		Status: apimgmtv3.ClusterRegistrationTokenStatus{
			TokenSecretName: "default-token-secret",
		},
	}

	clusterRegistrationTokenCache.EXPECT().GetByIndex(planner.ClusterRegToken, "c-m-test").Return([]*apimgmtv3.ClusterRegistrationToken{token}, nil)
	mgmtClusterCache.EXPECT().Get("c-m-test").Return(mgmtCluster, nil).Times(2)
	secretCache.EXPECT().Get("fleet-default", "default-token-secret").Return(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-token-secret",
			Namespace: "fleet-default",
		},
		Data: map[string][]byte{
			"token": []byte("test-token"),
		},
	}, nil)

	retriever := &Retriever{
		mgmtClusterCache:              mgmtClusterCache,
		clusterRegistrationTokenCache: clusterRegistrationTokenCache,
		secretCache:                   secretCache,
	}

	files, err := retriever.GeneratePreBootstrapClusterAgentManifest(controlPlane)

	assert.NoError(t, err)
	if assert.Len(t, files, 1) {
		assert.Equal(t, path.Join(capr.GetDistroDataDir(controlPlane), "server/manifests/rancher/cluster-agent.yaml"), files[0].Path)
		assert.True(t, files[0].Dynamic)
		assert.True(t, files[0].Minor)

		content, err := base64.StdEncoding.DecodeString(files[0].Content)
		assert.NoError(t, err)
		assert.Contains(t, string(content), "name: cattle-cluster-agent-bootstrap")
		assert.Contains(t, string(content), "name: CATTLE_PREBOOTSTRAP")
		assert.Contains(t, string(content), "hostNetwork: true")
	}
}
