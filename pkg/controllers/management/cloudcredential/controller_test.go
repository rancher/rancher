package cloudcredential

import (
	"errors"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"testing"
	"time"

	fapply "github.com/rancher/wrangler/v3/pkg/apply/fake"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
)

func TestSyncHarvesterToken(t *testing.T) {
	now := time.Now()
	type test struct {
		name string

		secret   *corev1.Secret
		expected *corev1.Secret

		token *v3.Token

		wantErr bool

		before func(*test)

		_secrets *fake.MockClientInterface[*corev1.Secret, *corev1.SecretList]
		_tokens  *fake.MockNonNamespacedClientInterface[*v3.Token, *v3.TokenList]
	}
	tests := []test{
		{
			name: "nil secret",
		},
		{
			name: "deleting and finalizer removed",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:         "cattle-global-data",
					Name:              "test",
					DeletionTimestamp: ptr.To(metav1.NewTime(now)),
				},
			},
		},
		{
			name: "no data",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "cattle-global-data",
					Name:      "test",
				},
			},
		},
		{
			name: "empty data",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "cattle-global-data",
					Name:      "test",
				},
				Data: map[string][]byte{
					"harvestercredentialConfig-kubeconfigContent": {},
				},
			},
		},
		{
			name: "already has annotation",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "cattle-global-data",
					Name:      "test",
					Annotations: map[string]string{
						"management.cattle.io/harvester-token-applied": "true",
					},
				},
			},
		},
		{
			name: "invalid prefix",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   "cattle-global-data",
					Name:        "test",
					Annotations: map[string]string{},
				},
				Data: map[string][]byte{
					"harvestercredentialConfig-kubeconfigContent": []byte("users:\n- user:\n    token: test-token"),
				},
			},
			before: func(tt *test) {
				tt._secrets.EXPECT().Update(gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
					return secret, nil
				}).Times(2)

			},
			expected: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "cattle-global-data",
					Name:      "test",
					Annotations: map[string]string{
						"management.cattle.io/harvester-token-checksum": "8c9a257f54763d4f3a1b02c148d9faf505c3be7f5726b27f17df5063c6fbcd7f",
					},
					Finalizers: []string{
						"management.cattle.io/harvester-token-cleanup",
					},
				},
				Data: map[string][]byte{
					"harvestercredentialConfig-kubeconfigContent": []byte("users:\n- user:\n    token: test-token"),
				},
			},
		},
		{
			name: "cannot get token",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   "cattle-global-data",
					Name:        "test",
					Annotations: map[string]string{},
					Finalizers: []string{
						"management.cattle.io/harvester-token-cleanup",
					},
				},
				Data: map[string][]byte{
					"harvestercredentialConfig-kubeconfigContent": []byte("users:\n- user:\n    token: kubeconfig-u-test:abcdef"),
				},
			},
			wantErr: true,
			before: func(tt *test) {
				tt._tokens.EXPECT().Get("kubeconfig-u-test", metav1.GetOptions{}).Return(nil, apierrors.NewNotFound(schema.GroupResource{}, "kubeconfig-u-test")).Times(1)
			},
		},
		{
			name: "fail to add finalizer",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "cattle-global-data",
					Name:      "test",
				},
				Data: map[string][]byte{
					"harvestercredentialConfig-kubeconfigContent": []byte("users:\n- user:\n    token: kubeconfig-u-test:abcdef"),
				},
			},
			wantErr: true,
			before: func(tt *test) {
				tt._secrets.EXPECT().Update(gomock.AssignableToTypeOf(&corev1.Secret{})).Return(nil, apierrors.NewInternalError(errors.New("error"))).Times(1)
			},
		},
		{
			name: "fail to add annotation",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   "cattle-global-data",
					Name:        "test",
					Annotations: map[string]string{},
				},
				Data: map[string][]byte{
					"harvestercredentialConfig-kubeconfigContent": []byte("users:\n- user:\n    token: kubeconfig-u-test:abcdef"),
				},
			},
			token: &v3.Token{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kubeconfig-u-test",
				},
				Token:     "test-token",
				TTLMillis: 1000,
			},
			wantErr: true,
			before: func(tt *test) {
				tt._tokens.EXPECT().Get("kubeconfig-u-test", metav1.GetOptions{}).Return(tt.token, nil).Times(1)
				tt._secrets.EXPECT().Update(gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(obj *corev1.Secret) (*corev1.Secret, error) {
					return obj, nil
				}).Times(1)
				tt._secrets.EXPECT().Update(gomock.AssignableToTypeOf(&corev1.Secret{})).Return(nil, apierrors.NewInternalError(errors.New("error"))).Times(1)
			},
		},
		{
			name: "success",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   "cattle-global-data",
					Name:        "test",
					Annotations: map[string]string{},
				},
				Data: map[string][]byte{
					"harvestercredentialConfig-kubeconfigContent": []byte("users:\n- user:\n    token: kubeconfig-u-test:abcdef"),
				},
			},
			expected: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "cattle-global-data",
					Name:      "test",
					Annotations: map[string]string{
						"management.cattle.io/harvester-token-checksum": "380a5176e6ba7262e104bfbcf4b2617b4125d0eedfa2df8d5c16f54ffbc46dd6",
					},
					Finalizers: []string{"management.cattle.io/harvester-token-cleanup"},
				},
				Data: map[string][]byte{
					"harvestercredentialConfig-kubeconfigContent": []byte("users:\n- user:\n    token: kubeconfig-u-test:abcdef"),
				},
			},
			token: &v3.Token{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kubeconfig-u-test",
				},
				Token:     "test-token",
				TTLMillis: 1000,
			},
			before: func(tt *test) {
				tt._tokens.EXPECT().Get("kubeconfig-u-test", metav1.GetOptions{}).Return(tt.token, nil).Times(1)
				tt._secrets.EXPECT().Update(gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(obj *corev1.Secret) (*corev1.Secret, error) {
					return obj, nil
				}).Times(2)
			},
		},
		{
			name: "has finalizer but not annotation",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   "cattle-global-data",
					Name:        "test",
					Annotations: map[string]string{},
					Finalizers:  []string{"management.cattle.io/harvester-token-cleanup"},
				},
				Data: map[string][]byte{
					"harvestercredentialConfig-kubeconfigContent": []byte("users:\n- user:\n    token: kubeconfig-u-test:abcdef"),
				},
			},
			expected: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "cattle-global-data",
					Name:      "test",
					Annotations: map[string]string{
						"management.cattle.io/harvester-token-checksum": "380a5176e6ba7262e104bfbcf4b2617b4125d0eedfa2df8d5c16f54ffbc46dd6",
					},
					Finalizers: []string{"management.cattle.io/harvester-token-cleanup"},
				},
				Data: map[string][]byte{
					"harvestercredentialConfig-kubeconfigContent": []byte("users:\n- user:\n    token: kubeconfig-u-test:abcdef"),
				},
			},
			token: &v3.Token{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kubeconfig-u-test",
				},
				Token:     "test-token",
				TTLMillis: 1000,
			},
			before: func(tt *test) {
				tt._tokens.EXPECT().Get("kubeconfig-u-test", metav1.GetOptions{}).Return(tt.token, nil).Times(1)
				tt._secrets.EXPECT().Update(gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(obj *corev1.Secret) (*corev1.Secret, error) {
					return obj, nil
				}).Times(1)
			},
		},
		{
			name: "successful removal",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:         "cattle-global-data",
					Name:              "test",
					Finalizers:        []string{"management.cattle.io/harvester-token-cleanup"},
					DeletionTimestamp: ptr.To(metav1.NewTime(now)),
				},
				Data: map[string][]byte{
					"harvestercredentialConfig-kubeconfigContent": []byte("users:\n- user:\n    token: kubeconfig-u-test:abcdef"),
				},
			},
			expected: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:         "cattle-global-data",
					Name:              "test",
					Finalizers:        []string{},
					DeletionTimestamp: ptr.To(metav1.NewTime(now)),
				},
				Data: map[string][]byte{
					"harvestercredentialConfig-kubeconfigContent": []byte("users:\n- user:\n    token: kubeconfig-u-test:abcdef"),
				},
			},
			token: &v3.Token{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kubeconfig-u-test",
				},
				Token:     "test-token",
				TTLMillis: 0,
			},
			before: func(tt *test) {
				tt._secrets.EXPECT().Update(gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(obj *corev1.Secret) (*corev1.Secret, error) {
					return obj, nil
				}).Times(1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			tt._secrets = fake.NewMockClientInterface[*corev1.Secret, *corev1.SecretList](ctrl)
			tt._tokens = fake.NewMockNonNamespacedClientInterface[*v3.Token, *v3.TokenList](ctrl)
			apply := &fapply.FakeApply{}

			c := Controller{
				secretClient: tt._secrets,
				tokenClient:  tt._tokens,
				apply:        apply,
			}

			if tt.before != nil {
				tt.before(&tt)
			}

			result, err := c.syncHarvesterToken("", tt.secret.DeepCopy())
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				if tt.expected != nil {
					assert.Equal(t, tt.expected, result)
				} else {
					assert.Equal(t, tt.secret, result)
				}
			}
		})
	}
}
