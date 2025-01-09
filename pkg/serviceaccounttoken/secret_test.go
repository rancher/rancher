package serviceaccounttoken

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestEnsureSecretForServiceAccount(t *testing.T) {
	t.Parallel()
	defaultWantSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
			Annotations: map[string]string{
				"rancher.io/service-account.secret-ref": "default/test-token-abcde",
			},
		},
	}
	defaultWantSecret := &corev1.Secret{
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
		Type: corev1.SecretTypeServiceAccountToken,
	}
	tests := []struct {
		name           string
		sa             *corev1.ServiceAccount
		wantSA         *corev1.ServiceAccount
		existingSecret *corev1.Secret
		wantSecret     *corev1.Secret
		wantErr        bool
	}{
		{
			name: "service account with no secret generates secret",
			sa: &corev1.ServiceAccount{
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
			sa: &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
			},
			wantSA: &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
					// Has no secret annotation.
				},
			},
			existingSecret: &corev1.Secret{
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
				Type: corev1.SecretTypeServiceAccountToken,
			},
			wantSecret: defaultWantSecret,
		},
		{
			name:    "returns error for nil service account",
			wantErr: true,
		},
		{
			name: "service account with invalid secret is updated with new secret",
			sa: &corev1.ServiceAccount{
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
			sa: &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
			},
			wantSA: defaultWantSA,
			existingSecret: &corev1.Secret{
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
				Type: corev1.SecretTypeOpaque,
			},
			wantSecret: defaultWantSecret,
		},
		{
			name: "secret for wrong service account type gets recreated",
			sa: &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
			},
			wantSA: defaultWantSA,
			existingSecret: &corev1.Secret{
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
				Type: corev1.SecretTypeServiceAccountToken,
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
					ret := action.(k8stesting.CreateAction).GetObject().(*corev1.Secret)
					ret.ObjectMeta.Name = ret.GenerateName + "abcde"
					ret.Data = map[string][]byte{
						"token": []byte("abcde"),
					}

					return true, ret, nil
				},
			)
			got, gotErr := EnsureSecretForServiceAccount(context.Background(), nil, k8sClient, tt.sa.DeepCopy())
			if tt.wantErr {
				assert.Error(t, gotErr)
				return
			}
			assert.NoError(t, gotErr)
			assert.Equal(t, tt.wantSecret.Name, got.Name)
			gotSA, _ := k8sClient.CoreV1().ServiceAccounts(tt.sa.Namespace).Get(context.Background(), tt.sa.Name, metav1.GetOptions{})
			assert.Equal(t, tt.wantSA, gotSA)
		})
	}
}

func TestServiceAccountSecret(t *testing.T) {
	type testState struct {
		clientset  *fake.Clientset
		fakeLister *fakeSecretLister
	}
	baseSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sa",
			Namespace: "test-ns",
		},
	}
	annotatedSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "annotated-sa",
			Namespace: "test-ns",
			Annotations: map[string]string{
				ServiceAccountSecretRefAnnotation: "test-ns/annotated-sa-secret",
			},
		},
	}
	validSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "base-sa-secret",
			Namespace: "test-ns",
			Labels: map[string]string{
				ServiceAccountSecretLabel: baseSA.Name,
			},
			Annotations: map[string]string{
				serviceAccountSecretAnnotation: baseSA.Name,
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}
	invalidSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "base-sa-secret",
			Namespace: "test-ns",
			Labels: map[string]string{
				ServiceAccountSecretLabel: baseSA.Name,
			},
			Annotations: map[string]string{
				serviceAccountSecretAnnotation: baseSA.Name,
			},
		},
		Type: corev1.SecretTypeOpaque,
	}

	referencedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "annotated-sa-secret",
			Namespace: "test-ns",
			Labels: map[string]string{
				ServiceAccountSecretLabel: annotatedSA.Name,
			},
			Annotations: map[string]string{
				serviceAccountSecretAnnotation: annotatedSA.Name,
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}
	tests := []struct {
		name             string
		stateSetup       func(testState)
		inputSA          *corev1.ServiceAccount
		wantSecret       *corev1.Secret
		remainingSecrets []*corev1.Secret
		wantError        bool
	}{
		{
			name:      "test nil sa",
			inputSA:   nil,
			wantError: true,
		},
		{
			name:    "test SA annotated with secret - secret exists",
			inputSA: annotatedSA,
			stateSetup: func(ts testState) {
				ts.clientset.Tracker().Add(referencedSecret)
			},
			wantSecret:       referencedSecret,
			remainingSecrets: []*corev1.Secret{referencedSecret},
		},
		{
			name:    "test SA annotated with secret - secret does not exist",
			inputSA: annotatedSA,
			stateSetup: func(ts testState) {
				ts.fakeLister.secrets = []*corev1.Secret{referencedSecret}
				// No secrets in the clientset
			},
			wantSecret: referencedSecret,
		},
		{
			name:    "test SA not annotated with secret but valid secret available",
			inputSA: baseSA,
			stateSetup: func(ts testState) {
				ts.fakeLister.secrets = []*corev1.Secret{validSecret}
				ts.clientset.Tracker().Add(validSecret)
			},
			wantSecret:       validSecret,
			remainingSecrets: []*corev1.Secret{referencedSecret},
		},
		{
			name:    "test SA not annotated with secret and no secrets",
			inputSA: baseSA,
		},
		{
			name:    "test SA not annotated with secret and no valid secrets",
			inputSA: baseSA,
			stateSetup: func(ts testState) {
				ts.fakeLister.secrets = []*corev1.Secret{invalidSecret}
				ts.clientset.Tracker().Add(invalidSecret)
			},
		},
		{
			name:    "test secret lister error",
			inputSA: baseSA,
			stateSetup: func(ts testState) {
				ts.fakeLister.secrets = []*corev1.Secret{invalidSecret}
				ts.fakeLister.err = fmt.Errorf("server unavailable")
			},
			wantError: true,
		},
		{
			name:    "test SA with no valid secrets removes additional secrets",
			inputSA: baseSA,
			stateSetup: func(ts testState) {
				ts.fakeLister.secrets = []*corev1.Secret{validSecret, invalidSecret}
				ts.clientset.Tracker().Add(validSecret)
				ts.clientset.Tracker().Add(invalidSecret)
			},
			wantSecret:       validSecret,
			remainingSecrets: []*corev1.Secret{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := testState{
				clientset:  fake.NewSimpleClientset(),
				fakeLister: &fakeSecretLister{},
			}
			if test.stateSetup != nil {
				test.stateSetup(state)
			}
			secretsMock := state.clientset.CoreV1().Secrets("test-ns")
			secret, err := ServiceAccountSecret(context.Background(), test.inputSA, state.fakeLister.list, secretsMock)
			require.Equal(t, test.wantSecret, secret)
			if test.wantError {
				require.NotNil(t, err)
			} else {
				require.Nil(t, err)
			}
			secrets, err := secretsMock.List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			require.Equal(t, len(test.remainingSecrets), len(secrets.Items))
		})
	}
}

type fakeSecretLister struct {
	secrets []*corev1.Secret
	err     error
}

func (f *fakeSecretLister) list(namespace string, selector labels.Selector) ([]*corev1.Secret, error) {
	return f.secrets, f.err
}
