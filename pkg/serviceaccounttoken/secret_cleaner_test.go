package serviceaccounttoken

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCleanServiceAccountSecrets(t *testing.T) {
	var secrets []*corev1.Secret
	for i := range 7 {
		secrets = append(secrets, newSecret(fmt.Sprintf("test-%v", i)))
	}
	k8sClient := fake.NewSimpleClientset(toRuntimeObjects(secrets)...)

	err := CleanServiceAccountSecrets(
		context.Background(),
		k8sClient.CoreV1().Secrets(impersonationNamespace),
		k8sClient.CoreV1().ServiceAccounts(impersonationNamespace), secrets[0:5])
	require.NoError(t, err)

	secretList, err := k8sClient.CoreV1().Secrets(impersonationNamespace).List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)

	require.Equal(t, 2, len(secretList.Items))
}

func TestCleanServiceAccountSecretsDoesNotRemoveNonServiceAccountSecrets(t *testing.T) {
	var secrets []*corev1.Secret
	for i := range 7 {
		secrets = append(secrets, newSecret(fmt.Sprintf("test-%v", i), withNoLabels))
	}
	k8sClient := fake.NewSimpleClientset(toRuntimeObjects(secrets)...)

	err := CleanServiceAccountSecrets(
		context.Background(),
		k8sClient.CoreV1().Secrets(impersonationNamespace),
		k8sClient.CoreV1().ServiceAccounts(impersonationNamespace), secrets[0:5])
	require.NoError(t, err)

	secretList, err := k8sClient.CoreV1().Secrets(impersonationNamespace).List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)

	require.Equal(t, 2, len(secretList.Items))
}

func TestCleanServiceAccountSecretsDoesNotRemoveReferencedSecrets(t *testing.T) {
	var secrets []*corev1.Secret
	var resources []runtime.Object
	for i := range 4 {
		s := newSecret(fmt.Sprintf("test-%v", i))
		secrets = append(secrets, s)
		resources = append(resources, s)
	}

	sa := newServiceAccount("test-sa")
	resources = append(resources, newSecret(fmt.Sprintf("test-%v", 4), referencingSA(sa)), sa)

	k8sClient := fake.NewSimpleClientset(resources...)

	err := CleanServiceAccountSecrets(
		context.Background(),
		k8sClient.CoreV1().Secrets(impersonationNamespace),
		k8sClient.CoreV1().ServiceAccounts(impersonationNamespace), secrets)
	require.NoError(t, err)

	secretList, err := k8sClient.CoreV1().Secrets(impersonationNamespace).List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)

	require.Equal(t, 1, len(secretList.Items))
}

func TestStartServiceAccountSecretCleaner(t *testing.T) {
	oldDelay := cleanCycleDelay
	oldBatchSize := cleaningBatchSize

	t.Cleanup(func() {
		cleanCycleDelay = oldDelay
		cleaningBatchSize = oldBatchSize
	})

	cleanCycleDelay = 500 * time.Millisecond
	cleaningBatchSize = 5

	var secrets []runtime.Object
	for i := range 7 {
		secrets = append(secrets, newSecret(fmt.Sprintf("test-%v", i)))
	}

	ctx, cancel := context.WithCancel(context.Background())
	k8sClient := fake.NewSimpleClientset(secrets...)
	StartServiceAccountSecretCleaner(ctx, k8sClient.CoreV1())
	<-time.After(time.Second)
	cancel()

	secretList, err := k8sClient.CoreV1().Secrets(impersonationNamespace).List(
		context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Equal(t, 2, len(secretList.Items))
}

func withNoLabels(s *corev1.Secret) {
	s.ObjectMeta.Labels = map[string]string{}
}

func referencingSA(sa *corev1.ServiceAccount) func(*corev1.Secret) {
	return func(s *corev1.Secret) {
		if sa.Annotations == nil {
			sa.Annotations = map[string]string{}
		}

		secretAnnotation := s.Namespace + "/" + s.Name
		sa.Annotations[ServiceAccountSecretRefAnnotation] = secretAnnotation
	}
}

func newSecret(saName string, opts ...func(s *corev1.Secret)) *corev1.Secret {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.SimpleNameGenerator.GenerateName("test-token-"),
			Namespace: impersonationNamespace,
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": saName,
			},
			Labels: map[string]string{
				"cattle.io/service-account.name": saName,
			},
		},
		Data: map[string][]byte{
			"token": []byte("abcde"),
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func newServiceAccount(name string, opts ...func(s *corev1.ServiceAccount)) *corev1.ServiceAccount {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: impersonationNamespace,
			Labels: map[string]string{
				"authz.cluster.cattle.io/impersonator": "true",
			},
		},
	}

	for _, opt := range opts {
		opt(sa)
	}

	return sa
}

func toRuntimeObjects(secrets []*corev1.Secret) []runtime.Object {
	var result []runtime.Object
	for _, s := range secrets {
		result = append(result, s)
	}

	return result
}
