package kubeconfig

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/user"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/rancher/wrangler/v3/pkg/randomtoken"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/watch"
	k8suser "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/printers"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

var (
	adminID = "user-2p7w6"
	adminSA = "system:admin"
	userID  = "u-w7drc"
)

var commonAuthorizer = authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
	switch a.GetUser().GetName() {
	case adminSA, adminID:
		return authorizer.DecisionAllow, "", nil
	default:
		return authorizer.DecisionDeny, "", nil
	}
})

type fakeTokenManager struct {
	clusterTokens          []string
	tokens                 []string
	ensureClusterTokenFunc func(clusterID string, input user.TokenInput) (string, runtime.Object, error)
	ensureTokenFunc        func(input user.TokenInput) (string, runtime.Object, error)
}

func (f *fakeTokenManager) EnsureClusterToken(clusterID string, input user.TokenInput) (string, runtime.Object, error) {
	if f.ensureClusterTokenFunc != nil {
		return f.ensureClusterTokenFunc(clusterID, input)
	}
	tokenKey, token, err := f.Generate()
	if err != nil {
		return "", nil, err
	}
	f.clusterTokens = append(f.clusterTokens, tokenKey)
	return tokenKey, token, nil
}

func (f *fakeTokenManager) EnsureToken(input user.TokenInput) (string, runtime.Object, error) {
	if f.ensureTokenFunc != nil {
		return f.ensureTokenFunc(input)
	}
	tokenKey, token, err := f.Generate()
	if err != nil {
		return "", nil, err
	}

	f.tokens = append(f.tokens, tokenKey)
	return tokenKey, token, nil
}

func (f *fakeTokenManager) Generate() (string, runtime.Object, error) {
	key, err := randomtoken.Generate()
	if err != nil {
		return "", nil, err
	}
	name := names.SimpleNameGenerator.GenerateName("token-")

	token := &v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			UID:  uuid.NewUUID(),
		},
	}

	return name + ":" + key, token, nil
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

func TestStoreNewList(t *testing.T) {
	store := &Store{}
	obj := store.NewList()
	require.NotNil(t, obj)
	require.IsType(t, &ext.KubeconfigList{}, obj)

	list := obj.(*ext.KubeconfigList)
	assert.Nil(t, list.Items)
}

func TestStoreGetSingularName(t *testing.T) {
	store := &Store{}
	assert.Equal(t, Singular, store.GetSingularName())
}

func TestStoreNamespaceScoped(t *testing.T) {
	store := &Store{}
	assert.False(t, store.NamespaceScoped())
}

func TestStoreGroupVersionKind(t *testing.T) {
	store := &Store{}
	assert.Equal(t, Kind, store.GroupVersionKind(ext.SchemeGroupVersion).Kind)
}

func TestStoreUserFrom(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	existingUserID := "user-2p7w6"
	userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	userCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.User, error) {
		switch name {
		case existingUserID:
			return &v3.User{}, nil
		case "error":
			return nil, fmt.Errorf("some error")
		default:
			return nil, apierrors.NewNotFound(gvr.GroupResource(), name)
		}
	}).AnyTimes()

	store := &Store{
		authorizer: commonAuthorizer,
		userCache:  userCache,
	}

	t.Run("valid authenticated user", func(t *testing.T) {
		t.Parallel()

		userInfo, _, _, err := store.userFrom(request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: existingUserID,
		}), "get")
		require.NoError(t, err)
		assert.NotNil(t, userInfo)
		assert.Equal(t, existingUserID, userInfo.GetName())
	})

	t.Run("no user info", func(t *testing.T) {
		t.Parallel()

		userInfo, _, _, err := store.userFrom(context.Background(), "get")
		require.Error(t, err)
		assert.Nil(t, userInfo)
	})

	t.Run("admin service account", func(t *testing.T) {
		t.Parallel()

		_, isAdmin, isRancherUser, err := store.userFrom(request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: "system:admin",
		}), "get")
		require.NoError(t, err)
		assert.True(t, isAdmin)
		assert.False(t, isRancherUser)
	})

	t.Run("user not found", func(t *testing.T) {
		t.Parallel()

		_, isAdmin, isRancherUser, err := store.userFrom(request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: "non-existent",
		}), "get")
		require.NoError(t, err)
		assert.False(t, isAdmin)
		assert.False(t, isRancherUser)
	})

	t.Run("error retrieving user", func(t *testing.T) {
		t.Parallel()

		_, _, _, err := store.userFrom(request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: "error",
		}), "get")
		require.Error(t, err)
	})
}

func TestStoreCreate(t *testing.T) {
	authTokenID := "token-nh98r"
	serverURL := "https://rancher.example.com"
	getServerURL := func() string { return serverURL }
	downstream1 := "c-m-tbgzfbgf"
	downstream2 := "c-m-bxn2p7w6" // ACE enabled.

	_, rancherCACert, err := generateCAKeyAndCert()
	require.NoError(t, err)
	_, localCACert, err := generateCAKeyAndCert()
	require.NoError(t, err)
	_, downstream1CACert, err := generateCAKeyAndCert()
	require.NoError(t, err)
	_, downstream2CACert, err := generateCAKeyAndCert()
	require.NoError(t, err)

	localCluster := &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "local"},
		Spec:       v3.ClusterSpec{DisplayName: "local"},
		Status:     v3.ClusterStatus{CACert: base64.StdEncoding.EncodeToString([]byte(localCACert))},
	}
	downstream1Cluster := &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: downstream1},
		Spec:       v3.ClusterSpec{DisplayName: "downstream1"},
		Status:     v3.ClusterStatus{CACert: base64.StdEncoding.EncodeToString([]byte(downstream1CACert))},
	}
	downstream2Cluster := &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: downstream2},
		Spec: v3.ClusterSpec{
			DisplayName: "downstream2",
			ClusterSpecBase: v3.ClusterSpecBase{
				LocalClusterAuthEndpoint: v3.LocalClusterAuthEndpoint{
					Enabled: true,
				},
			},
		},
		Status: v3.ClusterStatus{CACert: base64.StdEncoding.EncodeToString([]byte(downstream2CACert))},
	}

	ctrl := gomock.NewController(t)
	userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	userCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.User, error) {
		switch name {
		case userID, adminID:
			return &v3.User{}, nil
		case "error":
			return nil, fmt.Errorf("some error")
		default:
			return nil, apierrors.NewNotFound(gvr.GroupResource(), name)
		}
	}).AnyTimes()

	tokenCache := fake.NewMockNonNamespacedCacheInterface[*v3.Token](ctrl)
	tokenCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.Token, error) {
		switch name {
		case authTokenID:
			return &v3.Token{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				UserID: userID,
			}, nil
		default:
			return nil, apierrors.NewNotFound(gvr.GroupResource(), name)
		}
	}).AnyTimes()
	tokenCache.EXPECT().List(gomock.Any()).DoAndReturn(func(selector labels.Selector) ([]*v3.Token, error) {
		return nil, nil
	}).AnyTimes()

	clusterCache := fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl)
	clusterCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.Cluster, error) {
		switch name {
		case "local":
			return localCluster.DeepCopy(), nil
		case downstream1:
			return downstream1Cluster.DeepCopy(), nil
		case downstream2:
			return downstream2Cluster.DeepCopy(), nil
		default:
			return nil, apierrors.NewNotFound(gvr.GroupResource(), name)
		}
	}).AnyTimes()
	clusterCache.EXPECT().List(labels.Everything()).DoAndReturn(func(selector labels.Selector) ([]*v3.Cluster, error) {
		return []*v3.Cluster{
			localCluster.DeepCopy(),
			downstream1Cluster.DeepCopy(),
			downstream2Cluster.DeepCopy(),
		}, nil
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
						InternalNodeStatus: corev1.NodeStatus{
							Addresses: []corev1.NodeAddress{
								{Type: corev1.NodeExternalIP, Address: "172.20.0.3"},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "worker"},
					Spec:       v3.NodeSpec{RequestedHostname: "worker"},
					Status: v3.NodeStatus{
						InternalNodeStatus: corev1.NodeStatus{
							Addresses: []corev1.NodeAddress{
								{Type: corev1.NodeExternalIP, Address: "172.20.0.4"},
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

	nsCache := fake.NewMockNonNamespacedCacheInterface[*corev1.Namespace](ctrl)
	nsCache.EXPECT().Get(namespace).AnyTimes()

	defaultTTLSeconds := int64(43200)
	getDefaultTTL := func() (*int64, error) {
		millis := defaultTTLSeconds * 1000
		return &millis, nil
	}
	shouldGenerateToken := func() bool { return true }
	options := &metav1.CreateOptions{}
	tokenManager := &fakeTokenManager{}

	t.Run("user creates a kubeconfig", func(t *testing.T) {
		authorizer := authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
			return authorizer.DecisionAllow, "", nil
		})

		var configMap *corev1.ConfigMap
		configMapClient := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
		configMapClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(obj *corev1.ConfigMap) (*corev1.ConfigMap, error) {
			require.Equal(t, namePrefix, obj.GenerateName)

			configMap = obj.DeepCopy()
			loc, err := time.LoadLocation("Europe/London") // This is to ensure we don't use html encoding e.g. "+01:00" -> "&#43;01:00"
			require.NoError(t, err)
			configMap.CreationTimestamp = metav1.NewTime(time.Now().In(loc))
			configMap.Name = names.SimpleNameGenerator.GenerateName(configMap.GenerateName)
			return configMap, nil
		}).Times(1)
		configMapClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(obj *corev1.ConfigMap) (*corev1.ConfigMap, error) {
			configMap = obj.DeepCopy()
			return configMap, nil
		}).Times(1)

		tokenManager := &fakeTokenManager{} // Subtest specific instance.

		store := &Store{
			mcmEnabled:          true,
			authorizer:          authorizer,
			nsCache:             nsCache,
			configMapClient:     configMapClient,
			userCache:           userCache,
			tokenCache:          tokenCache,
			clusterCache:        clusterCache,
			nodeCache:           nodeCache,
			tokenMgr:            tokenManager,
			getCACert:           func() string { return rancherCACert },
			getDefaultTTL:       getDefaultTTL,
			getServerURL:        getServerURL,
			shouldGenerateToken: shouldGenerateToken,
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

		obj, err := store.Create(ctx, kubeconfig, createValidation, options)
		require.NoError(t, err)
		assert.NotNil(t, obj)
		assert.IsType(t, &ext.Kubeconfig{}, obj)

		assert.True(t, createValidationCalled)

		created := obj.(*ext.Kubeconfig)
		assert.NotEmpty(t, created.Name)
		assert.Empty(t, created.Namespace) // Kubeconfig is a cluster scoped resource.
		assert.Equal(t, defaultTTLSeconds, created.Spec.TTL)
		assert.Equal(t, kubeconfig.Spec.Description, created.Spec.Description)
		assert.NotEmpty(t, created.Spec.CurrentContext)
		assert.Equal(t, kubeconfig.Spec.Clusters, created.Spec.Clusters)

		require.NotNil(t, configMap)
		assert.Equal(t, created.Name, configMap.Name) // Check against the created Kubeconfig instance.
		assert.Equal(t, namespace, configMap.Namespace)
		require.NotNil(t, configMap.Labels)
		assert.Equal(t, userID, configMap.Labels[UserIDLabel])
		assert.Equal(t, KindLabelValue, configMap.Labels[KindLabel])
		require.NotNil(t, configMap.Annotations)
		assert.NotEmpty(t, configMap.Annotations[UIDAnnotation])
		require.NotNil(t, configMap.Data)
		assert.Equal(t, strconv.FormatInt(defaultTTLSeconds, 10), configMap.Data[TTLField])
		assert.Equal(t, kubeconfig.Spec.Description, configMap.Data[DescriptionField])
		assert.Equal(t, created.Spec.CurrentContext, configMap.Data[CurrentContextField]) // Check against the created Kubeconfig instance.
		clustersValue, err := json.Marshal(kubeconfig.Spec.Clusters)
		require.NoError(t, err)
		assert.Equal(t, string(clustersValue), configMap.Data[ClustersField])

		require.NotNil(t, created.Status)
		assert.NotEmpty(t, created.Status.Value)
		assert.Equal(t, StatusSummaryComplete, created.Status.Summary)
		require.Len(t, created.Status.Conditions, 2)
		require.Len(t, created.Status.Tokens, 2)

		scanner := bufio.NewScanner(strings.NewReader(created.Status.Value))
		var lines int
		for scanner.Scan() && lines < 4 {
			line := scanner.Text()

			switch lines {
			case 0:
				assert.Equal(t, "# Generated by Rancher", line)
			case 1:
				assert.Equal(t, "# name: "+created.Name, line)
			case 2:
				assert.Equal(t, "# createdTimestamp: "+configMap.CreationTimestamp.Time.Format(time.RFC3339), line)
			case 3:
				assert.Equal(t, "# ttl: "+strconv.FormatInt(defaultTTLSeconds, 10), line)
			default:
			}
			lines++
		}
		require.NoError(t, scanner.Err())

		config, err := clientcmd.Load([]byte(created.Status.Value))
		require.NoError(t, err)
		require.Len(t, config.Clusters, 4)
		assert.Equal(t, serverURL, config.Clusters[defaultClusterName].Server)
		assert.Equal(t, rancherCACert, string(config.Clusters[defaultClusterName].CertificateAuthorityData))
		assert.Equal(t, fmt.Sprintf("%s/k8s/clusters/%s", serverURL, downstream1), config.Clusters["downstream1"].Server)
		assert.Equal(t, fmt.Sprintf("%s/k8s/clusters/%s", serverURL, downstream2), config.Clusters["downstream2"].Server)
		assert.Equal(t, "https://172.20.0.3:6443", config.Clusters["downstream2-cp"].Server)
		assert.Equal(t, downstream2CACert, string(config.Clusters["downstream2-cp"].CertificateAuthorityData))

		require.Len(t, config.Contexts, 4)
		assert.Equal(t, defaultClusterName, config.Contexts[defaultClusterName].Cluster)
		assert.Equal(t, defaultClusterName, config.Contexts[defaultClusterName].AuthInfo)
		assert.Equal(t, "downstream1", config.Contexts["downstream1"].Cluster)
		assert.Equal(t, defaultClusterName, config.Contexts["downstream1"].AuthInfo)
		assert.Equal(t, "downstream2", config.Contexts["downstream2"].Cluster)
		assert.Equal(t, "downstream2", config.Contexts["downstream2"].AuthInfo)
		assert.Equal(t, "downstream2-cp", config.Contexts["downstream2-cp"].Cluster)
		assert.Equal(t, "downstream2", config.Contexts["downstream2-cp"].AuthInfo)

		require.Len(t, config.AuthInfos, 2)
		require.Len(t, tokenManager.tokens, 1)
		assert.Equal(t, tokenManager.tokens[0], config.AuthInfos[defaultClusterName].Token)
		require.Len(t, tokenManager.clusterTokens, 1)
		assert.Equal(t, tokenManager.clusterTokens[0], config.AuthInfos["downstream2"].Token)

		assert.Equal(t, "downstream1", config.CurrentContext)
	})
	t.Run("no cluster specified", func(t *testing.T) {
		var configMap *corev1.ConfigMap
		configMapClient := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
		configMapClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(obj *corev1.ConfigMap) (*corev1.ConfigMap, error) {
			require.Equal(t, namePrefix, obj.GenerateName)

			configMap = obj.DeepCopy()
			configMap.CreationTimestamp = metav1.Now()
			configMap.Name = names.SimpleNameGenerator.GenerateName(configMap.GenerateName)
			return configMap, nil
		}).Times(1)
		configMapClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(obj *corev1.ConfigMap) (*corev1.ConfigMap, error) {
			configMap = obj.DeepCopy()
			return configMap, nil
		}).Times(1)

		tokenManager := &fakeTokenManager{} // Subtest specific instance.

		store := &Store{
			mcmEnabled:          true,
			authorizer:          commonAuthorizer,
			nsCache:             nsCache,
			configMapClient:     configMapClient,
			userCache:           userCache,
			tokenCache:          tokenCache,
			clusterCache:        clusterCache,
			tokenMgr:            tokenManager,
			getCACert:           func() string { return "" },
			getDefaultTTL:       getDefaultTTL,
			getServerURL:        getServerURL,
			shouldGenerateToken: shouldGenerateToken,
		}

		ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: userID,
			Extra: map[string][]string{
				common.ExtraRequestTokenID: {authTokenID},
			},
		})
		kubeconfig := &ext.Kubeconfig{
			Spec: ext.KubeconfigSpec{
				Description: "Test Kubeconfig",
			},
		}
		var createValidationCalled bool
		createValidation := func(ctx context.Context, obj runtime.Object) error {
			createValidationCalled = true
			return nil
		}

		obj, err := store.Create(ctx, kubeconfig, createValidation, options)
		require.NoError(t, err)
		assert.NotNil(t, obj)
		assert.IsType(t, &ext.Kubeconfig{}, obj)

		assert.True(t, createValidationCalled)

		created := obj.(*ext.Kubeconfig)
		assert.NotEmpty(t, created.Name)
		assert.Empty(t, created.Namespace) // Kubeconfig is a cluster scoped resource.
		assert.Equal(t, defaultTTLSeconds, created.Spec.TTL)
		assert.Equal(t, kubeconfig.Spec.Description, created.Spec.Description)
		assert.Empty(t, kubeconfig.Spec.CurrentContext)
		assert.Len(t, created.Spec.Clusters, 0)

		require.NotNil(t, configMap)
		assert.Equal(t, created.Name, configMap.Name) // Check against the created Kubeconfig instance.
		assert.Equal(t, namespace, configMap.Namespace)
		require.NotNil(t, configMap.Labels)
		assert.Equal(t, userID, configMap.Labels[UserIDLabel])
		assert.Equal(t, KindLabelValue, configMap.Labels[KindLabel])
		require.NotNil(t, configMap.Annotations)
		assert.NotEmpty(t, configMap.Annotations[UIDAnnotation])
		require.NotNil(t, configMap.Data)
		assert.Equal(t, strconv.FormatInt(defaultTTLSeconds, 10), configMap.Data[TTLField])
		assert.Equal(t, kubeconfig.Spec.Description, configMap.Data[DescriptionField])
		assert.Equal(t, created.Spec.CurrentContext, configMap.Data[CurrentContextField]) // Check against the created Kubeconfig instance.
		assert.Empty(t, configMap.Data[ClustersField])

		require.NotNil(t, created.Status)
		assert.NotEmpty(t, created.Status.Value)
		assert.Equal(t, StatusSummaryComplete, created.Status.Summary)
		require.Len(t, created.Status.Conditions, 1)
		require.Len(t, created.Status.Tokens, 1)

		config, err := clientcmd.Load([]byte(created.Status.Value))
		require.NoError(t, err)
		require.Len(t, config.Clusters, 1)
		assert.Equal(t, serverURL, config.Clusters["rancher"].Server)
		assert.Empty(t, config.Clusters["rancher"].CertificateAuthorityData)

		require.Len(t, config.Contexts, 1)
		assert.Equal(t, "rancher", config.Contexts["rancher"].Cluster)
		assert.Equal(t, "rancher", config.Contexts["rancher"].AuthInfo)

		require.Len(t, config.AuthInfos, 1)
		require.Len(t, tokenManager.tokens, 1)
		assert.Equal(t, tokenManager.tokens[0], config.AuthInfos["rancher"].Token)
		require.Len(t, tokenManager.clusterTokens, 0)

		assert.Equal(t, "rancher", config.CurrentContext)
	})
	t.Run("dry run", func(t *testing.T) {
		tokenManager := &fakeTokenManager{} // Subtest specific instance.

		store := &Store{
			mcmEnabled:          true,
			authorizer:          commonAuthorizer,
			userCache:           userCache,
			tokenCache:          tokenCache,
			clusterCache:        clusterCache,
			nodeCache:           nodeCache,
			tokenMgr:            tokenManager,
			getCACert:           func() string { return "" },
			getDefaultTTL:       getDefaultTTL,
			getServerURL:        getServerURL,
			shouldGenerateToken: shouldGenerateToken,
		}

		ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: adminID,
			Extra: map[string][]string{
				common.ExtraRequestTokenID: {authTokenID},
			},
		})
		kubeconfig := &ext.Kubeconfig{
			Spec: ext.KubeconfigSpec{
				Clusters:       []string{downstream1, downstream2},
				CurrentContext: downstream1,
				Description:    "Test Kubeconfig",
			},
		}
		var createValidationCalled bool
		createValidation := func(ctx context.Context, obj runtime.Object) error {
			createValidationCalled = true
			return nil
		}

		options := &metav1.CreateOptions{
			DryRun: []string{"All"},
		}

		obj, err := store.Create(ctx, kubeconfig, createValidation, options)
		require.NoError(t, err)
		assert.NotNil(t, obj)
		assert.IsType(t, &ext.Kubeconfig{}, obj)

		assert.True(t, createValidationCalled)

		created := obj.(*ext.Kubeconfig)
		assert.NotEmpty(t, created.Name)
		assert.Empty(t, created.Namespace) // Kubeconfig is a cluster scoped resource.
		assert.Equal(t, defaultTTLSeconds, created.Spec.TTL)
		assert.Equal(t, kubeconfig.Spec.Description, created.Spec.Description)
		assert.Equal(t, downstream1, kubeconfig.Spec.CurrentContext)
		require.Len(t, created.Spec.Clusters, 2)
		assert.Equal(t, downstream1, created.Spec.Clusters[0])
		assert.Equal(t, downstream2, created.Spec.Clusters[1])

		require.NotNil(t, created.Status)
		assert.NotEmpty(t, created.Status.Value)
		assert.Equal(t, StatusSummaryComplete, created.Status.Summary)
		require.Len(t, created.Status.Conditions, 0) // No tokens created.
		require.Len(t, created.Status.Tokens, 0)     // No tokens created.

		config, err := clientcmd.Load([]byte(created.Status.Value))
		require.NoError(t, err)
		require.Len(t, config.Clusters, 4)
		assert.Equal(t, serverURL, config.Clusters["rancher"].Server)
		assert.Equal(t, fmt.Sprintf("%s/k8s/clusters/%s", serverURL, downstream1), config.Clusters["downstream1"].Server)
		assert.Equal(t, fmt.Sprintf("%s/k8s/clusters/%s", serverURL, downstream2), config.Clusters["downstream2"].Server)
		assert.Equal(t, "https://172.20.0.3:6443", config.Clusters["downstream2-cp"].Server)

		require.Len(t, config.Contexts, 4)
		assert.Equal(t, defaultClusterName, config.Contexts[defaultClusterName].Cluster)
		assert.Equal(t, defaultClusterName, config.Contexts[defaultClusterName].AuthInfo)
		assert.Equal(t, "downstream1", config.Contexts["downstream1"].Cluster)
		assert.Equal(t, defaultClusterName, config.Contexts["downstream1"].AuthInfo)
		assert.Equal(t, "downstream2", config.Contexts["downstream2"].Cluster)
		assert.Equal(t, "downstream2", config.Contexts["downstream2"].AuthInfo)
		assert.Equal(t, "downstream2-cp", config.Contexts["downstream2-cp"].Cluster)
		assert.Equal(t, "downstream2", config.Contexts["downstream2-cp"].AuthInfo)

		require.Len(t, config.AuthInfos, 2)
		require.Len(t, tokenManager.tokens, 0)
		assert.Equal(t, "", config.AuthInfos[defaultClusterName].Token)
		require.Len(t, tokenManager.clusterTokens, 0)
		assert.Equal(t, "", config.AuthInfos["downstream2"].Token)

		assert.Equal(t, "downstream1", config.CurrentContext)
	})
	t.Run("all clusters specified", func(t *testing.T) {
		authorizer := authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
			switch a.GetResource() {
			case v3.ClusterResourceName:
				if a.GetName() == downstream1 {
					return authorizer.DecisionAllow, "", nil
				}
			case "*":
				return commonAuthorizer(ctx, a)
			default:
				require.Fail(t, "Unexpected sar request")
			}
			return authorizer.DecisionDeny, "", nil
		})

		var configMap *corev1.ConfigMap
		configMapClient := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
		configMapClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(obj *corev1.ConfigMap) (*corev1.ConfigMap, error) {
			require.Equal(t, namePrefix, obj.GenerateName)

			configMap = obj.DeepCopy()
			configMap.CreationTimestamp = metav1.Now()
			configMap.Name = names.SimpleNameGenerator.GenerateName(configMap.GenerateName)
			return configMap, nil
		}).Times(1)
		configMapClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(obj *corev1.ConfigMap) (*corev1.ConfigMap, error) {
			configMap = obj.DeepCopy()
			return configMap, nil
		}).Times(1)

		tokenManager := &fakeTokenManager{} // Subtest specific instance.

		store := &Store{
			mcmEnabled:          true,
			authorizer:          authorizer,
			nsCache:             nsCache,
			configMapClient:     configMapClient,
			userCache:           userCache,
			tokenCache:          tokenCache,
			clusterCache:        clusterCache,
			tokenMgr:            tokenManager,
			getCACert:           func() string { return "" },
			getDefaultTTL:       getDefaultTTL,
			getServerURL:        getServerURL,
			shouldGenerateToken: shouldGenerateToken,
		}

		ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: userID,
			Extra: map[string][]string{
				common.ExtraRequestTokenID: {authTokenID},
			},
		})
		kubeconfig := &ext.Kubeconfig{
			Spec: ext.KubeconfigSpec{
				Clusters:    []string{"*"},
				Description: "Test Kubeconfig",
			},
		}
		var createValidationCalled bool
		createValidation := func(ctx context.Context, obj runtime.Object) error {
			createValidationCalled = true
			return nil
		}

		obj, err := store.Create(ctx, kubeconfig, createValidation, options)
		require.NoError(t, err)
		assert.NotNil(t, obj)
		assert.IsType(t, &ext.Kubeconfig{}, obj)

		assert.True(t, createValidationCalled)

		created := obj.(*ext.Kubeconfig)
		assert.NotEmpty(t, created.Name)
		assert.Empty(t, created.Namespace) // Kubeconfig is a cluster scoped resource.
		assert.Equal(t, defaultTTLSeconds, created.Spec.TTL)
		assert.Equal(t, kubeconfig.Spec.Description, created.Spec.Description)
		assert.Empty(t, kubeconfig.Spec.CurrentContext)
		require.Len(t, created.Spec.Clusters, 1)
		assert.Equal(t, "*", created.Spec.Clusters[0])

		require.NotNil(t, configMap)
		assert.Equal(t, created.Name, configMap.Name) // Check against the created Kubeconfig instance.
		assert.Equal(t, namespace, configMap.Namespace)
		require.NotNil(t, configMap.Labels)
		assert.Equal(t, userID, configMap.Labels[UserIDLabel])
		assert.Equal(t, KindLabelValue, configMap.Labels[KindLabel])
		require.NotNil(t, configMap.Annotations)
		assert.NotEmpty(t, configMap.Annotations[UIDAnnotation])
		require.NotNil(t, configMap.Data)
		assert.Equal(t, strconv.FormatInt(defaultTTLSeconds, 10), configMap.Data[TTLField])
		assert.Equal(t, kubeconfig.Spec.Description, configMap.Data[DescriptionField])
		assert.Equal(t, created.Spec.CurrentContext, configMap.Data[CurrentContextField]) // Check against the created Kubeconfig instance.
		assert.Equal(t, "[\"*\"]", configMap.Data[ClustersField])

		require.NotNil(t, created.Status)
		assert.NotEmpty(t, created.Status.Value)
		assert.Equal(t, StatusSummaryComplete, created.Status.Summary)
		require.Len(t, created.Status.Conditions, 1)
		require.Len(t, created.Status.Tokens, 1)

		config, err := clientcmd.Load([]byte(created.Status.Value))
		require.NoError(t, err)
		require.Len(t, config.Clusters, 2)
		assert.Equal(t, serverURL, config.Clusters["rancher"].Server)
		assert.Equal(t, fmt.Sprintf("%s/k8s/clusters/%s", serverURL, downstream1), config.Clusters["downstream1"].Server)

		require.Len(t, config.Contexts, 2)
		assert.Equal(t, defaultClusterName, config.Contexts[defaultClusterName].Cluster)
		assert.Equal(t, defaultClusterName, config.Contexts[defaultClusterName].AuthInfo)
		assert.Equal(t, "downstream1", config.Contexts["downstream1"].Cluster)
		assert.Equal(t, defaultClusterName, config.Contexts["downstream1"].AuthInfo)

		require.Len(t, config.AuthInfos, 1)
		require.Len(t, tokenManager.tokens, 1)
		assert.Equal(t, tokenManager.tokens[0], config.AuthInfos[defaultClusterName].Token)
		require.Len(t, tokenManager.clusterTokens, 0)

		assert.Equal(t, "downstream1", config.CurrentContext)
	})
	t.Run("MCM disabled", func(t *testing.T) {
		authorizer := authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
			return authorizer.DecisionAllow, "", nil
		})

		var configMap *corev1.ConfigMap
		configMapClient := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
		configMapClient.EXPECT().Create(gomock.Any()).DoAndReturn(func(obj *corev1.ConfigMap) (*corev1.ConfigMap, error) {
			require.Equal(t, namePrefix, obj.GenerateName)

			configMap = obj.DeepCopy()
			configMap.CreationTimestamp = metav1.NewTime(time.Now())
			configMap.Name = names.SimpleNameGenerator.GenerateName(configMap.GenerateName)
			return configMap, nil
		}).Times(1)
		configMapClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(obj *corev1.ConfigMap) (*corev1.ConfigMap, error) {
			configMap = obj.DeepCopy()
			return configMap, nil
		}).Times(1)

		tokenManager := &fakeTokenManager{} // Subtest specific instance.

		store := &Store{
			authorizer:          authorizer,
			nsCache:             nsCache,
			configMapClient:     configMapClient,
			userCache:           userCache,
			tokenCache:          tokenCache,
			clusterCache:        clusterCache,
			tokenMgr:            tokenManager,
			getCACert:           func() string { return rancherCACert },
			getDefaultTTL:       getDefaultTTL,
			getServerURL:        getServerURL,
			shouldGenerateToken: shouldGenerateToken,
		}

		ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: userID,
			Extra: map[string][]string{
				common.ExtraRequestTokenID: {authTokenID},
			},
		})
		kubeconfig := &ext.Kubeconfig{
			Spec: ext.KubeconfigSpec{
				Clusters: []string{"local"},
			},
		}
		var createValidationCalled bool
		createValidation := func(ctx context.Context, obj runtime.Object) error {
			createValidationCalled = true
			return nil
		}

		obj, err := store.Create(ctx, kubeconfig, createValidation, options)
		require.NoError(t, err)
		assert.NotNil(t, obj)
		assert.IsType(t, &ext.Kubeconfig{}, obj)

		assert.True(t, createValidationCalled)

		created := obj.(*ext.Kubeconfig)
		assert.NotEmpty(t, created.Name)
		assert.Empty(t, created.Namespace) // Kubeconfig is a cluster scoped resource.
		assert.Equal(t, defaultTTLSeconds, created.Spec.TTL)
		assert.Equal(t, kubeconfig.Spec.Description, created.Spec.Description)
		assert.NotEmpty(t, created.Spec.CurrentContext)
		assert.Equal(t, kubeconfig.Spec.Clusters, created.Spec.Clusters)

		require.NotNil(t, configMap)
		assert.Equal(t, created.Name, configMap.Name) // Check against the created Kubeconfig instance.
		assert.Equal(t, namespace, configMap.Namespace)
		require.NotNil(t, configMap.Labels)
		assert.Equal(t, userID, configMap.Labels[UserIDLabel])
		assert.Equal(t, KindLabelValue, configMap.Labels[KindLabel])
		require.NotNil(t, configMap.Annotations)
		assert.NotEmpty(t, configMap.Annotations[UIDAnnotation])
		require.NotNil(t, configMap.Data)
		assert.Equal(t, strconv.FormatInt(defaultTTLSeconds, 10), configMap.Data[TTLField])
		assert.Equal(t, kubeconfig.Spec.Description, configMap.Data[DescriptionField])
		assert.Equal(t, created.Spec.CurrentContext, configMap.Data[CurrentContextField]) // Check against the created Kubeconfig instance.
		clustersValue, err := json.Marshal(kubeconfig.Spec.Clusters)
		require.NoError(t, err)
		assert.Equal(t, string(clustersValue), configMap.Data[ClustersField])

		require.NotNil(t, created.Status)
		assert.NotEmpty(t, created.Status.Value)
		assert.Equal(t, StatusSummaryComplete, created.Status.Summary)
		require.Len(t, created.Status.Conditions, 1)
		require.Len(t, created.Status.Tokens, 1)

		config, err := clientcmd.Load([]byte(created.Status.Value))
		require.NoError(t, err)
		require.Len(t, config.Clusters, 2)
		assert.Equal(t, serverURL, config.Clusters[defaultClusterName].Server)
		assert.Equal(t, rancherCACert, string(config.Clusters[defaultClusterName].CertificateAuthorityData))
		assert.Equal(t, fmt.Sprintf("%s/k8s/clusters/%s", serverURL, "local"), config.Clusters["local"].Server)

		require.Len(t, config.Contexts, 2)
		assert.Equal(t, defaultClusterName, config.Contexts[defaultClusterName].Cluster)
		assert.Equal(t, defaultClusterName, config.Contexts[defaultClusterName].AuthInfo)

		require.Len(t, config.AuthInfos, 1)
		require.Len(t, tokenManager.tokens, 1)
		assert.Equal(t, tokenManager.tokens[0], config.AuthInfos[defaultClusterName].Token)

		assert.Equal(t, "local", config.CurrentContext)
	})
	t.Run("only a rancher user can create kubeconfig", func(t *testing.T) {
		store := &Store{
			authorizer: commonAuthorizer,
			userCache:  userCache,
			tokenCache: tokenCache,
			tokenMgr:   tokenManager,
		}

		ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: adminSA,
		})
		kubeconfig := &ext.Kubeconfig{
			Spec: ext.KubeconfigSpec{
				Clusters:       []string{downstream1, downstream2},
				CurrentContext: downstream1,
			},
		}

		obj, err := store.Create(ctx, kubeconfig, nil, options)
		require.Error(t, err)
		assert.Nil(t, obj)
		assert.True(t, apierrors.IsForbidden(err))
		assert.Contains(t, err.Error(), "user "+adminSA+" is not a Rancher user")
	})
	t.Run("missing request token", func(t *testing.T) {
		store := &Store{
			authorizer: commonAuthorizer,
			userCache:  userCache,
			tokenCache: tokenCache,
			tokenMgr:   tokenManager,
		}

		ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: userID,
		})
		kubeconfig := &ext.Kubeconfig{
			Spec: ext.KubeconfigSpec{
				Clusters:       []string{downstream1, downstream2},
				CurrentContext: downstream1,
			},
		}

		obj, err := store.Create(ctx, kubeconfig, nil, options)
		require.Error(t, err)
		assert.Nil(t, obj)
		assert.True(t, apierrors.IsForbidden(err))
		assert.Contains(t, err.Error(), "missing request token ID")
	})
	t.Run("request token doesn't exist", func(t *testing.T) {
		store := &Store{
			authorizer: commonAuthorizer,
			userCache:  userCache,
			tokenCache: tokenCache,
			tokenMgr:   tokenManager,
		}

		ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Extra: map[string][]string{
				common.ExtraRequestTokenID: {"non-existent"},
			},
			Name: userID,
		})
		kubeconfig := &ext.Kubeconfig{
			Spec: ext.KubeconfigSpec{
				Clusters:       []string{downstream1, downstream2},
				CurrentContext: downstream1,
			},
		}

		obj, err := store.Create(ctx, kubeconfig, nil, options)
		require.Error(t, err)
		assert.Nil(t, obj)
		assert.True(t, apierrors.IsForbidden(err))
		assert.Contains(t, err.Error(), "\"non-existent\" not found")
	})
	t.Run("negative ttl", func(t *testing.T) {
		store := &Store{
			authorizer:    commonAuthorizer,
			userCache:     userCache,
			tokenCache:    tokenCache,
			tokenMgr:      tokenManager,
			getDefaultTTL: getDefaultTTL,
		}

		ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Extra: map[string][]string{
				common.ExtraRequestTokenID: {authTokenID},
			},
			Name: userID,
		})
		kubeconfig := &ext.Kubeconfig{
			Spec: ext.KubeconfigSpec{
				Clusters:       []string{downstream1, downstream2},
				CurrentContext: downstream1,
				TTL:            -1,
			},
		}

		obj, err := store.Create(ctx, kubeconfig, nil, options)
		require.Error(t, err)
		assert.Nil(t, obj)
		assert.True(t, apierrors.IsBadRequest(err))
		assert.Contains(t, err.Error(), "spec.ttl can't be negative")
	})
	t.Run("ttl exceeds the max", func(t *testing.T) {
		store := &Store{
			authorizer:    commonAuthorizer,
			userCache:     userCache,
			tokenCache:    tokenCache,
			tokenMgr:      tokenManager,
			getDefaultTTL: getDefaultTTL,
		}

		ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Extra: map[string][]string{
				common.ExtraRequestTokenID: {authTokenID},
			},
			Name: userID,
		})
		kubeconfig := &ext.Kubeconfig{
			Spec: ext.KubeconfigSpec{
				Clusters:       []string{downstream1, downstream2},
				CurrentContext: downstream1,
				TTL:            defaultTTLSeconds + 1,
			},
		}

		obj, err := store.Create(ctx, kubeconfig, nil, options)
		require.Error(t, err)
		assert.Nil(t, obj)
		assert.True(t, apierrors.IsBadRequest(err))
		assert.Contains(t, err.Error(), "exceeds max ttl")
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
	return key, string(pem), nil
}

func TestWatcherStop(t *testing.T) {
	t.Parallel()

	t.Run("safe to call Stop more than once", func(t *testing.T) {
		w := &watcher{
			ch: make(chan watch.Event, 1),
		}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.Stop()
		}()

		w.Stop()
		wg.Wait()
	})
}

func TestWatcherAdd(t *testing.T) {
	t.Parallel()

	w := &watcher{
		ch: make(chan watch.Event, 3),
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.add(watch.Event{
			Type: watch.Added,
			Object: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "obj1",
					Namespace: namespace,
				},
			},
		})
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		w.add(watch.Event{
			Type: watch.Added,
			Object: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "obj2",
					Namespace: namespace,
				},
			},
		})
	}()

	wg.Wait()
	w.Stop()

	var added []string
	for e := range w.ResultChan() {
		objMeta, err := meta.Accessor(e.Object)
		require.NoError(t, err)
		added = append(added, objMeta.GetName())
	}
	assert.Len(t, added, 2)
	assert.Contains(t, added, "obj1")
	assert.Contains(t, added, "obj2")

	assert.False(t, w.add(watch.Event{}))
	_, ok := <-w.ResultChan()
	assert.False(t, ok)
}

func TestStoreGet(t *testing.T) {
	t.Parallel()

	userID := "u-w7drc"
	authTokenID := "token-nh98r"
	kubeconfigID := "kubeconfig-49d5p"

	ctrl := gomock.NewController(t)
	userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	userCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.User, error) {
		switch name {
		case userID:
			return &v3.User{}, nil
		case "error":
			return nil, fmt.Errorf("some error")
		default:
			return nil, apierrors.NewNotFound(gvr.GroupResource(), name)
		}
	}).AnyTimes()

	fieldMapJSON, err := fieldpath.NewSet(
		pathCMData,
		pathCMClustersField,
		pathCMCurrentContextField,
		pathCMDescriptionField,
		pathCMTTLField,
		pathCMStatusConditionsField,
		pathCMStatusSummaryField,
		pathCMStatusTokensField,
		pathCMLabelKind,
		fieldpath.MakePathOrDie("metadata"),
		fieldpath.MakePathOrDie("type"),
	).ToJSON()
	assert.Nil(t, err)

	now := metav1.Now()

	defaultTTL := int64(43200)
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeconfigID,
			Namespace: namespace,
			Labels: map[string]string{
				UserIDLabel: userID,
				KindLabel:   KindLabelValue,
			},
			Annotations: map[string]string{
				UIDAnnotation: string(uuid.NewUUID()),
				"custom":      "annotation",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "management.cattle.io/v3",
					Kind:       "Token",
					Name:       "kubeconfig-u-w7drcgc66",
					UID:        uuid.NewUUID(),
				},
			},
			ManagedFields: []metav1.ManagedFieldsEntry{
				metav1.ManagedFieldsEntry{
					Manager:    "kubeconfig",
					Operation:  "something",
					Time:       &now,
					FieldsType: "v1",
					FieldsV1: &metav1.FieldsV1{
						Raw: fieldMapJSON,
					},
				},
			},
		},
		Data: map[string]string{
			TTLField:            strconv.FormatInt(defaultTTL, 10),
			DescriptionField:    "test",
			CurrentContextField: "c-m-tbgzfbgf",
			ClustersField:       `["c-m-tbgzfbgf","c-m-bxn2p7w6"]`,
		},
	}

	fieldKCJSON, err := fieldpath.NewSet(
		pathKConfigClustersField,
		pathKConfigCurrentContextField,
		pathKConfigDescriptionField,
		pathKConfigTTLField,
		fieldpath.MakePathOrDie("metadata"),
		fieldpath.MakePathOrDie("type"),
	).ToJSON()
	assert.Nil(t, err)

	kcFields := []metav1.ManagedFieldsEntry{
		metav1.ManagedFieldsEntry{
			Manager:    "kubeconfig",
			Operation:  "something",
			Time:       &now,
			FieldsType: "v1",
			FieldsV1: &metav1.FieldsV1{
				Raw: fieldKCJSON,
			},
		},
	}

	configMapCache := fake.NewMockCacheInterface[*corev1.ConfigMap](ctrl)
	configMapCache.EXPECT().Get(namespace, gomock.Any()).DoAndReturn(func(namespace, name string) (*corev1.ConfigMap, error) {
		switch name {
		case kubeconfigID:
			return configMap, nil
		default:
			return nil, apierrors.NewNotFound(gvr.GroupResource(), name)
		}
	}).AnyTimes()

	tokenManager := &fakeTokenManager{}

	ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
		Name: userID,
		Extra: map[string][]string{
			common.ExtraRequestTokenID: {authTokenID},
		},
	})

	t.Run("admin gets kubeconfig", func(t *testing.T) {
		store := &Store{
			authorizer:     commonAuthorizer,
			configMapCache: configMapCache,
			userCache:      userCache,
			tokenMgr:       tokenManager,
		}

		ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: adminID,
			Extra: map[string][]string{
				common.ExtraRequestTokenID: {"token-8wrqh"},
			},
		})

		obj, err := store.Get(ctx, kubeconfigID, &metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, obj)
		require.IsType(t, &ext.Kubeconfig{}, obj)

		kubeconfig := obj.(*ext.Kubeconfig)
		assert.Equal(t, configMap.Name, kubeconfig.Name)
		assert.Empty(t, kubeconfig.Namespace)

		require.NotNil(t, kubeconfig.Labels)
		assert.Equal(t, userID, kubeconfig.Labels[UserIDLabel])

		require.NotNil(t, kubeconfig.Annotations)
		assert.Empty(t, kubeconfig.Annotations[UIDAnnotation])
		assert.Equal(t, "annotation", kubeconfig.Annotations["custom"])

		assert.Equal(t, defaultTTL, kubeconfig.Spec.TTL)
		assert.Equal(t, configMap.Data[DescriptionField], kubeconfig.Spec.Description)
		assert.Equal(t, configMap.Data[CurrentContextField], kubeconfig.Spec.CurrentContext)

		clustersValue, err := json.Marshal(kubeconfig.Spec.Clusters)
		require.NoError(t, err)
		assert.Equal(t, string(clustersValue), configMap.Data[ClustersField])

		assert.Equal(t, kubeconfig.ObjectMeta.ManagedFields, kcFields)
	})

	t.Run("user gets kubeconfig", func(t *testing.T) {
		store := &Store{
			authorizer:     commonAuthorizer,
			configMapCache: configMapCache,
			userCache:      userCache,
			tokenMgr:       tokenManager,
		}

		obj, err := store.Get(ctx, kubeconfigID, &metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, obj)
		require.IsType(t, &ext.Kubeconfig{}, obj)

		kubeconfig := obj.(*ext.Kubeconfig)
		assert.Equal(t, kubeconfigID, kubeconfig.Name)
	})

	t.Run("user can't get other user's kubeconfig", func(t *testing.T) {
		oldConfigMap := configMap.DeepCopy()
		defer func() {
			configMap = oldConfigMap.DeepCopy()
		}()
		configMap.Labels[UserIDLabel] = "other-user"

		store := &Store{
			authorizer:     commonAuthorizer,
			configMapCache: configMapCache,
			userCache:      userCache,
			tokenMgr:       tokenManager,
		}

		obj, err := store.Get(ctx, kubeconfigID, &metav1.GetOptions{})
		require.Error(t, err)
		assert.True(t, apierrors.IsNotFound(err))
		require.Nil(t, obj)
	})

	t.Run("configmap client is used if options are set", func(t *testing.T) {
		configMapClient := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
		configMapClient.EXPECT().Get(namespace, kubeconfigID, gomock.Any()).DoAndReturn(func(namespace, name string, options metav1.GetOptions) (*corev1.ConfigMap, error) {
			assert.Equal(t, "1", options.ResourceVersion)
			return configMap, nil
		}).AnyTimes()

		store := &Store{
			authorizer:      commonAuthorizer,
			configMapClient: configMapClient,
			userCache:       userCache,
			tokenMgr:        tokenManager,
		}

		obj, err := store.Get(ctx, kubeconfigID, &metav1.GetOptions{ResourceVersion: "1"})
		require.NoError(t, err)
		require.NotNil(t, obj)
		require.IsType(t, &ext.Kubeconfig{}, obj)

		kubeconfig := obj.(*ext.Kubeconfig)
		assert.Equal(t, kubeconfigID, kubeconfig.Name)
	})

	t.Run("not found error points to correct resource", func(t *testing.T) {
		store := &Store{
			authorizer:     commonAuthorizer,
			configMapCache: configMapCache,
			userCache:      userCache,
			tokenMgr:       tokenManager,
		}

		obj, err := store.Get(ctx, "non-existing", &metav1.GetOptions{})
		require.Error(t, err)
		require.Nil(t, obj)
		assert.True(t, apierrors.IsNotFound(err))

		statusErr, ok := err.(*apierrors.StatusError)
		require.True(t, ok)
		assert.Equal(t, gvr.Group, statusErr.Status().Details.Group)
		assert.Equal(t, ext.KubeconfigResourceName, statusErr.Status().Details.Kind)
		assert.Equal(t, "non-existing", statusErr.Status().Details.Name)
	})
}

func TestStoreList(t *testing.T) {
	t.Parallel()

	userID1 := "u-w7drc"
	userID2 := "u-2p7w6"
	kubeconfigID1 := "kubeconfig-49d5p"
	kubeconfigID2 := "kubeconfig-7kp6c"

	ctrl := gomock.NewController(t)
	userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	userCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.User, error) {
		switch name {
		case userID1, userID2:
			return &v3.User{ObjectMeta: metav1.ObjectMeta{Name: name}}, nil
		case "error":
			return nil, fmt.Errorf("some error")
		default:
			return nil, apierrors.NewNotFound(gvr.GroupResource(), name)
		}
	}).AnyTimes()

	defaultTTL := int64(43200)
	configMap1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeconfigID1,
			Namespace: namespace,
			Labels: map[string]string{
				UserIDLabel: userID1,
				KindLabel:   KindLabelValue,
			},
			Annotations: map[string]string{
				UIDAnnotation: string(uuid.NewUUID()),
				"custom":      "annotation",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "management.cattle.io/v3",
					Kind:       "Token",
					Name:       "kubeconfig-u-w7drcgc66",
					UID:        uuid.NewUUID(),
				},
			},
		},
		Data: map[string]string{
			TTLField:            strconv.FormatInt(defaultTTL, 10),
			DescriptionField:    "test1",
			CurrentContextField: "c-m-tbgzfbgf",
			ClustersField:       `["c-m-tbgzfbgf","c-m-bxn2p7w6"]`,
		},
	}
	configMap2 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeconfigID2,
			Namespace: namespace,
			Labels: map[string]string{
				UserIDLabel: userID2,
				KindLabel:   KindLabelValue,
			},
			Annotations: map[string]string{
				UIDAnnotation: string(uuid.NewUUID()),
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "management.cattle.io/v3",
					Kind:       "Token",
					Name:       "kubeconfig-u-2p7w6gc66",
					UID:        uuid.NewUUID(),
				},
			},
		},
		Data: map[string]string{
			TTLField:            strconv.FormatInt(defaultTTL, 10),
			DescriptionField:    "test2",
			CurrentContextField: "c-m-tbgzfbgf",
			ClustersField:       `["c-m-tbgzfbgf"]`,
		},
	}

	configMapClient := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
	configMapClient.EXPECT().List(namespace, gomock.Any()).DoAndReturn(func(namespace string, opts metav1.ListOptions) (*corev1.ConfigMapList, error) {
		labelSet, err := labels.ConvertSelectorToLabelsMap(opts.LabelSelector)
		require.NoError(t, err)

		var items []corev1.ConfigMap
		switch labelSet[UserIDLabel] {
		case userID1:
			items = []corev1.ConfigMap{*configMap1}
		case userID2:
			items = []corev1.ConfigMap{*configMap2}
		default:
			items = []corev1.ConfigMap{*configMap1, *configMap2}
		}

		return &corev1.ConfigMapList{
			ListMeta: metav1.ListMeta{
				ResourceVersion: "1",
			},
			Items: items,
		}, nil
	}).AnyTimes()

	tokenManager := &fakeTokenManager{}

	t.Run("admin lists kubeconfigs", func(t *testing.T) {
		store := &Store{
			authorizer:      commonAuthorizer,
			configMapClient: configMapClient,
			userCache:       userCache,
			tokenMgr:        tokenManager,
		}

		ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: adminID,
		})

		obj, err := store.List(ctx, &metainternalversion.ListOptions{})
		require.NoError(t, err)
		require.NotNil(t, obj)
		require.IsType(t, &ext.KubeconfigList{}, obj)

		list := obj.(*ext.KubeconfigList)
		assert.Len(t, list.Items, 2)

		assert.Equal(t, configMap1.Name, list.Items[0].Name)
		assert.Equal(t, configMap2.Name, list.Items[1].Name)

		assert.NotEmpty(t, list.ResourceVersion)
	})

	t.Run("user lists kubeconfigs", func(t *testing.T) {
		store := &Store{
			authorizer:      commonAuthorizer,
			configMapClient: configMapClient,
			userCache:       userCache,
			tokenMgr:        tokenManager,
		}

		ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: userID1,
		})

		obj, err := store.List(ctx, &metainternalversion.ListOptions{})
		require.NoError(t, err)
		require.NotNil(t, obj)
		require.IsType(t, &ext.KubeconfigList{}, obj)

		list := obj.(*ext.KubeconfigList)
		assert.Len(t, list.Items, 1)

		assert.Equal(t, configMap1.Name, list.Items[0].Name)

		assert.NotEmpty(t, list.ResourceVersion)
	})
}

func TestStoreWatch(t *testing.T) {
	t.Parallel()

	userID := "u-w7drc"
	kubeconfigID := "kubeconfig-49d5p"

	ctrl := gomock.NewController(t)
	userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	userCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.User, error) {
		switch name {
		case userID:
			return &v3.User{ObjectMeta: metav1.ObjectMeta{Name: name}}, nil
		case "error":
			return nil, fmt.Errorf("some error")
		default:
			return nil, apierrors.NewNotFound(gvr.GroupResource(), name)
		}
	}).AnyTimes()

	defaultTTL := int64(43200)
	configMap1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeconfigID,
			Namespace: namespace,
			Labels: map[string]string{
				UserIDLabel: userID,
				KindLabel:   KindLabelValue,
			},
			Annotations: map[string]string{
				UIDAnnotation: string(uuid.NewUUID()),
				"custom":      "annotation",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "management.cattle.io/v3",
					Kind:       "Token",
					Name:       "kubeconfig-u-w7drcgc66",
					UID:        uuid.NewUUID(),
				},
			},
		},
		Data: map[string]string{
			TTLField:            strconv.FormatInt(defaultTTL, 10),
			DescriptionField:    "test1",
			CurrentContextField: "c-m-tbgzfbgf",
			ClustersField:       `["c-m-tbgzfbgf","c-m-bxn2p7w6"]`,
		},
	}

	tokenManager := &fakeTokenManager{}

	t.Run("admin watches kubeconfigs", func(t *testing.T) {
		configMapWatcher := &watcher{
			ch: make(chan watch.Event),
		}
		defer configMapWatcher.Stop()

		configMapClient := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
		configMapClient.EXPECT().Watch(namespace, gomock.Any()).DoAndReturn(func(namespace string, options metav1.ListOptions) (watch.Interface, error) {
			labelSet, err := labels.ConvertSelectorToLabelsMap(options.LabelSelector)
			require.NoError(t, err)
			assert.Equal(t, KindLabelValue, labelSet[KindLabel])
			assert.NotContains(t, labelSet, UserIDLabel)

			return configMapWatcher, nil
		}).AnyTimes()

		store := &Store{
			authorizer:      commonAuthorizer,
			configMapClient: configMapClient,
			userCache:       userCache,
			tokenMgr:        tokenManager,
		}

		ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: adminID,
		})

		watcher, err := store.Watch(ctx, &metainternalversion.ListOptions{})
		require.NoError(t, err)
		defer watcher.Stop()

		configMapWatcher.add(watch.Event{
			Type:   watch.Added,
			Object: configMap1,
		})

		event := <-watcher.ResultChan()
		require.Equal(t, watch.Added, event.Type)
		k, ok := event.Object.(*ext.Kubeconfig)
		require.True(t, ok)
		assert.Equal(t, configMap1.Name, k.Name)

		configMapWatcher.add(watch.Event{
			Type: watch.Bookmark,
			Object: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "1",
				},
			},
		})
		event = <-watcher.ResultChan()
		require.Equal(t, watch.Bookmark, event.Type)
		k, ok = event.Object.(*ext.Kubeconfig)
		require.True(t, ok)
		assert.Equal(t, "1", k.ResourceVersion)

		configMapWatcher.add(watch.Event{
			Type:   watch.Modified,
			Object: configMap1,
		})
		event = <-watcher.ResultChan()
		require.Equal(t, watch.Modified, event.Type)
		k, ok = event.Object.(*ext.Kubeconfig)
		require.True(t, ok)
		assert.Equal(t, configMap1.Name, k.Name)

		configMapWatcher.add(watch.Event{
			Type:   watch.Deleted,
			Object: configMap1,
		})
		event = <-watcher.ResultChan()
		require.Equal(t, watch.Deleted, event.Type)
		k, ok = event.Object.(*ext.Kubeconfig)
		require.True(t, ok)
		assert.Equal(t, configMap1.Name, k.Name)

		statusIn := &metav1.Status{
			Status:  metav1.StatusFailure,
			Message: "The resourceVersion for the provided watch is too old.",
			Reason:  metav1.StatusReasonExpired,
			Code:    http.StatusGone,
		}

		configMapWatcher.add(watch.Event{
			Type:   watch.Error,
			Object: statusIn,
		})

		event = <-watcher.ResultChan()
		require.Equal(t, watch.Error, event.Type)
		statusOut, ok := event.Object.(*metav1.Status)
		require.True(t, ok)
		require.NotNil(t, statusOut)
		assert.Equal(t, statusIn.Status, statusOut.Status)
		assert.Equal(t, statusIn.Message, statusOut.Message)
		assert.Equal(t, statusIn.Reason, statusOut.Reason)
		assert.Equal(t, statusIn.Code, statusOut.Code)

		// Not a Status error.
		configMapWatcher.add(watch.Event{
			Type: watch.Error,
		})

		event = <-watcher.ResultChan()
		require.Equal(t, watch.Error, event.Type)
		require.Nil(t, event.Object)

		// Add another bookmark after an error event.
		configMapWatcher.add(watch.Event{
			Type: watch.Bookmark,
			Object: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "2",
				},
			},
		})
		event = <-watcher.ResultChan()
		require.Equal(t, watch.Bookmark, event.Type)
		k, ok = event.Object.(*ext.Kubeconfig)
		require.True(t, ok)
		assert.Equal(t, "2", k.ResourceVersion)
	})

	t.Run("user watches kubeconfigs", func(t *testing.T) {
		configMapWatcher := &watcher{
			ch: make(chan watch.Event),
		}
		defer configMapWatcher.Stop()

		configMapClient := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
		configMapClient.EXPECT().Watch(namespace, gomock.Any()).DoAndReturn(func(namespace string, options metav1.ListOptions) (watch.Interface, error) {
			labelSet, err := labels.ConvertSelectorToLabelsMap(options.LabelSelector)
			require.NoError(t, err)
			assert.Equal(t, KindLabelValue, labelSet[KindLabel])
			assert.Equal(t, userID, labelSet[UserIDLabel])

			return configMapWatcher, nil
		}).AnyTimes()

		store := &Store{
			authorizer:      commonAuthorizer,
			configMapClient: configMapClient,
			userCache:       userCache,
			tokenMgr:        tokenManager,
		}

		ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: userID,
		})

		watcher, err := store.Watch(ctx, &metainternalversion.ListOptions{})
		require.NoError(t, err)
		defer watcher.Stop()

		configMapWatcher.add(watch.Event{
			Type:   watch.Added,
			Object: configMap1,
		})

		event := <-watcher.ResultChan()
		require.Equal(t, watch.Added, event.Type)
		k, ok := event.Object.(*ext.Kubeconfig)
		require.True(t, ok)
		assert.Equal(t, configMap1.Name, k.Name)
	})
}

type fakeUpdatedObjectInfo struct {
	obj runtime.Object
	err error
}

func (i *fakeUpdatedObjectInfo) Preconditions() *metav1.Preconditions {
	return nil
}

func (i *fakeUpdatedObjectInfo) UpdatedObject(ctx context.Context, oldObj runtime.Object) (newObj runtime.Object, err error) {
	return i.obj, i.err
}

func TestStoreUpdate(t *testing.T) {
	t.Parallel()

	userID := "u-w7drc"
	kubeconfigID := "kubeconfig-49d5p"

	ctrl := gomock.NewController(t)
	userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	userCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.User, error) {
		switch name {
		case userID:
			return &v3.User{}, nil
		case "error":
			return nil, fmt.Errorf("some error")
		default:
			return nil, apierrors.NewNotFound(gvr.GroupResource(), name)
		}
	}).AnyTimes()

	oldConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeconfigID,
			Namespace: namespace,
			Labels: map[string]string{
				UserIDLabel: userID,
				KindLabel:   KindLabelValue,
			},
			Annotations: map[string]string{
				UIDAnnotation: string(uuid.NewUUID()),
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "management.cattle.io/v3",
					Kind:       "Token",
					Name:       "kubeconfig-u-w7drcgc66",
					UID:        uuid.NewUUID(),
				},
			},
		},
		Data: map[string]string{
			TTLField:            "43200",
			DescriptionField:    "test",
			CurrentContextField: "c-m-tbgzfbgf",
			ClustersField:       `["c-m-tbgzfbgf","c-m-bxn2p7w6"]`,
		},
	}

	tokenManager := &fakeTokenManager{}

	t.Run("admin updates a kubeconfig", func(t *testing.T) {
		configMapClient := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
		configMapClient.EXPECT().Get(namespace, kubeconfigID, gomock.Any()).DoAndReturn(func(namespace, name string, options metav1.GetOptions) (*corev1.ConfigMap, error) {
			return oldConfigMap.DeepCopy(), nil
		})
		configMapClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
			assert.Equal(t, "bar", configMap.Labels["foo"])
			assert.Equal(t, "bar", configMap.Annotations["foo"])
			assert.Equal(t, "updated", configMap.Data[DescriptionField])
			return configMap.DeepCopy(), nil
		})

		store := &Store{
			authorizer:      commonAuthorizer,
			configMapClient: configMapClient,
			userCache:       userCache,
			tokenMgr:        tokenManager,
		}

		createValidation := func(ctx context.Context, obj runtime.Object) error {
			assert.Fail(t, "createValidation should not be called")
			return nil
		}

		var updateValidationCalled bool
		updateValidation := func(ctx context.Context, obj, old runtime.Object) error {
			updateValidationCalled = true
			require.NotNil(t, obj)
			require.IsType(t, &ext.Kubeconfig{}, obj)
			require.NotNil(t, old)
			require.IsType(t, &ext.Kubeconfig{}, old)
			return nil
		}

		ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: adminID,
		})

		oldKubeconfig, err := store.fromConfigMap(oldConfigMap)
		assert.NoError(t, err)

		update := oldKubeconfig.DeepCopy()
		update.Spec.Description = "updated"
		update.Annotations["foo"] = "bar"
		update.Labels["foo"] = "bar"
		update.Finalizers = append(update.Finalizers, "foo/bar")
		objInfo := &fakeUpdatedObjectInfo{obj: update}

		options := &metav1.UpdateOptions{}

		obj, isCreated, err := store.Update(ctx, kubeconfigID, objInfo, createValidation, updateValidation, false, options)
		require.NoError(t, err)
		assert.NotNil(t, obj)
		assert.IsType(t, &ext.Kubeconfig{}, obj)
		assert.False(t, isCreated)

		assert.True(t, updateValidationCalled)

		newKubeconfig, ok := obj.(*ext.Kubeconfig)
		assert.True(t, ok)
		assert.Equal(t, "bar", newKubeconfig.Labels["foo"])
		assert.Equal(t, "bar", newKubeconfig.Annotations["foo"])
		assert.Equal(t, "updated", newKubeconfig.Spec.Description)
		require.Len(t, newKubeconfig.Finalizers, 1)
		assert.Equal(t, "foo/bar", newKubeconfig.Finalizers[0])
		require.Len(t, newKubeconfig.OwnerReferences, 1)
		assert.Equal(t, oldConfigMap.OwnerReferences[0].UID, newKubeconfig.OwnerReferences[0].UID)
	})
	t.Run("user updates their kubeconfig", func(t *testing.T) {
		configMapClient := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
		configMapClient.EXPECT().Get(namespace, kubeconfigID, gomock.Any()).DoAndReturn(func(namespace, name string, options metav1.GetOptions) (*corev1.ConfigMap, error) {
			return oldConfigMap.DeepCopy(), nil
		})
		configMapClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
			assert.Equal(t, "bar", configMap.Labels["foo"])
			assert.Equal(t, "bar", configMap.Annotations["foo"])
			assert.Equal(t, "updated", configMap.Data[DescriptionField])
			return configMap.DeepCopy(), nil
		})

		store := &Store{
			authorizer:      commonAuthorizer,
			configMapClient: configMapClient,
			userCache:       userCache,
			tokenMgr:        tokenManager,
		}

		createValidation := func(ctx context.Context, obj runtime.Object) error {
			assert.Fail(t, "createValidation should not be called")
			return nil
		}

		var updateValidationCalled bool
		updateValidation := func(ctx context.Context, obj, old runtime.Object) error {
			updateValidationCalled = true
			require.NotNil(t, obj)
			require.IsType(t, &ext.Kubeconfig{}, obj)
			require.NotNil(t, old)
			require.IsType(t, &ext.Kubeconfig{}, old)
			return nil
		}

		ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: userID, // Kubeconfig owner.
		})

		oldKubeconfig, err := store.fromConfigMap(oldConfigMap)
		assert.NoError(t, err)

		update := oldKubeconfig.DeepCopy()
		update.Spec.Description = "updated"
		update.Annotations["foo"] = "bar"
		update.Labels["foo"] = "bar"
		update.Finalizers = append(update.Finalizers, "foo/bar")
		objInfo := &fakeUpdatedObjectInfo{obj: update}

		options := &metav1.UpdateOptions{}

		obj, isCreated, err := store.Update(ctx, kubeconfigID, objInfo, createValidation, updateValidation, false, options)
		require.NoError(t, err)
		assert.NotNil(t, obj)
		assert.IsType(t, &ext.Kubeconfig{}, obj)
		assert.False(t, isCreated)

		assert.True(t, updateValidationCalled)

		newKubeconfig, ok := obj.(*ext.Kubeconfig)
		assert.True(t, ok)
		assert.Equal(t, "bar", newKubeconfig.Labels["foo"])
		assert.Equal(t, "bar", newKubeconfig.Annotations["foo"])
		assert.Equal(t, "updated", newKubeconfig.Spec.Description)
		require.Len(t, newKubeconfig.Finalizers, 1)
		assert.Equal(t, "foo/bar", newKubeconfig.Finalizers[0])
		require.Len(t, newKubeconfig.OwnerReferences, 1)
		assert.Equal(t, oldConfigMap.OwnerReferences[0].UID, newKubeconfig.OwnerReferences[0].UID)
	})

	t.Run("user can't update other user's kubeconfig", func(t *testing.T) {
		configMapClient := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
		configMapClient.EXPECT().Get(namespace, kubeconfigID, gomock.Any()).DoAndReturn(func(namespace, name string, options metav1.GetOptions) (*corev1.ConfigMap, error) {
			return oldConfigMap.DeepCopy(), nil
		})

		store := &Store{
			authorizer:      commonAuthorizer,
			configMapClient: configMapClient,
			userCache:       userCache,
			tokenMgr:        tokenManager,
		}

		ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: "not-an-owner",
		})

		options := &metav1.UpdateOptions{}

		kubeconfig, isCreated, err := store.Update(ctx, kubeconfigID, nil, nil, nil, false, options)
		require.Error(t, err)
		assert.Nil(t, kubeconfig)
		assert.False(t, isCreated)
		assert.True(t, apierrors.IsNotFound(err))

		statusErr, ok := err.(*apierrors.StatusError)
		require.True(t, ok)
		assert.Equal(t, gvr.Group, statusErr.Status().Details.Group)
		assert.Equal(t, ext.KubeconfigResourceName, statusErr.Status().Details.Kind)
		assert.Equal(t, kubeconfigID, statusErr.Status().Details.Name)
	})
	t.Run("configMap doen't have correct kind label", func(t *testing.T) {
		oldConfigMap := oldConfigMap.DeepCopy()
		oldConfigMap.Labels[KindLabel] = "not-a-kubeconfig"
		configMapClient := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
		configMapClient.EXPECT().Get(namespace, kubeconfigID, gomock.Any()).DoAndReturn(func(namespace, name string, options metav1.GetOptions) (*corev1.ConfigMap, error) {
			return oldConfigMap.DeepCopy(), nil
		})

		store := &Store{
			authorizer:      commonAuthorizer,
			configMapClient: configMapClient,
			userCache:       userCache,
			tokenMgr:        tokenManager,
		}

		ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: userID,
		})

		options := &metav1.UpdateOptions{}

		kubeconfig, isCreated, err := store.Update(ctx, kubeconfigID, nil, nil, nil, false, options)
		require.Error(t, err)
		assert.Nil(t, kubeconfig)
		assert.False(t, isCreated)
		assert.True(t, apierrors.IsNotFound(err))

		statusErr, ok := err.(*apierrors.StatusError)
		require.True(t, ok)
		assert.Equal(t, gvr.Group, statusErr.Status().Details.Group)
		assert.Equal(t, ext.KubeconfigResourceName, statusErr.Status().Details.Kind)
		assert.Equal(t, kubeconfigID, statusErr.Status().Details.Name)
	})
	t.Run("configMap doen't exist", func(t *testing.T) {
		configMapClient := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
		configMapClient.EXPECT().Get(namespace, kubeconfigID, gomock.Any()).DoAndReturn(func(namespace, name string, options metav1.GetOptions) (*corev1.ConfigMap, error) {
			return nil, apierrors.NewNotFound(gvr.GroupResource(), name)
		})

		store := &Store{
			authorizer:      commonAuthorizer,
			configMapClient: configMapClient,
			userCache:       userCache,
			tokenMgr:        tokenManager,
		}

		ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: userID,
		})

		options := &metav1.UpdateOptions{}

		kubeconfig, isCreated, err := store.Update(ctx, kubeconfigID, nil, nil, nil, false, options)
		require.Error(t, err)
		assert.Nil(t, kubeconfig)
		assert.False(t, isCreated)
		assert.True(t, apierrors.IsNotFound(err))

		statusErr, ok := err.(*apierrors.StatusError)
		require.True(t, ok)
		assert.Equal(t, gvr.Group, statusErr.Status().Details.Group)
		assert.Equal(t, ext.KubeconfigResourceName, statusErr.Status().Details.Kind)
		assert.Equal(t, kubeconfigID, statusErr.Status().Details.Name)
	})
	t.Run("immutable fields", func(t *testing.T) {
		configMapClient := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
		configMapClient.EXPECT().Get(namespace, kubeconfigID, gomock.Any()).DoAndReturn(func(namespace, name string, options metav1.GetOptions) (*corev1.ConfigMap, error) {
			return oldConfigMap.DeepCopy(), nil
		}).AnyTimes()

		store := &Store{
			authorizer:      commonAuthorizer,
			configMapClient: configMapClient,
			userCache:       userCache,
			tokenMgr:        tokenManager,
		}

		updateValidation := func(ctx context.Context, obj, old runtime.Object) error { return nil }

		ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: userID,
		})

		options := &metav1.UpdateOptions{}

		oldKubeconfig, err := store.fromConfigMap(oldConfigMap)
		assert.NoError(t, err)

		t.Run("spec.clusters", func(t *testing.T) {
			newKubeconfig := oldKubeconfig.DeepCopy()
			newKubeconfig.Spec.Clusters = []string{"foo", "bar"}
			objInfo := &fakeUpdatedObjectInfo{obj: newKubeconfig}

			kubeconfig, isCreated, err := store.Update(ctx, kubeconfigID, objInfo, nil, updateValidation, false, options)
			require.Error(t, err)
			assert.Nil(t, kubeconfig)
			assert.False(t, isCreated)
			assert.True(t, apierrors.IsBadRequest(err))
		})
		t.Run("spec.currentContext", func(t *testing.T) {
			newKubeconfig := oldKubeconfig.DeepCopy()
			newKubeconfig.Spec.CurrentContext = "foo"
			objInfo := &fakeUpdatedObjectInfo{obj: newKubeconfig}

			kubeconfig, isCreated, err := store.Update(ctx, kubeconfigID, objInfo, nil, updateValidation, false, options)
			require.Error(t, err)
			assert.Nil(t, kubeconfig)
			assert.False(t, isCreated)
			assert.True(t, apierrors.IsBadRequest(err))
		})
		t.Run("spec.ttl", func(t *testing.T) {
			newKubeconfig := oldKubeconfig.DeepCopy()
			newKubeconfig.Spec.TTL = oldKubeconfig.Spec.TTL + 1
			objInfo := &fakeUpdatedObjectInfo{obj: newKubeconfig}

			kubeconfig, isCreated, err := store.Update(ctx, kubeconfigID, objInfo, nil, updateValidation, false, options)
			require.Error(t, err)
			assert.Nil(t, kubeconfig)
			assert.False(t, isCreated)
			assert.True(t, apierrors.IsBadRequest(err))
		})
	})
	t.Run("dryRun", func(t *testing.T) {
		configMapClient := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
		configMapClient.EXPECT().Get(namespace, kubeconfigID, gomock.Any()).DoAndReturn(func(namespace, name string, options metav1.GetOptions) (*corev1.ConfigMap, error) {
			return oldConfigMap.DeepCopy(), nil
		})
		configMapClient.EXPECT().Update(gomock.Any()).Times(0)

		store := &Store{
			authorizer:      commonAuthorizer,
			configMapClient: configMapClient,
			userCache:       userCache,
			tokenMgr:        tokenManager,
		}

		var updateValidationCalled bool
		updateValidation := func(ctx context.Context, obj, old runtime.Object) error {
			updateValidationCalled = true
			require.NotNil(t, obj)
			require.IsType(t, &ext.Kubeconfig{}, obj)
			require.NotNil(t, old)
			require.IsType(t, &ext.Kubeconfig{}, old)
			return nil
		}

		ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: userID, // Kubeconfig owner.
		})

		oldKubeconfig, err := store.fromConfigMap(oldConfigMap)
		assert.NoError(t, err)

		newKubeconfig := oldKubeconfig.DeepCopy()
		newKubeconfig.Spec.Description = "updated"
		newKubeconfig.Annotations["foo"] = "bar"
		newKubeconfig.Labels["foo"] = "bar"
		objInfo := &fakeUpdatedObjectInfo{obj: newKubeconfig}

		options := &metav1.UpdateOptions{
			DryRun: []string{metav1.DryRunAll},
		}

		kubeconfig, isCreated, err := store.Update(ctx, kubeconfigID, objInfo, nil, updateValidation, false, options)
		require.NoError(t, err)
		assert.NotNil(t, kubeconfig)
		assert.IsType(t, &ext.Kubeconfig{}, kubeconfig)
		assert.False(t, isCreated)

		assert.True(t, updateValidationCalled)

		assert.Equal(t, "bar", newKubeconfig.Labels["foo"])
		assert.Equal(t, "bar", newKubeconfig.Annotations["foo"])
		assert.Equal(t, "updated", newKubeconfig.Spec.Description)
	})
}

func TestStoreDelete(t *testing.T) {
	t.Parallel()

	kubeconfigID := "kubeconfig-49d5p"

	ctrl := gomock.NewController(t)
	userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	userCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.User, error) {
		switch name {
		case userID:
			return &v3.User{}, nil
		case "error":
			return nil, fmt.Errorf("some error")
		default:
			return nil, apierrors.NewNotFound(gvr.GroupResource(), name)
		}
	}).AnyTimes()

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeconfigID,
			Namespace: namespace,
			Labels: map[string]string{
				UserIDLabel: userID,
				KindLabel:   KindLabelValue,
			},
			Annotations: map[string]string{
				UIDAnnotation: string(uuid.NewUUID()),
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "management.cattle.io/v3",
					Kind:       "Token",
					Name:       "kubeconfig-u-w7drcgc66",
					UID:        uuid.NewUUID(),
				},
			},
		},
		Data: map[string]string{
			TTLField:            "43200",
			DescriptionField:    "test",
			CurrentContextField: "c-m-tbgzfbgf",
			ClustersField:       `["c-m-tbgzfbgf","c-m-bxn2p7w6"]`,
		},
	}

	tokenManager := &fakeTokenManager{}

	t.Run("admin deletes a kubeconfig", func(t *testing.T) {
		var deleteValidationCalled bool
		deleteValidation := func(ctx context.Context, obj runtime.Object) error {
			deleteValidationCalled = true
			return nil
		}
		deleteOptions := &metav1.DeleteOptions{
			GracePeriodSeconds: ptr.To(int64(60)),
		}

		configMapClient := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
		configMapClient.EXPECT().Get(namespace, gomock.Any(), gomock.Any()).DoAndReturn(func(namespace, name string, options metav1.GetOptions) (*corev1.ConfigMap, error) {
			return configMap.DeepCopy(), nil
		}).Times(1)
		configMapClient.EXPECT().Delete(namespace, kubeconfigID, gomock.Any()).DoAndReturn(func(namespace, name string, options *metav1.DeleteOptions) error {
			assert.Equal(t, deleteOptions, options)
			return nil
		}).Times(1)

		tokenID1 := "kubeconfig-" + adminID + "agc66"
		tokenID2 := "kubeconfig-" + adminID + "d12fg"

		tokenCache := fake.NewMockNonNamespacedCacheInterface[*v3.Token](ctrl)
		tokenCache.EXPECT().List(gomock.Any()).DoAndReturn(func(selector labels.Selector) ([]*v3.Token, error) {
			set, err := labels.ConvertSelectorToLabelsMap(selector.String())
			require.NoError(t, err)
			assert.Equal(t, kubeconfigID, set.Get(tokens.TokenKubeconfigIDLabel))
			assert.Equal(t, KindLabelValue, set.Get(tokens.TokenKindLabel))

			return []*v3.Token{
				{ObjectMeta: metav1.ObjectMeta{Name: tokenID1}},
				{ObjectMeta: metav1.ObjectMeta{Name: tokenID2}},
			}, nil
		}).AnyTimes()
		tokenClient := fake.NewMockNonNamespacedClientInterface[*v3.Token, *v3.TokenList](ctrl)
		tokenClient.EXPECT().Delete(gomock.Any(), gomock.Any()).DoAndReturn(func(name string, options *metav1.DeleteOptions) error {
			assert.Contains(t, []string{tokenID1, tokenID2}, name)
			assert.Equal(t, deleteOptions.GracePeriodSeconds, options.GracePeriodSeconds)
			assert.Equal(t, deleteOptions.PropagationPolicy, options.PropagationPolicy)
			assert.Equal(t, deleteOptions.DryRun, options.DryRun)

			return nil
		}).Times(2)

		store := &Store{
			authorizer:      commonAuthorizer,
			configMapClient: configMapClient,
			tokenCache:      tokenCache,
			tokens:          tokenClient,
			userCache:       userCache,
			tokenMgr:        tokenManager,
		}

		ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: adminID,
		})

		obj, _, err := store.Delete(ctx, kubeconfigID, deleteValidation, deleteOptions)
		require.NoError(t, err)
		require.NotNil(t, obj)
		require.IsType(t, &ext.Kubeconfig{}, obj)

		kubeconfig := obj.(*ext.Kubeconfig)
		assert.Equal(t, kubeconfigID, kubeconfig.Name)

		assert.True(t, deleteValidationCalled)
	})
	t.Run("user can't delete other user's kubeconfig", func(t *testing.T) {
		configMapClient := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
		configMapClient.EXPECT().Get(namespace, gomock.Any(), gomock.Any()).DoAndReturn(func(namespace, name string, options metav1.GetOptions) (*corev1.ConfigMap, error) {
			return configMap.DeepCopy(), nil
		}).Times(1)

		store := &Store{
			authorizer:      commonAuthorizer,
			configMapClient: configMapClient,
			userCache:       userCache,
			tokenMgr:        tokenManager,
		}

		ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: "not-an-owner",
		})

		obj, _, err := store.Delete(ctx, kubeconfigID, nil, &metav1.DeleteOptions{})
		require.Error(t, err)
		require.Nil(t, obj)
		assert.True(t, apierrors.IsNotFound(err))

		statusErr, ok := err.(*apierrors.StatusError)
		require.True(t, ok)
		assert.Equal(t, gvr.Group, statusErr.Status().Details.Group)
		assert.Equal(t, ext.KubeconfigResourceName, statusErr.Status().Details.Kind)
		assert.Equal(t, kubeconfigID, statusErr.Status().Details.Name)
	})
}

func TestStoreDeleteCollection(t *testing.T) {
	t.Parallel()

	kubeconfigID := "kubeconfig-49d5p"

	ctrl := gomock.NewController(t)
	userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	userCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.User, error) {
		switch name {
		case userID:
			return &v3.User{}, nil
		case "error":
			return nil, fmt.Errorf("some error")
		default:
			return nil, apierrors.NewNotFound(gvr.GroupResource(), name)
		}
	}).AnyTimes()

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeconfigID,
			Namespace: namespace,
			Labels: map[string]string{
				UserIDLabel: userID,
				KindLabel:   KindLabelValue,
			},
			Annotations: map[string]string{
				UIDAnnotation: string(uuid.NewUUID()),
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "management.cattle.io/v3",
					Kind:       "Token",
					Name:       "kubeconfig-u-w7drcgc66",
					UID:        uuid.NewUUID(),
				},
			},
		},
		Data: map[string]string{
			TTLField:            "43200",
			DescriptionField:    "test",
			CurrentContextField: "c-m-tbgzfbgf",
			ClustersField:       `["c-m-tbgzfbgf","c-m-bxn2p7w6"]`,
		},
	}

	tokenManager := &fakeTokenManager{}

	t.Run("admin deletes users kubeconfigs with a label", func(t *testing.T) {
		var deleteValidationCalledTimes int
		deleteValidation := func(ctx context.Context, obj runtime.Object) error {
			deleteValidationCalledTimes++
			return nil
		}
		deleteOptions := &metav1.DeleteOptions{
			GracePeriodSeconds: ptr.To(int64(60)),
		}
		listOptions := &metainternalversion.ListOptions{
			LabelSelector: labels.Set{
				UserIDLabel: userID,
				"custom":    "label",
			}.AsSelector(),
		}

		configMapClient := fake.NewMockClientInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
		configMapClient.EXPECT().List(namespace, gomock.Any()).DoAndReturn(func(namespace string, options metav1.ListOptions) (*corev1.ConfigMapList, error) {
			labelSet, err := labels.ConvertSelectorToLabelsMap(options.LabelSelector)
			require.NoError(t, err)
			assert.Equal(t, KindLabelValue, labelSet[KindLabel])
			assert.Equal(t, userID, labelSet[UserIDLabel])
			assert.Equal(t, "label", labelSet["custom"])

			return &corev1.ConfigMapList{
				ListMeta: metav1.ListMeta{
					ResourceVersion: "1",
				},
				Items: []corev1.ConfigMap{*configMap.DeepCopy()},
			}, nil
		}).Times(1)
		configMapClient.EXPECT().Delete(namespace, kubeconfigID, gomock.Any()).DoAndReturn(func(namespace, name string, options *metav1.DeleteOptions) error {
			assert.Equal(t, deleteOptions, options)
			return nil
		}).Times(1)

		tokenID1 := "kubeconfig-" + userID + "agc66"
		tokenID2 := "kubeconfig-" + userID + "d12fg"

		tokenCache := fake.NewMockNonNamespacedCacheInterface[*v3.Token](ctrl)
		tokenCache.EXPECT().List(gomock.Any()).DoAndReturn(func(selector labels.Selector) ([]*v3.Token, error) {
			set, err := labels.ConvertSelectorToLabelsMap(selector.String())
			require.NoError(t, err)
			assert.Equal(t, kubeconfigID, set.Get(tokens.TokenKubeconfigIDLabel))
			assert.Equal(t, KindLabelValue, set.Get(tokens.TokenKindLabel))

			return []*v3.Token{
				{ObjectMeta: metav1.ObjectMeta{Name: tokenID1}},
				{ObjectMeta: metav1.ObjectMeta{Name: tokenID2}},
			}, nil
		}).AnyTimes()
		tokenClient := fake.NewMockNonNamespacedClientInterface[*v3.Token, *v3.TokenList](ctrl)
		tokenClient.EXPECT().Delete(gomock.Any(), gomock.Any()).DoAndReturn(func(name string, options *metav1.DeleteOptions) error {
			assert.Contains(t, []string{tokenID1, tokenID2}, name)
			assert.Equal(t, deleteOptions.GracePeriodSeconds, options.GracePeriodSeconds)
			assert.Equal(t, deleteOptions.PropagationPolicy, options.PropagationPolicy)
			assert.Equal(t, deleteOptions.DryRun, options.DryRun)

			return nil
		}).Times(2)

		store := &Store{
			authorizer:      commonAuthorizer,
			configMapClient: configMapClient,
			tokenCache:      tokenCache,
			tokens:          tokenClient,
			userCache:       userCache,
			tokenMgr:        tokenManager,
		}

		ctx := request.WithUser(context.Background(), &k8suser.DefaultInfo{
			Name: adminID,
		})

		obj, err := store.DeleteCollection(ctx, deleteValidation, deleteOptions, listOptions)
		require.NoError(t, err)
		require.NotNil(t, obj)
		require.IsType(t, &ext.KubeconfigList{}, obj)

		list := obj.(*ext.KubeconfigList)
		require.Len(t, list.Items, 1)
		assert.Equal(t, "1", list.ResourceVersion)

		assert.Equal(t, deleteValidationCalledTimes, 1)
	})
}

func TestPrintKubeconfig(t *testing.T) {
	t.Parallel()

	kubeconfig := &ext.Kubeconfig{
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: metav1.NewTime(time.Now()),
			Name:              "kubeconfig-49d5p",
			Labels: map[string]string{
				UserIDLabel: "u-w7drc",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "management.cattle.io/v3",
					Kind:       "Token",
					Name:       "kubeconfig-u-w7drcd12fg",
					UID:        uuid.NewUUID(),
				},
			},
		},
		Spec: ext.KubeconfigSpec{
			Description:    "test",
			CurrentContext: "c-m-tbgzfbgf",
			Clusters:       []string{"c-m-tbgzfbgf", "c-m-bxn2p7w6"},
			TTL:            43200,
		},
		Status: ext.KubeconfigStatus{
			Summary: StatusSummaryComplete,
			Tokens: []string{
				"kubeconfig-u-w7drcgc66",
				"kubeconfig-u-w7drcd12fg",
			},
		},
	}

	t.Run("completed kubeconfig", func(t *testing.T) {
		rows, err := printKubeconfig(kubeconfig, printers.GenerateOptions{})
		require.NoError(t, err)
		require.Len(t, rows, 1)
		row := rows[0]
		require.Len(t, row.Cells, 8)
		assert.Equal(t, kubeconfig.Name, row.Cells[0].(string))
		assert.Equal(t, "12h", row.Cells[1].(string))
		assert.Equal(t, "1/2", row.Cells[2].(string))
		assert.Equal(t, "Complete", row.Cells[3].(string))
		assert.Equal(t, "0s", row.Cells[4].(string))
		assert.Equal(t, kubeconfig.Labels[UserIDLabel], row.Cells[5].(string))
		assert.Equal(t, "c-m-tbgzfbgf,c-m-bxn2p7w6", row.Cells[6].(string))
		assert.Equal(t, kubeconfig.Spec.Description, row.Cells[7].(string))
	})
	t.Run("missing age and status", func(t *testing.T) {
		kubeconfig := kubeconfig.DeepCopy()
		kubeconfig.CreationTimestamp = metav1.Time{}
		kubeconfig.Status = ext.KubeconfigStatus{}

		rows, err := printKubeconfig(kubeconfig, printers.GenerateOptions{})
		require.NoError(t, err)
		require.Len(t, rows, 1)
		row := rows[0]
		require.Len(t, row.Cells, 8)
		assert.Equal(t, unknownValue, row.Cells[3].(string))
		assert.Equal(t, unknownValue, row.Cells[4].(string))
	})
}
