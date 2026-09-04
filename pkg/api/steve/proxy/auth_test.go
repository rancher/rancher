package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
			wantID: Prefix + "c-abc",
			wantOK: true,
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

			a := &clusterProxyAuthorizer{secretCache: mockCache}

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", tt.authHeader)

			id, ok, err := a.Authorize(req)
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
