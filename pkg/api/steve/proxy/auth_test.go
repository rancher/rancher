package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/tunnelserver"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestClusterProxyAuthorizer_Authorize(t *testing.T) {
	tests := []struct {
		name          string
		authHeader    string
		secrets       []*corev1.Secret
		getByIndex    []*corev1.Secret
		getByIndexErr error
		wantID        string
		wantOK        bool
		wantErr       bool
		// wantTracked is the UID of the cluster the session should be tracked for, if any.
		wantTracked types.UID
	}{
		{
			name:       "no prefix",
			authHeader: "Bearer sometoken",
			wantOK:     false,
		},
		{
			name:       "valid token maps to cluster namespace",
			authHeader: "Bearer " + Prefix + "tok",
			getByIndex: []*corev1.Secret{
				{ObjectMeta: metav1.ObjectMeta{Namespace: "c-abc", Name: "crt-token-system"}},
			},
			wantID:      Prefix + "c-abc",
			wantOK:      true,
			wantTracked: "uid-abc",
		},
		{
			// Its session could not be tracked by UID, and would survive into a cluster created under the name.
			name:       "token of a cluster that can't be found is unauthorized",
			authHeader: "Bearer " + Prefix + "tok",
			getByIndex: []*corev1.Secret{
				{ObjectMeta: metav1.ObjectMeta{Namespace: "c-missing", Name: "crt-token-system"}},
			},
			wantOK: false,
		},
		{
			name:          "not found error is treated as unauthorized",
			authHeader:    "Bearer " + Prefix + "tok",
			getByIndexErr: apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, "tok"),
			wantOK:        false,
		},
		{
			name:       "no matching secret",
			authHeader: "Bearer " + Prefix + "tok",
			getByIndex: nil,
			wantOK:     false,
		},
		{
			name:       "token from a previous cluster with the same name is unauthorized",
			authHeader: "Bearer " + Prefix + "tok",
			getByIndex: []*corev1.Secret{
				{ObjectMeta: metav1.ObjectMeta{Namespace: "c-old", Name: "crt-token-system"}},
			},
			wantOK: false,
		},
		{
			name:       "a stale match does not hide a usable one",
			authHeader: "Bearer " + Prefix + "tok",
			getByIndex: []*corev1.Secret{
				{ObjectMeta: metav1.ObjectMeta{Namespace: "c-old", Name: "crt-token-system"}},
				{ObjectMeta: metav1.ObjectMeta{Namespace: "c-abc", Name: "crt-token-system"}},
			},
			wantID:      Prefix + "c-abc",
			wantOK:      true,
			wantTracked: "uid-abc",
		},
		{
			name:       "unexpected error with results is propagated",
			authHeader: "Bearer " + Prefix + "tok",
			getByIndex: []*corev1.Secret{
				{ObjectMeta: metav1.ObjectMeta{Namespace: "c-abc", Name: "crt-token-system"}},
			},
			getByIndexErr: assert.AnError,
			wantOK:        false,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockCache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
			mockCache.EXPECT().GetByIndex(tokenIndex, gomock.Any()).Return(tt.getByIndex, tt.getByIndexErr).AnyTimes()

			// c-old is the namespace of a cluster that was deleted; its tokens are still being torn down.
			namespaceCache := fake.NewMockNonNamespacedCacheInterface[*corev1.Namespace](ctrl)
			namespaceCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*corev1.Namespace, error) {
				ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
				if name == "c-old" {
					ns.Status.Phase = corev1.NamespaceTerminating
				}
				return ns, nil
			}).AnyTimes()

			clusterCache := fake.NewMockNonNamespacedCacheInterface[*apimgmtv3.Cluster](ctrl)
			clusterCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*apimgmtv3.Cluster, error) {
				switch name {
				case "c-abc":
					return &apimgmtv3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: name, UID: "uid-abc"}}, nil
				case "c-old":
					return &apimgmtv3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: name, UID: "uid-old"}}, nil
				}
				return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "clusters"}, name)
			}).AnyTimes()

			a := &clusterProxyAuthorizer{secretCache: mockCache, namespaceCache: namespaceCache, clusterCache: clusterCache}

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", tt.authHeader)

			var (
				id         string
				ok         bool
				err        error
				trackedUID types.UID
			)
			tunnelserver.NewSessionTracker().Handler(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				id, ok, err = a.Authorize(r)
				_, trackedUID = tunnelserver.SessionCluster(r)
			})).ServeHTTP(httptest.NewRecorder(), req)
			assert.Equal(t, tt.wantTracked, trackedUID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantID, id)
		})
	}
}

func TestClusterProxyAuthorizerTracksTheSessionForTheClusterTheTokenWasCheckedAgainst(t *testing.T) {
	// A second lookup could find a cluster that replaced the checked one under the same name: the session
	// would then be tracked as the replacement's, and survive the removal of the cluster it belongs to.
	ctrl := gomock.NewController(t)
	secretCache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
	secretCache.EXPECT().GetByIndex(tokenIndex, "tok").Return([]*corev1.Secret{
		{ObjectMeta: metav1.ObjectMeta{Namespace: "c-abc", Name: "crt-token-system"}},
	}, nil)
	namespaceCache := fake.NewMockNonNamespacedCacheInterface[*corev1.Namespace](ctrl)
	namespaceCache.EXPECT().Get("c-abc").Return(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "c-abc"}}, nil)
	clusterCache := fake.NewMockNonNamespacedCacheInterface[*apimgmtv3.Cluster](ctrl)
	gomock.InOrder(
		clusterCache.EXPECT().Get("c-abc").Return(&apimgmtv3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-abc", UID: "uid-checked"}}, nil),
		clusterCache.EXPECT().Get("c-abc").Return(&apimgmtv3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-abc", UID: "uid-replacement"}}, nil).AnyTimes(),
	)
	a := &clusterProxyAuthorizer{secretCache: secretCache, namespaceCache: namespaceCache, clusterCache: clusterCache}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+Prefix+"tok")
	var trackedUID types.UID
	tunnelserver.NewSessionTracker().Handler(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, ok, err := a.Authorize(r)
		require.NoError(t, err)
		require.True(t, ok)
		_, trackedUID = tunnelserver.SessionCluster(r)
	})).ServeHTTP(httptest.NewRecorder(), req)

	assert.Equal(t, types.UID("uid-checked"), trackedUID)
}
