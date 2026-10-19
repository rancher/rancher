package prebootstrap

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/capr/planner"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// newPreBootstrapTestRetriever returns a retriever for a cluster still to be pre-bootstrapped, whose registration
// tokens are tokens, each stored in a secret holding the token's name followed by "-value".
func newPreBootstrapTestRetriever(t *testing.T, mgmtCluster *apimgmtv3.Cluster, tokens ...string) *Retriever {
	t.Helper()
	ctrl := gomock.NewController(t)
	mgmtClusterCache := fake.NewMockNonNamespacedCacheInterface[*apimgmtv3.Cluster](ctrl)
	clusterRegistrationTokenCache := fake.NewMockCacheInterface[*apimgmtv3.ClusterRegistrationToken](ctrl)
	secretCache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)

	var crts []*apimgmtv3.ClusterRegistrationToken
	for _, name := range tokens {
		crts = append(crts, &apimgmtv3.ClusterRegistrationToken{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: mgmtCluster.Name},
			Status:     apimgmtv3.ClusterRegistrationTokenStatus{TokenSecretName: name + "-secret"},
		})
	}
	clusterRegistrationTokenCache.EXPECT().GetByIndex(planner.ClusterRegToken, mgmtCluster.Name).Return(crts, nil)
	mgmtClusterCache.EXPECT().Get(mgmtCluster.Name).Return(mgmtCluster, nil).AnyTimes()
	secretCache.EXPECT().Get(mgmtCluster.Name, gomock.Any()).DoAndReturn(func(_, name string) (*corev1.Secret, error) {
		return &corev1.Secret{Data: map[string][]byte{"token": []byte(strings.TrimSuffix(name, "-secret") + "-value")}}, nil
	}).AnyTimes()

	return &Retriever{
		mgmtClusterCache:              mgmtClusterCache,
		clusterRegistrationTokenCache: clusterRegistrationTokenCache,
		secretCache:                   secretCache,
	}
}

func newPreBootstrapTestCluster() *apimgmtv3.Cluster {
	return &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c-m-test"},
		Spec:       apimgmtv3.ClusterSpec{DisplayName: "test-cluster", FleetWorkspaceName: "fleet-default"},
	}
}

// credentialsSecretName returns the name of the credentials Secret the agent of the cluster named clusterName is
// rendered with for token.
func credentialsSecretName(clusterName, token string) string {
	d := sha256.Sum256([]byte(fmt.Sprintf("%s.%s.%s", settings.ServerURL.Get(), token, clusterName)))
	return "cattle-credentials-" + hex.EncodeToString(d[:])[:10]
}

func TestGeneratePreBootstrapClusterAgentManifestSwitchesToTheSystemTokenOnceDeployed(t *testing.T) {
	// clusterDeploy redeploys the pre-bootstrap agent with the system token once it has connected with the
	// default-token. Rendering the default-token after that would roll the agent back.
	originalServerURL := settings.ServerURL.Get()
	t.Cleanup(func() { _ = settings.ServerURL.Set(originalServerURL) })
	assert.NoError(t, settings.ServerURL.Set("https://rancher.example.com"))
	controlPlane := &rkev1.RKEControlPlane{Spec: rkev1.RKEControlPlaneSpec{KubernetesVersion: "v1.28.8+rke2r1", ManagementClusterName: "c-m-test"}}

	for _, deployed := range []bool{false, true} {
		cluster := newPreBootstrapTestCluster()
		want, other := "default-token-value", "system-value"
		if deployed {
			apimgmtv3.ClusterConditionAgentDeployed.True(cluster)
			want, other = other, want
		}
		retriever := newPreBootstrapTestRetriever(t, cluster, "system", "default-token")

		files, err := retriever.GeneratePreBootstrapClusterAgentManifest(controlPlane)

		require.NoError(t, err)
		require.Len(t, files, 1)
		content, err := base64.StdEncoding.DecodeString(files[0].Content)
		require.NoError(t, err)
		assert.Contains(t, string(content), credentialsSecretName("c-m-test", want), "deployed=%v", deployed)
		assert.NotContains(t, string(content), credentialsSecretName("c-m-test", other), "deployed=%v", deployed)
	}
}

func TestGeneratePreBootstrapClusterAgentManifestWaitsForTheSystemToken(t *testing.T) {
	cluster := newPreBootstrapTestCluster()
	apimgmtv3.ClusterConditionAgentDeployed.True(cluster)
	retriever := newPreBootstrapTestRetriever(t, cluster, "default-token")
	controlPlane := &rkev1.RKEControlPlane{Spec: rkev1.RKEControlPlaneSpec{KubernetesVersion: "v1.28.8+rke2r1", ManagementClusterName: "c-m-test"}}

	_, err := retriever.GeneratePreBootstrapClusterAgentManifest(controlPlane)

	require.Error(t, err)
	assert.True(t, planner.IsErrWaiting(err), "it should wait rather than fall back to the default-token: %v", err)
}
