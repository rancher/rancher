package kubeconfig

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"testing"
	"time"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	authTokens "github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/user"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/rancher/wrangler/v3/pkg/randomtoken"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	k8scorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8suser "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/tools/clientcmd"
)

type fakeUserManager struct {
	clusterTokens          []string
	tokens                 []string
	ensureClusterTokenFunc func(clusterID string, input user.TokenInput) (string, error)
	ensureTokenFunc        func(input user.TokenInput) (string, error)
}

func (f *fakeUserManager) EnsureClusterToken(clusterID string, input user.TokenInput) (string, error) {
	if f.ensureClusterTokenFunc != nil {
		return f.ensureClusterTokenFunc(clusterID, input)
	}
	token, err := f.Generate()
	if err != nil {
		return "", err
	}
	f.clusterTokens = append(f.clusterTokens, token)
	return token, nil
}

func (f *fakeUserManager) EnsureToken(input user.TokenInput) (string, error) {
	if f.ensureTokenFunc != nil {
		return f.ensureTokenFunc(input)
	}
	token, err := f.Generate()
	if err != nil {
		return "", err
	}
	f.tokens = append(f.tokens, token)
	return token, nil
}

func (f *fakeUserManager) Generate() (string, error) {
	key, err := randomtoken.Generate()
	if err != nil {
		return "", err
	}
	name := names.SimpleNameGenerator.GenerateName("token-")
	return name + ":" + key, nil
}

func TestIsUnique(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		ids    []string
		unique bool
	}{
		{
			name:   "unique IDs",
			ids:    []string{"id1", "id2", "id3"},
			unique: true,
		},
		{
			name: "duplicate IDs",
			ids:  []string{"id1", "id2", "id1"},
		},
		{
			name:   "empty list",
			ids:    []string{},
			unique: true,
		},
		{
			name:   "single ID",
			ids:    []string{"id1"},
			unique: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.unique, isUnique(tt.ids))
		})
	}
}
func TestStoreNew(t *testing.T) {
	t.Parallel()

	store := &Store{}
	obj := store.New()
	require.NotNil(t, obj)
	assert.IsType(t, &ext.Kubeconfig{}, obj)
}

func TestStoreUserFrom(t *testing.T) {
	t.Parallel()

	systemAdmin := "system:admin"
	rancherAdmin := "user-2p7w6"
	rancherUser := "u-s857n"

	ctrl := gomock.NewController(t)
	userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	userCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.User, error) {
		switch name {
		case rancherAdmin, rancherUser:
			return &v3.User{
				ObjectMeta: metav1.ObjectMeta{Name: name},
			}, nil
		case "error":
			return nil, fmt.Errorf("some error")
		default:
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}
	}).AnyTimes()

	store := &Store{
		userCache: userCache,
		authorizer: authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
			switch a.GetUser().GetName() {
			case systemAdmin, rancherAdmin:
				return authorizer.DecisionAllow, "", nil
			default:
				return authorizer.DecisionDeny, "", nil
			}
		}),
	}

	t.Run("global admin", func(t *testing.T) {
		userInfo, isAdmin, isRancherUser, err := store.userFrom(request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: "system:admin",
		}))
		require.NoError(t, err)
		assert.NotNil(t, userInfo)
		assert.Equal(t, systemAdmin, userInfo.GetName())
		assert.True(t, isAdmin)
		assert.False(t, isRancherUser)
	})

	t.Run("rancher admin", func(t *testing.T) {
		userInfo, isAdmin, isRancherUser, err := store.userFrom(request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: rancherAdmin,
		}))
		require.NoError(t, err)
		assert.NotNil(t, userInfo)
		assert.Equal(t, rancherAdmin, userInfo.GetName())
		assert.True(t, isAdmin)
		assert.True(t, isRancherUser)
	})

	t.Run("rancher user", func(t *testing.T) {
		userInfo, isAdmin, isRancherUser, err := store.userFrom(request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: rancherUser,
		}))
		require.NoError(t, err)
		assert.NotNil(t, userInfo)
		assert.Equal(t, rancherUser, userInfo.GetName())
		assert.False(t, isAdmin)
		assert.True(t, isRancherUser)
	})

	t.Run("user not found", func(t *testing.T) {
		userInfo, isAdmin, isRancherUser, err := store.userFrom(request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: "not-found",
		}))
		require.NoError(t, err)
		assert.NotNil(t, userInfo)
		assert.False(t, isAdmin)
		assert.False(t, isRancherUser)
	})

	t.Run("no user info", func(t *testing.T) {
		_, _, _, err := store.userFrom(context.Background())
		require.Error(t, err)
	})

	t.Run("error retrieving user", func(t *testing.T) {
		_, _, _, err := store.userFrom(request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: "error",
		}))
		require.Error(t, err)
	})
}

var allowAll = authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
	return authorizer.DecisionAllow, "", nil
})

func TestStoreCreate(t *testing.T) {
	t.Parallel()

	userID := "user-2p7w6"
	authTokenID := "token-nh98r"
	serverURL := "https://rancher.example.com"
	downstream1 := "c-m-tbgzfbgf"
	downstream2 := "c-m-bxn2p7w6" // ACE enabled.

	_, localCACert, err := generateCAKeyAndCert()
	require.NoError(t, err)
	_, downstream1CACert, err := generateCAKeyAndCert()
	require.NoError(t, err)
	_, downstream2CACert, err := generateCAKeyAndCert()
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	userCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.User, error) {
		switch name {
		case userID:
			return &v3.User{}, nil
		case "error":
			return nil, fmt.Errorf("some error")
		default:
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}
	}).AnyTimes()
	tokenCache := fake.NewMockNonNamespacedCacheInterface[*v3.Token](ctrl)
	tokenCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.Token, error) {
		return &v3.Token{
			ObjectMeta: metav1.ObjectMeta{
				Name: authTokenID,
			},
			UserID: userID,
		}, nil
	}).Times(1)
	tokenCache.EXPECT().List(gomock.Any()).DoAndReturn(func(selector labels.Selector) ([]*v3.Token, error) {
		return nil, nil
	}).AnyTimes()

	clusterCache := fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl)
	clusterCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.Cluster, error) {
		switch name {
		case "local":
			return &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "local"},
				Spec:       v3.ClusterSpec{DisplayName: "local"},
				Status:     v3.ClusterStatus{CACert: localCACert},
			}, nil
		case downstream1:
			return &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: downstream1},
				Spec:       v3.ClusterSpec{DisplayName: "downstream1"},
				Status:     v3.ClusterStatus{CACert: downstream1CACert},
			}, nil
		case downstream2:
			return &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: downstream2},
				Spec: v3.ClusterSpec{
					DisplayName: "downstream2",
					ClusterSpecBase: v3.ClusterSpecBase{
						LocalClusterAuthEndpoint: v3.LocalClusterAuthEndpoint{
							Enabled: true,
						},
					},
				},
				Status: v3.ClusterStatus{CACert: downstream2CACert},
			}, nil
		default:
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}
	}).AnyTimes()
	nodeCache := fake.NewMockCacheInterface[*v3.Node](ctrl)
	nodeCache.EXPECT().List(gomock.Any(), labels.Everything()).DoAndReturn(func(namespace string, selector labels.Selector) ([]*v3.Node, error) {
		switch namespace {
		case downstream1:
			return nil, nil
		case downstream2:
			return []*v3.Node{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "cp"},
					Spec: v3.NodeSpec{
						RequestedHostname: "cp",
						ControlPlane:      true,
					},
					Status: v3.NodeStatus{
						InternalNodeStatus: k8scorev1.NodeStatus{
							Addresses: []k8scorev1.NodeAddress{
								{Type: k8scorev1.NodeExternalIP, Address: "172.20.0.3"},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "worker"},
					Spec:       v3.NodeSpec{RequestedHostname: "worker"},
					Status: v3.NodeStatus{
						InternalNodeStatus: k8scorev1.NodeStatus{
							Addresses: []k8scorev1.NodeAddress{
								{Type: k8scorev1.NodeExternalIP, Address: "172.20.0.4"},
							},
						},
					},
				},
			}, nil
		default:
			require.Fail(t, "unexpected call")
			return nil, nil
		}
	}).AnyTimes()

	userManager := &fakeUserManager{}

	store := &Store{
		authorizer:   allowAll,
		userCache:    userCache,
		tokenCache:   tokenCache,
		clusterCache: clusterCache,
		nodeCache:    nodeCache,
		userMgr:      userManager,
		getServerURL: func() string {
			return serverURL
		},
		getDefaultTTL: func() (*int64, error) {
			ttl := int64(43200)
			return &ttl, nil
		},
		shouldGenerateToken: func() bool {
			return true
		},
	}

	ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
		Name: userID,
		Extra: map[string][]string{
			common.ExtraRequestTokenID: {authTokenID},
		},
	})
	kubeconfig := &ext.Kubeconfig{
		Spec: ext.KubeconfigSpec{
			Clusters:       []string{downstream1, downstream2},
			CurrentContext: downstream1,
		},
	}
	var createValidationCalled bool
	createValidation := func(ctx context.Context, obj runtime.Object) error {
		createValidationCalled = true
		return nil
	}
	options := &metav1.CreateOptions{}

	obj, err := store.Create(ctx, kubeconfig, createValidation, options)
	require.NoError(t, err)
	assert.NotNil(t, obj)
	assert.IsType(t, &ext.Kubeconfig{}, obj)

	assert.True(t, createValidationCalled)

	generated := obj.(*ext.Kubeconfig)
	assert.NotEmpty(t, generated.Status)
	assert.NotEmpty(t, generated.Status.Value)

	require.Len(t, userManager.tokens, 1)

	config, err := clientcmd.Load([]byte(generated.Status.Value))
	require.NoError(t, err)
	require.Len(t, config.Clusters, 4)
	assert.Equal(t, serverURL, config.Clusters["rancher"].Server)
	assert.Equal(t, fmt.Sprintf("%s/k8s/clusters/%s", serverURL, downstream1), config.Clusters["downstream1"].Server)
	assert.Equal(t, fmt.Sprintf("%s/k8s/clusters/%s", serverURL, downstream2), config.Clusters["downstream2"].Server)
	assert.Equal(t, "https://172.20.0.3:6443", config.Clusters["downstream2-cp"].Server)

	require.Len(t, config.Contexts, 4)
	assert.Equal(t, "rancher", config.Contexts["rancher"].Cluster)
	assert.Equal(t, "downstream1", config.Contexts["downstream1"].Cluster)
	assert.Equal(t, "downstream1", config.Contexts["downstream1"].AuthInfo)
	assert.Equal(t, "downstream2", config.Contexts["downstream2"].Cluster)
	assert.Equal(t, "downstream2", config.Contexts["downstream2"].AuthInfo)
	assert.Equal(t, "downstream2-cp", config.Contexts["downstream2-cp"].Cluster)
	assert.Equal(t, "downstream2", config.Contexts["downstream2"].AuthInfo)

	require.Len(t, userManager.tokens, 1)
	require.Len(t, userManager.clusterTokens, 1)

	require.Len(t, config.AuthInfos, 3)
	assert.Equal(t, userManager.tokens[0], config.AuthInfos["rancher"].Token)
	assert.Equal(t, userManager.tokens[0], config.AuthInfos["downstream1"].Token)
	assert.Equal(t, userManager.clusterTokens[0], config.AuthInfos["downstream2"].Token)
}

func TestStoreList(t *testing.T) {
	t.Parallel()

	adminID := "user-2p7w6"
	userID := "u-s857n"
	authTokenID := "token-nh98r"
	kubeconfigID1 := "kubeconfig-4bgj2"
	kubeconfigID2 := "kubeconfig-c3km5"
	kubeconfigID3 := "kubeconfig-fk1mn"

	now := time.Now()

	tokens := []*v3.Token{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "kubeconfig-user-2p7w6abcde",
				CreationTimestamp: metav1.NewTime(now.Add(-time.Hour)),
				Labels: map[string]string{
					authTokens.TokenKindLabel:         "kubeconfig",
					authTokens.TokenKubeconfigIDLabel: kubeconfigID1,
					authTokens.UserIDLabel:            adminID,
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "kubeconfig-user-2p7w6fghij",
				CreationTimestamp: metav1.NewTime(now.Add(-time.Hour)),
				Labels: map[string]string{
					authTokens.TokenKindLabel:         "kubeconfig",
					authTokens.TokenKubeconfigIDLabel: kubeconfigID2,
					authTokens.UserIDLabel:            adminID,
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "kubeconfig-user-2p7w6klmno",
				CreationTimestamp: metav1.NewTime(now.Add(-time.Hour)),
				Labels: map[string]string{
					authTokens.TokenKindLabel:         "kubeconfig",
					authTokens.TokenKubeconfigIDLabel: kubeconfigID2,
					authTokens.UserIDLabel:            adminID,
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "kubeconfig-u-s857nklmno",
				CreationTimestamp: metav1.NewTime(now.Add(-time.Hour)),
				Labels: map[string]string{
					authTokens.TokenKindLabel:         "kubeconfig",
					authTokens.TokenKubeconfigIDLabel: kubeconfigID3,
					authTokens.UserIDLabel:            userID,
				},
			},
		},
	}

	ctrl := gomock.NewController(t)
	userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	userCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.User, error) {
		switch name {
		case adminID, userID:
			return &v3.User{ObjectMeta: metav1.ObjectMeta{Name: name}}, nil
		case "error":
			return nil, fmt.Errorf("some error")
		default:
			return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
		}
	}).AnyTimes()

	userManager := &fakeUserManager{}

	t.Run("admin", func(t *testing.T) {
		tokenCache := fake.NewMockNonNamespacedCacheInterface[*v3.Token](ctrl)
		tokenCache.EXPECT().List(gomock.Any()).DoAndReturn(func(selector labels.Selector) ([]*v3.Token, error) {
			requirements, selectable := selector.Requirements()
			assert.True(t, selectable)
			require.Len(t, requirements, 1)
			value, ok := selector.RequiresExactMatch(authTokens.TokenKindLabel)
			assert.True(t, ok)
			assert.Equal(t, "kubeconfig", value)

			return tokens, nil
		}).AnyTimes()

		store := &Store{
			authorizer: allowAll,
			userCache:  userCache,
			tokenCache: tokenCache,
			userMgr:    userManager,
		}

		ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: userID,
			Extra: map[string][]string{
				common.ExtraRequestTokenID: {authTokenID},
			},
		})

		obj, err := store.List(ctx, nil)
		require.NoError(t, err)
		require.NotNil(t, obj)
		assert.IsType(t, &ext.KubeconfigList{}, obj)

		list := obj.(*ext.KubeconfigList)
		require.Len(t, list.Items, 3)
		assert.Equal(t, kubeconfigID1, list.Items[0].Name)
		assert.Equal(t, kubeconfigID2, list.Items[1].Name)
		assert.Equal(t, kubeconfigID3, list.Items[2].Name)
	})

	t.Run("user", func(t *testing.T) {
		authorizer := authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
			return authorizer.DecisionDeny, "", nil
		})

		tokenCache := fake.NewMockNonNamespacedCacheInterface[*v3.Token](ctrl)
		tokenCache.EXPECT().List(gomock.Any()).DoAndReturn(func(selector labels.Selector) ([]*v3.Token, error) {
			requirements, selectable := selector.Requirements()
			assert.True(t, selectable)
			require.Len(t, requirements, 2)
			value, ok := selector.RequiresExactMatch(authTokens.TokenKindLabel)
			assert.True(t, ok)
			assert.Equal(t, "kubeconfig", value)
			value, ok = selector.RequiresExactMatch(authTokens.UserIDLabel)
			assert.True(t, ok)
			assert.Equal(t, userID, value)

			return tokens[0:3], nil
		}).AnyTimes()

		store := &Store{
			authorizer: authorizer,
			userCache:  userCache,
			tokenCache: tokenCache,
			userMgr:    userManager,
		}

		ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: userID,
			Extra: map[string][]string{
				common.ExtraRequestTokenID: {authTokenID},
			},
		})

		obj, err := store.List(ctx, nil)
		require.NoError(t, err)
		require.NotNil(t, obj)
		assert.IsType(t, &ext.KubeconfigList{}, obj)

		list := obj.(*ext.KubeconfigList)
		require.Len(t, list.Items, 2)
		assert.Equal(t, kubeconfigID1, list.Items[0].Name)
		assert.Equal(t, kubeconfigID2, list.Items[1].Name)
	})
}

func generateCAKeyAndCert() (*ecdsa.PrivateKey, string, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, "", err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, "", err
	}

	pem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	return key, base64.StdEncoding.EncodeToString(pem), nil
}
