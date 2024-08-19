package serviceaccounttoken

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestEnsureSecretForServiceAccount(t *testing.T) {
	t.Parallel()
	defaultWantSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
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
			wantSA: defaultWantSA,
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
			got, gotErr := EnsureSecretForServiceAccount(context.Background(), nil, k8sClient, tt.sa, "")
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

func TestServiceAccountSecret(t *testing.T) {
	type testState struct {
		clientset  *fake.Clientset
		fakeLister *fakeSecretLister
	}
	baseSA := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "base-sa",
			Namespace: "test-ns",
		},
	}
	validSecret := corev1.Secret{
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
	invalidSecretType := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "invalid-secret-type",
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
	invalidSecretAnnotation := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "invalid-secret-annotation",
			Namespace: "test-ns",
			Labels: map[string]string{
				ServiceAccountSecretLabel: baseSA.Name,
			},
			Annotations: map[string]string{
				serviceAccountSecretAnnotation: "some-other-sa",
			},
		},
		Type: corev1.SecretTypeOpaque,
	}
	tests := []struct {
		name       string
		stateSetup func(testState)
		inputSA    *corev1.ServiceAccount
		wantSecret *corev1.Secret
		wantError  bool
	}{
		{
			name:      "test nil sa",
			inputSA:   nil,
			wantError: true,
		},
		{
			name:       "test no secrets",
			inputSA:    &baseSA,
			wantError:  false,
			wantSecret: nil,
		},
		{
			name:    "test valid secrets, first returned",
			inputSA: &baseSA,
			stateSetup: func(ts testState) {
				validSecondSecret := validSecret.DeepCopy()
				validSecondSecret.Name = "base-sa-secret-2"
				ts.fakeLister.secrets = []*corev1.Secret{&validSecret, validSecondSecret}
			},
			wantError:  false,
			wantSecret: &validSecret,
		},
		{
			name:    "test invalid secrets, none returned",
			inputSA: &baseSA,
			stateSetup: func(ts testState) {
				ts.fakeLister.secrets = []*corev1.Secret{&invalidSecretType, &invalidSecretAnnotation}
				ts.clientset.Tracker().Add(&invalidSecretType)
				ts.clientset.Tracker().Add(&invalidSecretAnnotation)
			},
			wantError:  false,
			wantSecret: nil,
		},
		{
			name:    "test invalid secrets delete failure, valid still returned",
			inputSA: &baseSA,
			stateSetup: func(ts testState) {
				ts.fakeLister.secrets = []*corev1.Secret{&invalidSecretType, &invalidSecretAnnotation, &validSecret}
				ts.clientset.Tracker().Add(&invalidSecretType)
				// don't add the invalid annotation secret to the state, this will cause a not-found error on delete
			},
			wantError:  false,
			wantSecret: &validSecret,
		},
		{
			name:    "test valid + invalid secrets, only valid returned",
			inputSA: &baseSA,
			stateSetup: func(ts testState) {
				ts.fakeLister.secrets = []*corev1.Secret{&invalidSecretType, &invalidSecretAnnotation, &validSecret}
				ts.clientset.Tracker().Add(&invalidSecretType)
				ts.clientset.Tracker().Add(&invalidSecretAnnotation)
			},
			wantError:  false,
			wantSecret: &validSecret,
		},
		{
			name:    "test secret lister error",
			inputSA: &baseSA,
			stateSetup: func(ts testState) {
				ts.fakeLister.secrets = []*corev1.Secret{&invalidSecretType, &invalidSecretAnnotation, &validSecret}
				ts.fakeLister.err = fmt.Errorf("server unavailable")
			},
			wantError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			k8sClient := fake.NewSimpleClientset()
			fakeLister := fakeSecretLister{}
			state := testState{
				clientset:  k8sClient,
				fakeLister: &fakeLister,
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

func TestEnsureSecretForServiceAccount_in_parallel(t *testing.T) {
	k8sClient := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret-1",
			Namespace: "default",
			Labels: map[string]string{
				ServiceAccountSecretLabel: "test",
			},
		},
		Type: corev1.SecretTypeOpaque,
	},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret-2",
				Namespace: "default",
				Labels: map[string]string{
					ServiceAccountSecretLabel: "test",
				},
			},
			Type: corev1.SecretTypeOpaque,
		},
	)
	var m sync.Mutex
	var created []*corev1.Secret

	k8sClient.PrependReactor("list", "secrets",
		func(a k8stesting.Action) (bool, runtime.Object, error) {
			m.Lock()
			defer m.Unlock()

			secrets := &corev1.SecretList{}
			for _, v := range created {
				secrets.Items = append(secrets.Items, *v)
			}

			return true, secrets, nil
		},
	)

	k8sClient.PrependReactor("create", "secrets",
		func(action k8stesting.Action) (bool, runtime.Object, error) {
			m.Lock()
			defer m.Unlock()
			ret := action.(k8stesting.CreateAction).GetObject().(*corev1.Secret)
			ret.ObjectMeta.Name = ret.GenerateName + rand.String(5)
			ret.ObjectMeta.Labels = map[string]string{
				ServiceAccountSecretLabel: "test", // sa.Name from below
			}
			ret.ObjectMeta.Annotations = map[string]string{
				serviceAccountSecretAnnotation: "test", // sa.Name from below
			}
			ret.Data = map[string][]byte{
				"token": []byte("abcde"),
			}
			created = append(created, ret)

			return true, ret, nil
		},
	)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := EnsureSecretForServiceAccount(context.Background(), nil, k8sClient, &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
			}, "cluster-1-")
			assert.NoError(t, err)
		}()
	}

	wg.Wait()
	if l := len(created); l != 1 {
		t.Errorf("EnsureSecretForServiceAccount() created %d secrets, want 1", l)
	}

	var remaining []string
	lockMap.Range(func(key, value any) bool {
		remaining = append(remaining, fmt.Sprintf("%v", key))
		return true
	})
	assert.Empty(t, remaining)

	remainingSecrets, err := k8sClient.CoreV1().Secrets("default").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	assert.Len(t, remainingSecrets.Items, 1)
}
