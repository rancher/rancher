package cluster

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rancher/norman/types"
	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	v3 "github.com/rancher/rancher/pkg/client/generated/management/v3"
	exttokenstore "github.com/rancher/rancher/pkg/ext/stores/tokens"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	managementSchema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	userMocks "github.com/rancher/rancher/pkg/user/mocks"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestGenerateKubeConfigBearer(t *testing.T) {
	const (
		testClusterName = "test-cluster"
		fakeHost        = "fake-request-host.fake"
		testUser        = "fake-user"
	)

	testSchemas := types.NewSchemas().AddSchemas(managementSchema.Schemas)
	clusterSchema := testSchemas.Schema(&managementSchema.Version, v3.ClusterType)
	fakeStore := fakeClusterStore{
		cluster: v3.Cluster{
			Name: testClusterName,
		},
	}
	clusterSchema.Store = &fakeStore

	t.Run("kubeconfig bearer token, ext token auth", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		userManager := userMocks.NewMockManager(ctrl)
		userManager.EXPECT().GetUser(gomock.Any()).Return(testUser).AnyTimes()

		recorder := normanRecorder{}
		apiContext := &types.APIContext{
			ID:             testClusterName,
			Version:        &managementSchema.Version,
			Type:           v3.ClusterType,
			ResponseWriter: &recorder,
			Schemas:        testSchemas,
			Request: &http.Request{
				Host: fakeHost,
				Body: io.NopCloser(strings.NewReader(`{}`)),
			},
		}

		fakeAuth := fakeExtAuthToken{
			token: ext.Token{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fake-ext",
				},
				Spec: ext.TokenSpec{
					UserID: testUser,
					UserPrincipal: ext.TokenPrincipal{
						Provider: "local",
						Name:     testUser,
					},
				},
			},
		}
		fakePrincipalBytes, _ := json.Marshal(fakeAuth.token.Spec.UserPrincipal)
		fakeAuthSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: fakeAuth.token.Name,
				Labels: map[string]string{
					exttokenstore.UserIDLabel:     testUser,
					exttokenstore.SecretKindLabel: exttokenstore.SecretKindLabelValue,
				},
				UID: "",
			},
			Data: map[string][]byte{
				exttokenstore.FieldDescription:    []byte(""),
				exttokenstore.FieldEnabled:        []byte("true"),
				exttokenstore.FieldHash:           []byte("al;dgkl;dkfdsafdioj"),
				exttokenstore.FieldKind:           []byte(exttokenstore.IsLogin),
				exttokenstore.FieldLastUpdateTime: []byte("13:00:05"),
				exttokenstore.FieldPrincipal:      fakePrincipalBytes,
				exttokenstore.FieldTTL:            []byte("-1"),
				exttokenstore.FieldUID:            []byte("kubid"),
				exttokenstore.FieldUserID:         []byte(testUser),
			},
		}

		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
		secrets.EXPECT().Cache().Return(scache)

		users := fake.NewMockNonNamespacedControllerInterface[*apimgmtv3.User, *apimgmtv3.UserList](ctrl)
		ucache := fake.NewMockNonNamespacedCacheInterface[*apimgmtv3.User](ctrl)
		users.EXPECT().Cache().Return(ucache)
		ucache.EXPECT().Get(testUser).Return(&apimgmtv3.User{
			ObjectMeta: metav1.ObjectMeta{
				Name: testUser,
			},
		}, nil).AnyTimes()

		v3tcache := fake.NewMockNonNamespacedCacheInterface[*apimgmtv3.Token](ctrl)
		v3tcache.EXPECT().Get(fakeAuth.token.Name).Return(nil,
			apierrors.NewNotFound(schema.GroupResource{}, fakeAuth.token.Name))

		scache.EXPECT().Get(exttokenstore.TokenNamespace, fakeAuth.token.Name).Return(fakeAuthSecret, nil)
		secrets.EXPECT().Create(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
			assert.Equal(t, "the-hash", s.StringData[exttokenstore.FieldHash])
			n := s.DeepCopy()
			n.Name = "token-xxx"
			n.Data = map[string][]byte{}
			for k, v := range n.StringData {
				n.Data[k] = []byte(v)
			}
			return n, nil
		})

		nscache := fake.NewMockNonNamespacedCacheInterface[*corev1.Namespace](ctrl)
		nscache.EXPECT().Get(exttokenstore.TokenNamespace).AnyTimes()

		handler := ActionHandler{
			NodeLister: &fakes.NodeListerMock{
				GetFunc: func(namespace string, name string) (*apimgmtv3.Node, error) {
					return nil, nil
				},
				ListFunc: func(namespace string, selector labels.Selector) ([]*apimgmtv3.Node, error) {
					return nil, nil
				},
			},
			UserMgr:   userManager,
			TokenMgr:  &fakeTokenManager{},
			AuthToken: &fakeAuth,
			ExtTokenStore: exttokenstore.NewSystem(nil, nscache, secrets, users, v3tcache, nil,
				exttokenstore.NewTimeHandler(),
				&fakeHash{},
				exttokenstore.NewAuthHandler(), nil),
		}
		bearer, err := handler.generateKubeConfigBearer(apiContext)
		assert.NoError(t, err, "got an error when calling generate kubeconfig")
		assert.Equal(t, "ext/token-xxx:the-secret", bearer)
	})

	t.Run("kubeconfig bearer token, legacy token auth", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		userManager := userMocks.NewMockManager(ctrl)
		userManager.EXPECT().GetUser(gomock.Any()).Return(testUser).AnyTimes()

		recorder := normanRecorder{}
		apiContext := &types.APIContext{
			ID:             testClusterName,
			Version:        &managementSchema.Version,
			Type:           v3.ClusterType,
			ResponseWriter: &recorder,
			Schemas:        testSchemas,
			Request: &http.Request{
				Host: fakeHost,
				Body: io.NopCloser(strings.NewReader(`{}`)),
			},
		}

		fakeAuth := fakeAuthToken{
			token: apimgmtv3.Token{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fake-legacy",
				},
				UserID: testUser,
				AuthProvider: "local",
				UserPrincipal: apimgmtv3.Principal{
					Provider: "local",
					ObjectMeta: metav1.ObjectMeta{
						Name: testUser,
					},
				},
			},
		}

		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
		secrets.EXPECT().Cache().Return(scache)

		users := fake.NewMockNonNamespacedControllerInterface[*apimgmtv3.User, *apimgmtv3.UserList](ctrl)
		ucache := fake.NewMockNonNamespacedCacheInterface[*apimgmtv3.User](ctrl)
		users.EXPECT().Cache().Return(ucache)
		ucache.EXPECT().Get(testUser).Return(&apimgmtv3.User{
			ObjectMeta: metav1.ObjectMeta{
				Name: testUser,
			},
		}, nil).AnyTimes()

		v3tcache := fake.NewMockNonNamespacedCacheInterface[*apimgmtv3.Token](ctrl)
		v3tcache.EXPECT().Get(fakeAuth.token.Name).Return(&fakeAuth.token, nil)

		secrets.EXPECT().Create(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
			assert.Equal(t, "the-hash", s.StringData[exttokenstore.FieldHash])
			n := s.DeepCopy()
			n.Name = "token-xxx"
			n.Data = map[string][]byte{}
			for k, v := range n.StringData {
				n.Data[k] = []byte(v)
			}
			return n, nil
		})

		nscache := fake.NewMockNonNamespacedCacheInterface[*corev1.Namespace](ctrl)
		nscache.EXPECT().Get(exttokenstore.TokenNamespace).AnyTimes()

		handler := ActionHandler{
			NodeLister: &fakes.NodeListerMock{
				GetFunc: func(namespace string, name string) (*apimgmtv3.Node, error) {
					return nil, nil
				},
				ListFunc: func(namespace string, selector labels.Selector) ([]*apimgmtv3.Node, error) {
					return nil, nil
				},
			},
			UserMgr:   userManager,
			TokenMgr:  &fakeTokenManager{},
			AuthToken: &fakeAuth,
			ExtTokenStore: exttokenstore.NewSystem(nil, nscache, secrets, users, v3tcache, nil,
				exttokenstore.NewTimeHandler(),
				&fakeHash{},
				exttokenstore.NewAuthHandler(), nil),
		}
		bearer, err := handler.generateKubeConfigBearer(apiContext)
		assert.NoError(t, err, "got an error when calling generate kubeconfig")
		assert.Equal(t, "ext/token-xxx:the-secret", bearer)
	})
}

// fakeAuthToken implements requests.Authenticator for the purposes of testing
type fakeExtAuthToken struct {
	token ext.Token
	err   error
}

func (f *fakeExtAuthToken) TokenFromRequest(req *http.Request) (accessor.TokenAccessor, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &f.token, nil
}

// fakeHash implements the [hashHandler] interface
type fakeHash struct {
}

func (h *fakeHash) MakeAndHashSecret() (string, string, error) {
	return "the-secret", "the-hash", nil
}
