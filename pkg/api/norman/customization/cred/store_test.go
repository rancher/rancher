package cred

import (
	"errors"
	"fmt"
	"testing"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	prov "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	provv1 "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	corefakes "github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	v3fakes "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestTokenNamesFromContent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		content   string
		input     Set[string]
		expected  Set[string]
		expectErr bool
	}{
		{
			name:    "empty content",
			content: "",
		},
		{
			name:      "invalid content",
			content:   "a:b:c:d",
			expectErr: true,
		},
		{
			name:    "no matching tokens",
			content: "users:\n- user:\n    token: non-kubeconfig-user",
		},
		{
			name:      "malformed token",
			content:   "users:\n- user:\n    token: kubeconfig-user-test",
			expectErr: true,
		},
		{
			name:     "single matching token",
			content:  "users:\n- user:\n    token: kubeconfig-user-test:thisisanexampletoken",
			input:    Set[string]{},
			expected: Set[string]{"kubeconfig-user-test": struct{}{}},
		},
		{
			name:     "multiple matching tokens",
			content:  "users:\n- user:\n    token: kubeconfig-user-test0:thisisanexampletoken\n- user:\n    token: kubeconfig-user-test1:thisisanexampletoken",
			input:    Set[string]{},
			expected: Set[string]{"kubeconfig-user-test0": struct{}{}, "kubeconfig-user-test1": struct{}{}},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := TokenNamesFromContent([]byte(tt.content), tt.input)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, tt.input)
			}
		})
	}
}

func TestProcessHarvesterCloudCredential(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        map[string]any
		expectErr    bool
		secretLister *corefakes.SecretListerMock
		tokenLister  *v3fakes.TokenListerMock
	}{
		{
			name:  "empty config",
			input: map[string]any{},
		},
		{
			name: "empty kubeconfig",
			input: map[string]any{
				"harvestercredentialConfig": map[string]any{},
			},
		},
		{
			name: "invalid kubeconfig",
			input: map[string]any{
				"harvestercredentialConfig": map[string]any{
					"kubeconfigContent": "a:b:c:d",
				},
			},
			expectErr: true,
		},
		{
			name: "token not found",
			input: map[string]any{
				"harvestercredentialConfig": map[string]any{
					"kubeconfigContent": "users:\n- user:\n    token: kubeconfig-user-test:thisisanexampletoken",
				},
			},
			expectErr: true,
			secretLister: &corefakes.SecretListerMock{
				ListFunc: func(namespace string, selector labels.Selector) ([]*corev1.Secret, error) {
					return []*corev1.Secret{}, nil
				},
			},
			tokenLister: &v3fakes.TokenListerMock{
				GetFunc: func(namespace string, name string) (*apimgmtv3.Token, error) {
					return nil, apierrors.NewNotFound(apimgmtv3.Resource("token"), name)
				},
			},
		},
		{
			name: "token expired",
			input: map[string]any{
				"harvestercredentialConfig": map[string]any{
					"kubeconfigContent": "users:\n- user:\n    token: kubeconfig-user-test:thisisanexampletoken",
				},
			},
			expectErr: true,
			secretLister: &corefakes.SecretListerMock{
				ListFunc: func(namespace string, selector labels.Selector) ([]*corev1.Secret, error) {
					return []*corev1.Secret{}, nil
				},
			},
			tokenLister: &v3fakes.TokenListerMock{
				GetFunc: func(namespace string, name string) (*apimgmtv3.Token, error) {
					return &apimgmtv3.Token{
						ObjectMeta: metav1.ObjectMeta{
							Name: name,
						},
						Expired: true,
					}, nil
				},
			},
		},
		{
			name: "success",
			input: map[string]any{
				"harvestercredentialConfig": map[string]any{
					"kubeconfigContent": "users:\n- user:\n    token: kubeconfig-user-test:thisisanexampletoken",
				},
			},
			secretLister: &corefakes.SecretListerMock{
				ListFunc: func(namespace string, selector labels.Selector) ([]*corev1.Secret, error) {
					return []*corev1.Secret{}, nil
				},
			},
			tokenLister: &v3fakes.TokenListerMock{
				GetFunc: func(namespace string, name string) (*apimgmtv3.Token, error) {
					return &apimgmtv3.Token{
						ObjectMeta: metav1.ObjectMeta{
							Name: name,
						},
					}, nil
				},
			},
		},
		{
			name: "multiple tokens",
			input: map[string]any{
				"harvestercredentialConfig": map[string]any{
					"kubeconfigContent": "users:\n- user:\n    token: kubeconfig-user-test-0:thisisanexampletoken\n- user:\n    token: kubeconfig-user-test-1:thisisanexampletoken",
				},
			},
			secretLister: &corefakes.SecretListerMock{
				ListFunc: func(namespace string, selector labels.Selector) ([]*corev1.Secret, error) {
					return []*corev1.Secret{}, nil
				},
			},
			tokenLister: &v3fakes.TokenListerMock{
				GetFunc: func(namespace string, name string) (*apimgmtv3.Token, error) {
					return &apimgmtv3.Token{
						ObjectMeta: metav1.ObjectMeta{
							Name: name,
						},
					}, nil
				},
			},
		},
		{
			name: "token is already owned",
			input: map[string]any{
				"harvestercredentialConfig": map[string]any{
					"kubeconfigContent": "users:\n- user:\n    token: kubeconfig-user-test:thisisanexampletoken",
				},
			},
			expectErr: true,
			secretLister: &corefakes.SecretListerMock{
				ListFunc: func(namespace string, selector labels.Selector) ([]*corev1.Secret, error) {
					return []*corev1.Secret{
						{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: namespace,
								Name:      "test-secret",
							},
							Data: map[string][]byte{
								"harvestercredentialConfig-kubeconfigContent": []byte("users:\n- user:\n    token: kubeconfig-user-test:thisisanexampletoken"),
							},
						},
					}, nil
				},
			},
			tokenLister: &v3fakes.TokenListerMock{
				GetFunc: func(namespace string, name string) (*apimgmtv3.Token, error) {
					return &apimgmtv3.Token{
						ObjectMeta: metav1.ObjectMeta{
							Name: name,
						},
					}, nil
				},
			},
		},
		{
			name: "single token is already owned",
			input: map[string]any{
				"harvestercredentialConfig": map[string]any{
					"kubeconfigContent": "users:\n- user:\n    token: kubeconfig-user-test-0:thisisanexampletoken\n- user:\n    token: kubeconfig-user-test-1:thisisanexampletoken",
				},
			},
			expectErr: true,
			secretLister: &corefakes.SecretListerMock{
				ListFunc: func(namespace string, selector labels.Selector) ([]*corev1.Secret, error) {
					return []*corev1.Secret{
						{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: namespace,
								Name:      "test-secret",
							},
							Data: map[string][]byte{
								"harvestercredentialConfig-kubeconfigContent": []byte("users:\n- user:\n    token: kubeconfig-user-test-2:thisisanexampletoken\n- user:\n    token: kubeconfig-user-test-1:thisisanexampletoken"),
							},
						},
					}, nil
				},
			},
			tokenLister: &v3fakes.TokenListerMock{
				GetFunc: func(namespace string, name string) (*apimgmtv3.Token, error) {
					return &apimgmtv3.Token{
						ObjectMeta: metav1.ObjectMeta{
							Name: name,
						},
					}, nil
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			s := Store{
				SecretLister: tt.secretLister,
				TokenLister:  tt.tokenLister,
			}
			err := s.processHarvesterCloudCredential(tt.input)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

type fakeInnerStore struct {
	types.Store
	deleteCalled bool
	response     map[string]any
	err          error
}

func (f *fakeInnerStore) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]any, error) {
	f.deleteCalled = true
	return f.response, f.err
}

func TestStore_Delete(t *testing.T) {
	const credID = "cattle-global-data:cc-xyz"

	type expect struct {
		wantAPIError *httperror.APIError
		wantDelete   bool
		wantResp     map[string]any
	}

	testCases := []struct {
		name     string
		setup    func(ctrl *gomock.Controller) provv1.ClusterCache
		inner    fakeInnerStore
		expected expect
	}{
		{
			name: "denies when referenced by cluster-level",
			setup: func(ctrl *gomock.Controller) provv1.ClusterCache {
				cache := fake.NewMockCacheInterface[*prov.Cluster](ctrl)
				cache.EXPECT().GetByIndex(cluster.ByCloudCred, credID).
					Return([]*prov.Cluster{{}}, nil)
				return cache
			},
			expected: expect{
				wantAPIError: &httperror.APIError{Code: httperror.InvalidAction},
				wantDelete:   false,
			},
		},
		{
			name: "denies when referenced by machine-pool",
			setup: func(ctrl *gomock.Controller) provv1.ClusterCache {
				cache := fake.NewMockCacheInterface[*prov.Cluster](ctrl)
				cache.EXPECT().GetByIndex(cluster.ByCloudCred, credID).
					Return(nil, nil)
				cache.EXPECT().GetByIndex(cluster.ByMachinePoolCloudCred, credID).
					Return([]*prov.Cluster{{}}, nil)
				return cache
			},
			expected: expect{
				wantAPIError: &httperror.APIError{Code: httperror.InvalidAction},
				wantDelete:   false,
			},
		},
		{
			name: "server error when cluster-level index fails",
			setup: func(ctrl *gomock.Controller) provv1.ClusterCache {
				cache := fake.NewMockCacheInterface[*prov.Cluster](ctrl)
				cache.EXPECT().GetByIndex(cluster.ByCloudCred, credID).
					Return(nil, errors.New("boom"))
				return cache
			},
			expected: expect{
				wantAPIError: &httperror.APIError{Code: httperror.ServerError},
				wantDelete:   false,
			},
		},
		{
			name: "server error when machine-pool index fails",
			setup: func(ctrl *gomock.Controller) provv1.ClusterCache {
				cache := fake.NewMockCacheInterface[*prov.Cluster](ctrl)
				cache.EXPECT().GetByIndex(cluster.ByCloudCred, credID).
					Return(nil, nil)
				cache.EXPECT().GetByIndex(cluster.ByMachinePoolCloudCred, credID).
					Return(nil, errors.New("boom2"))
				return cache
			},
			expected: expect{
				wantAPIError: &httperror.APIError{Code: httperror.ServerError},
				wantDelete:   false,
			},
		},
		{
			name: "delegates to inner store when not referenced",
			setup: func(ctrl *gomock.Controller) provv1.ClusterCache {
				cache := fake.NewMockCacheInterface[*prov.Cluster](ctrl)
				cache.EXPECT().GetByIndex(cluster.ByCloudCred, credID).
					Return(nil, nil)
				cache.EXPECT().GetByIndex(cluster.ByMachinePoolCloudCred, credID).
					Return(nil, nil)
				return cache
			},
			inner: fakeInnerStore{
				response: map[string]any{"ok": true},
			},
			expected: expect{
				wantAPIError: nil,
				wantDelete:   true,
				wantResp:     map[string]any{"ok": true},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			cache := testCase.setup(ctrl)
			s := &Store{
				Store:            &testCase.inner,
				ProvClusterCache: cache,
			}

			resp, err := s.Delete(nil, nil, credID)

			if testCase.expected.wantAPIError != nil {
				require.Error(t, err)
				var apiErr *httperror.APIError
				require.True(t, errors.As(err, &apiErr), "expected httperror.APIError, got %T", err)
				assert.Equal(t, testCase.expected.wantAPIError.Code.Status, apiErr.Code.Status)
				assert.False(t, testCase.inner.deleteCalled, "inner store should not be called on deny/error")
				return
			}

			require.NoError(t, err)
			assert.True(t, testCase.inner.deleteCalled, "inner store should be called")
			assert.Equal(t, testCase.expected.wantResp, resp)
		})
	}
}

func TestStore_Delete_InnerStoreErrorBubbles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const credID = "cattle-global-data:cc-err"

	cache := fake.NewMockCacheInterface[*prov.Cluster](ctrl)
	cache.EXPECT().GetByIndex(cluster.ByCloudCred, credID).Return(nil, nil)
	cache.EXPECT().GetByIndex(cluster.ByMachinePoolCloudCred, credID).Return(nil, nil)

	expectedErr := fmt.Errorf("inner-fail")

	inner := &fakeInnerStore{err: expectedErr}
	s := &Store{
		Store:            inner,
		ProvClusterCache: cache,
	}

	_, err := s.Delete(nil, nil, credID)
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
	assert.True(t, inner.deleteCalled)
}
