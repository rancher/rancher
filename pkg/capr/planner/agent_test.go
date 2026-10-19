package planner

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func newAgentTestRegistrationToken(name string) *apisv3.ClusterRegistrationToken {
	return &apisv3.ClusterRegistrationToken{
		ObjectMeta: metav1.ObjectMeta{Namespace: "c-m-test", Name: name},
		Status:     apisv3.ClusterRegistrationTokenStatus{TokenSecretName: "crt-" + name},
	}
}

func TestClusterAgentRegistrationToken(t *testing.T) {
	both := []*apisv3.ClusterRegistrationToken{
		newAgentTestRegistrationToken(defaultRegistrationTokenName),
		newAgentTestRegistrationToken(systemRegistrationTokenName),
	}
	tests := []struct {
		name      string
		condition func(*apisv3.Cluster)
		want      string
	}{
		{
			name:      "no agent has connected yet",
			condition: func(*apisv3.Cluster) {},
			want:      defaultRegistrationTokenName,
		},
		{
			name:      "an agent is connected",
			condition: func(c *apisv3.Cluster) { apisv3.ClusterConditionConnected.True(c) },
			want:      systemRegistrationTokenName,
		},
		{
			// Connected is held False while pre-bootstrapping, but pre-bootstrapping needs a connected agent.
			name:      "the cluster was pre-bootstrapped",
			condition: func(c *apisv3.Cluster) { apisv3.ClusterConditionPreBootstrapped.True(c) },
			want:      systemRegistrationTokenName,
		},
		{
			// The agent being replaced disconnects for a while, which must not switch back.
			name: "the agent was deployed and is disconnected",
			condition: func(c *apisv3.Cluster) {
				apisv3.ClusterConditionAgentDeployed.True(c)
				apisv3.ClusterConditionConnected.False(c)
			},
			want: systemRegistrationTokenName,
		},
		{
			name:      "only being pre-bootstrapped is not connected",
			condition: func(c *apisv3.Cluster) { apisv3.ClusterConditionPreBootstrapped.Unknown(c) },
			want:      defaultRegistrationTokenName,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := &apisv3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test"}}
			tt.condition(cluster)

			crt, err := ClusterAgentRegistrationToken(cluster, both)

			require.NoError(t, err)
			assert.Equal(t, tt.want, crt.Name)
		})
	}
}

func TestClusterAgentRegistrationTokenWaitsForTheSystemToken(t *testing.T) {
	// Falling back to the default-token would render an agent clusterDeploy is about to replace.
	cluster := &apisv3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test"}}
	apisv3.ClusterConditionPreBootstrapped.True(cluster)

	_, err := ClusterAgentRegistrationToken(cluster, []*apisv3.ClusterRegistrationToken{newAgentTestRegistrationToken(defaultRegistrationTokenName)})

	require.Error(t, err)
	assert.True(t, IsErrWaiting(err), "it should wait rather than fail: %v", err)
}

func TestClusterAgentRegistrationTokenFailsWithoutTheDefaultToken(t *testing.T) {
	cluster := &apisv3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test"}}

	_, err := ClusterAgentRegistrationToken(cluster, []*apisv3.ClusterRegistrationToken{newAgentTestRegistrationToken("other")})
	require.Error(t, err)
	assert.False(t, IsErrWaiting(err))

	_, err = ClusterAgentRegistrationToken(cluster, nil)
	require.Error(t, err)
	assert.False(t, IsErrWaiting(err))
}

// agentCredentialsSecretName returns the name of the credentials Secret systemtemplate renders the agent of the
// cluster named clusterName with, for token.
func agentCredentialsSecretName(t *testing.T, clusterName, token string) string {
	t.Helper()
	d := sha256.Sum256([]byte(fmt.Sprintf("%s.%s.%s", settings.ServerURL.Get(), token, clusterName)))
	return "cattle-credentials-" + hex.EncodeToString(d[:])[:10]
}

func TestGenerateClusterAgentManifestUsesTheTokenOfClusterDeploy(t *testing.T) {
	originalServerURL := settings.ServerURL.Get()
	t.Cleanup(func() { _ = settings.ServerURL.Set(originalServerURL) })
	require.NoError(t, settings.ServerURL.Set("https://rancher.example.com"))

	tests := []struct {
		name      string
		condition func(*apisv3.Cluster)
		wantToken string
		otherOne  string
	}{
		{
			name:      "before an agent connects",
			condition: func(*apisv3.Cluster) {},
			wantToken: "default-secret-value",
			otherOne:  "system-secret-value",
		},
		{
			// Right after pre-bootstrapping, before Connected and AgentDeployed are set: rendering the
			// default-token here rolled out a second agent next to the one clusterDeploy deployed.
			name:      "once pre-bootstrapped",
			condition: func(c *apisv3.Cluster) { apisv3.ClusterConditionPreBootstrapped.True(c) },
			wantToken: "system-secret-value",
			otherOne:  "default-secret-value",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mp := newMockPlanner(t, InfoFunctions{})
			cluster := &apisv3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test"}}
			tt.condition(cluster)
			mp.managementClusters.EXPECT().Get("c-m-test").Return(cluster, nil)
			mp.clusterRegistrationTokenCache.EXPECT().GetByIndex(ClusterRegToken, "c-m-test").Return([]*apisv3.ClusterRegistrationToken{
				newAgentTestRegistrationToken(defaultRegistrationTokenName),
				newAgentTestRegistrationToken(systemRegistrationTokenName),
			}, nil)
			secrets := map[string]string{
				"crt-" + defaultRegistrationTokenName: "default-secret-value",
				"crt-" + systemRegistrationTokenName:  "system-secret-value",
			}
			mp.secretCache.EXPECT().Get("c-m-test", gomock.Any()).DoAndReturn(func(_, name string) (*v1.Secret, error) {
				return &v1.Secret{Data: map[string][]byte{"token": []byte(secrets[name])}}, nil
			}).AnyTimes()

			controlPlane := &rkev1.RKEControlPlane{Spec: rkev1.RKEControlPlaneSpec{
				ManagementClusterName: "c-m-test",
				KubernetesVersion:     "v1.33.1+rke2r1",
			}}
			entry := &planEntry{Machine: &capi.Machine{}, Metadata: &plan.Metadata{}}

			data, err := mp.planner.generateClusterAgentManifest(controlPlane, entry)

			require.NoError(t, err)
			assert.Contains(t, string(data), agentCredentialsSecretName(t, "c-m-test", tt.wantToken))
			assert.NotContains(t, string(data), agentCredentialsSecretName(t, "c-m-test", tt.otherOne))
		})
	}
}
