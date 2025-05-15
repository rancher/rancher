package cred

import (
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	corefakes "github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	v3fakes "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"testing"

	"github.com/stretchr/testify/assert"
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
