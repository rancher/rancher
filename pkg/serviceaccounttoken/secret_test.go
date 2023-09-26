package serviceaccounttoken

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestEnsureSecretForServiceAccount(t *testing.T) {
	t.Parallel()
	defaultWantSA := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
	}
	defaultWantSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-token-abcde",
			Namespace: "default",
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": "test",
			},
			Labels: map[string]string{
				"cattle.io/service-account.name": "test",
			},
		},
		Data: map[string][]byte{
			"token": []byte("abcde"),
		},
		Type: v1.SecretTypeServiceAccountToken,
	}
	tests := []struct {
		name           string
		sa             *v1.ServiceAccount
		wantSA         *v1.ServiceAccount
		existingSecret *v1.Secret
		wantSecret     *v1.Secret
		wantErr        bool
	}{
		{
			name: "service account with no secret generates secret",
			sa: &v1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
			},
			wantSA:     defaultWantSA,
			wantSecret: defaultWantSecret,
		},
		{
			name: "service account with existing secret returns it",
			sa: &v1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
			},
			wantSA: defaultWantSA,
			existingSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-token-abcde",
					Namespace: "default",
					Annotations: map[string]string{
						"kubernetes.io/service-account.name": "test",
					},
					Labels: map[string]string{
						"cattle.io/service-account.name": "test",
					},
				},
				Data: map[string][]byte{
					"token": []byte("abcde"),
				},
				Type: v1.SecretTypeServiceAccountToken,
			},
			wantSecret: defaultWantSecret,
		},
		{
			name:    "returns error for nil service account",
			wantErr: true,
		},
		{
			name: "service account with invalid secret is updated with new secret",
			sa: &v1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
			},
			wantSA:     defaultWantSA,
			wantSecret: defaultWantSecret,
		},
		{
			name: "secret of wrong type gets recreated",
			sa: &v1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
			},
			wantSA: defaultWantSA,
			existingSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-token-xyz",
					Namespace: "default",
					Annotations: map[string]string{
						"kubernetes.io/service-account.name": "test",
					},
					Labels: map[string]string{
						"cattle.io/service-account.name": "test",
					},
				},
				Data: map[string][]byte{
					"token": []byte("abcde"),
				},
				Type: v1.SecretTypeOpaque,
			},
			wantSecret: defaultWantSecret,
		},
		{
			name: "secret for wrong service account type gets recreated",
			sa: &v1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
			},
			wantSA: defaultWantSA,
			existingSecret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-token-xyz",
					Namespace: "default",
					Annotations: map[string]string{
						"kubernetes.io/service-account.name": "wrong",
					},
					Labels: map[string]string{
						"cattle.io/service-account.name": "wrong",
					},
				},
				Data: map[string][]byte{
					"token": []byte("abcde"),
				},
				Type: v1.SecretTypeServiceAccountToken,
			},
			wantSecret: defaultWantSecret,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var k8sClient *fake.Clientset
			objs := []runtime.Object{}
			if tt.sa != nil {
				objs = append(objs, tt.sa)
			}
			if tt.existingSecret != nil {
				objs = append(objs, tt.existingSecret)
			}
			k8sClient = fake.NewSimpleClientset(objs...)
			k8sClient.PrependReactor("create", "secrets",
				func(action k8stesting.Action) (bool, runtime.Object, error) {
					ret := action.(k8stesting.CreateAction).GetObject().(*v1.Secret)
					ret.ObjectMeta.Name = ret.GenerateName + "abcde"
					ret.Data = map[string][]byte{
						"token": []byte("abcde"),
					}

					return true, ret, nil
				},
			)
			got, gotErr := EnsureSecretForServiceAccount(context.Background(), nil, k8sClient, tt.sa)
			if tt.wantErr {
				assert.Error(t, gotErr)
				return
			}
			assert.NoError(t, gotErr)
			assert.Equal(t, tt.wantSecret.Name, got.Name)
			gotSA, _ := k8sClient.CoreV1().ServiceAccounts("default").Get(context.Background(), "test", metav1.GetOptions{})
			assert.Equal(t, tt.wantSA.Secrets, gotSA.Secrets)
			assert.Equal(t, tt.sa, gotSA)
		})
	}
}
