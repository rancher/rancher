package kubeconfig

import (
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/utils/pointer"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	tokenKey               = "cccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	invalidTokenKey        = "dddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	badHashVersionTokenKey = "$-1:jwvzsLqh6Rg:FyeWbQuUt6VEMhQOe5J1kXPf0D4H9MRjub0aNaGzyx8"
)

func Test_kubeConfigValid(t *testing.T) {
	type kubeconfigData struct {
		serverURL   string
		serverCA    string
		clusterName string
		token       string
	}

	hashToken := func(token *v3.Token, useBadHashVersion bool) *v3.Token {
		features.TokenHashing.Set(true)
		newToken := token.DeepCopy()
		err := tokens.ConvertTokenKeyToHash(newToken)
		if useBadHashVersion {
			// ConvertTokenKeyToHash also adds an annotation, so we want to do this after that call
			newToken.Token = badHashVersionTokenKey
		}
		require.NoError(t, err, "error when hashing token")
		// test only requires an individual token to be hashed
		features.TokenHashing.Set(false)
		return newToken
	}
	cluster := v1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "c-m-1234xyz",
			Namespace: "fleet-default",
		},
	}

	token := v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			// name of the token for the above cluster as calculated by getPrincipalAndUserName
			Name: "u-fngwm5ys7g",
		},
		Token:   tokenKey,
		UserID:  "test-user",
		Enabled: pointer.Bool(true),
	}

	tests := []struct {
		name         string
		currentData  *kubeconfigData
		wantData     *kubeconfigData
		cluster      *v1.Cluster
		storedToken  *v3.Token
		secretLabels map[string]string

		invalidServerURL  bool
		invalidKubeconfig bool
		tokenCacheError   error

		wantError bool
		wantValid bool
	}{
		{
			name: "kubeconfig up to date",
			currentData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "ABC",
				clusterName: "c-1234xyz",
				token:       fmt.Sprintf("%s:%s", token.Name, tokenKey),
			},
			wantData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "ABC",
				clusterName: "c-1234xyz",
			},
			storedToken: &token,
			cluster:     &cluster,
			secretLabels: map[string]string{
				capi.ClusterNameLabel: "c-m-1234xyz",
			},

			wantError: false,
			wantValid: true,
		},
		{
			name: "kubeconfig up to date, hashed token",
			currentData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "ABC",
				clusterName: "c-1234xyz",
				token:       fmt.Sprintf("%s:%s", token.Name, tokenKey),
			},
			wantData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "ABC",
				clusterName: "c-1234xyz",
			},
			storedToken: hashToken(&token, false),
			cluster:     &cluster,
			secretLabels: map[string]string{
				capi.ClusterNameLabel: "c-m-1234xyz",
			},

			wantError: false,
			wantValid: true,
		},
		{
			name: "kubeconfig bad token",
			currentData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "ABC",
				clusterName: "c-1234xyz",
				token:       fmt.Sprintf("%s:%s", token.Name, invalidTokenKey),
			},
			wantData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "ABC",
				clusterName: "c-1234xyz",
			},
			storedToken: &token,
			cluster:     &cluster,
			secretLabels: map[string]string{
				capi.ClusterNameLabel: "c-m-1234xyz",
			},

			wantError: false,
			wantValid: false,
		},
		{
			name: "kubeconfig bad token, token hashing enabled",
			currentData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "ABC",
				clusterName: "c-1234xyz",
				token:       fmt.Sprintf("%s:%s", token.Name, invalidTokenKey),
			},
			wantData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "ABC",
				clusterName: "c-1234xyz",
			},
			storedToken: hashToken(&token, false),
			cluster:     &cluster,
			secretLabels: map[string]string{
				capi.ClusterNameLabel: "c-m-1234xyz",
			},

			wantError: false,
			wantValid: false,
		},
		{
			name: "changed server url",
			currentData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "ABC",
				clusterName: "c-1234xyz",
				token:       fmt.Sprintf("%s:%s", token.Name, tokenKey),
			},
			wantData: &kubeconfigData{
				serverURL:   "https://test.new.io",
				serverCA:    "ABC",
				clusterName: "c-1234xyz",
			},
			storedToken: &token,
			cluster:     &cluster,
			secretLabels: map[string]string{
				capi.ClusterNameLabel: "c-m-1234xyz",
			},

			wantError: false,
			wantValid: false,
		},
		{
			name: "changed server CA",
			currentData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "ABC",
				clusterName: "c-1234xyz",
				token:       fmt.Sprintf("%s:%s", token.Name, tokenKey),
			},
			wantData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "XYZ",
				clusterName: "c-1234xyz",
			},
			storedToken: &token,
			cluster:     &cluster,
			secretLabels: map[string]string{
				capi.ClusterNameLabel: "c-m-1234xyz",
			},

			wantError: false,
			wantValid: false,
		},
		{
			name: "changed cluster name",
			currentData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "ABC",
				clusterName: "c-1234xyz",
				token:       fmt.Sprintf("%s:%s", token.Name, tokenKey),
			},
			wantData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "ABC",
				clusterName: "c-1234abc",
			},
			storedToken: &token,
			cluster:     &cluster,
			secretLabels: map[string]string{
				capi.ClusterNameLabel: "c-m-1234xyz",
			},

			wantError: false,
			wantValid: false,
		},
		{
			name:        "empty kubeconfig data",
			currentData: nil,
			wantData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "ABC",
				clusterName: "c-1234abc",
			},
			storedToken: &token,
			cluster:     &cluster,
			secretLabels: map[string]string{
				capi.ClusterNameLabel: "c-m-1234xyz",
			},

			wantError: true,
			wantValid: false,
		},
		{
			name:        "current kubeconfig invalid",
			currentData: nil,
			wantData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "ABC",
				clusterName: "c-1234abc",
			},
			storedToken: &token,
			cluster:     &cluster,
			secretLabels: map[string]string{
				capi.ClusterNameLabel: "c-m-1234xyz",
			},
			invalidKubeconfig: true,

			wantError: true,
			wantValid: false,
		},
		{
			name: "CAPI cluster label is set correctly",
			currentData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "ABC",
				clusterName: "c-1234xyz",
				token:       fmt.Sprintf("%s:%s", token.Name, tokenKey),
			},
			wantData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "ABC",
				clusterName: "c-1234xyz",
			},
			storedToken: &token,
			cluster:     &cluster,
			secretLabels: map[string]string{
				capi.ClusterNameLabel: "c-m-1234xyz",
			},

			wantError: false,
			wantValid: true,
		},
		{
			name: "CAPI cluster label is set incorrectly",
			currentData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "ABC",
				clusterName: "c-1234xyz",
				token:       fmt.Sprintf("%s:%s", token.Name, tokenKey),
			},
			wantData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "ABC",
				clusterName: "c-1234xyz",
			},
			storedToken: &token,
			cluster:     &cluster,
			secretLabels: map[string]string{
				capi.ClusterNameLabel: "random-cluster-name",
			},

			wantError: false,
			wantValid: false,
		},
		{
			name: "CAPI cluster label is missing",
			currentData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "ABC",
				clusterName: "c-1234xyz",
				token:       fmt.Sprintf("%s:%s", token.Name, tokenKey),
			},
			wantData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "ABC",
				clusterName: "c-1234xyz",
			},
			storedToken:       &token,
			cluster:           &cluster,
			invalidKubeconfig: true,
			secretLabels:      map[string]string{},

			wantError: false,
			wantValid: false,
		},
		{
			name: "invalid server url",
			currentData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "ABC",
				clusterName: "c-1234abc",
			},
			wantData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "ABC",
				clusterName: "c-1234abc",
			},
			storedToken: &token,
			cluster:     &cluster,
			secretLabels: map[string]string{
				capi.ClusterNameLabel: "c-m-1234xyz",
			},
			invalidServerURL: true,

			wantError: true,
			wantValid: false,
		},
		{
			name: "invalid hash version on stored token",
			currentData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "ABC",
				clusterName: "c-1234abc",
			},
			wantData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "ABC",
				clusterName: "c-1234abc",
			},
			storedToken: hashToken(&token, true),
			cluster:     &cluster,
			secretLabels: map[string]string{
				capi.ClusterNameLabel: "c-m-1234xyz",
			},

			wantError: true,
			wantValid: false,
		},
		{
			name: "unable to get stored token",
			currentData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "ABC",
				clusterName: "c-1234abc",
			},
			wantData: &kubeconfigData{
				serverURL:   "https://test.cluster.io",
				serverCA:    "ABC",
				clusterName: "c-1234abc",
			},
			cluster: &cluster,
			secretLabels: map[string]string{
				capi.ClusterNameLabel: "c-m-1234xyz",
			},
			tokenCacheError: fmt.Errorf("server unavailable"),

			wantError: true,
			wantValid: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var kcData []byte
			if test.currentData != nil {
				var err error
				var serverURL string
				if test.invalidServerURL {
					serverURL = test.currentData.serverURL
				} else {
					serverURL = fmt.Sprintf("%s/k8s/clusters/%s", test.currentData.serverURL, test.currentData.clusterName)
				}
				config := clientcmdapi.Config{
					Clusters: map[string]*clientcmdapi.Cluster{
						"cluster": {
							Server:                   serverURL,
							CertificateAuthorityData: []byte(test.currentData.serverCA),
						},
					},
					AuthInfos: map[string]*clientcmdapi.AuthInfo{
						"user": {
							Token: test.currentData.token,
						},
					},
					Contexts: map[string]*clientcmdapi.Context{
						"default": {
							Cluster:  "cluster",
							AuthInfo: "user",
						},
					},
					CurrentContext: "default",
				}
				kcData, err = clientcmd.Write(config)
				require.NoError(t, err, "error when writing kubeconfig for test setup")

			} else if test.invalidKubeconfig {
				kcData = []byte{'a'}
			}

			ctrl := gomock.NewController(t)
			mockCache := fake.NewMockNonNamespacedCacheInterface[*v3.Token](ctrl)

			mockCache.EXPECT().Get(gomock.Any()).AnyTimes().DoAndReturn(func(name string) (*v3.Token, error) {
				if test.storedToken != nil && name == test.storedToken.Name {
					return test.storedToken, nil
				}
				if test.tokenCacheError != nil {
					return nil, test.tokenCacheError
				}
				return nil, apierrors.NewNotFound(schema.GroupResource{Group: "management.cattle.io", Resource: "Token"}, name)

			})
			m := Manager{
				tokensCache: mockCache,
			}
			isError, isValid := m.kubeConfigValid(kcData, test.cluster, test.wantData.serverURL, test.wantData.serverCA, test.wantData.clusterName, test.secretLabels)
			require.Equal(t, test.wantError, isError)
			require.Equal(t, test.wantValid, isValid)
		})
	}
}
