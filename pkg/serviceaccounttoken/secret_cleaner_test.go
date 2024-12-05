package serviceaccounttoken

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

func TestCleanServiceAccountSecrets(t *testing.T) {
	var secrets []*corev1.Secret
	totalSecrets := 7
	cleanedSecrets := 5

	for i := range totalSecrets {
		saName := fmt.Sprintf("test-%v", i)
		secrets = append(secrets, newSecret(saName, withLabels(map[string]string{
			"cattle.io/service-account.name": saName,
		})))
	}
	k8sClient := fake.NewSimpleClientset(toRuntimeObjects(secrets)...)
	stubServiceAccounts := newServiceAccountsFake()

	err := CleanServiceAccountSecrets(
		context.Background(),
		k8sClient.CoreV1().Secrets(impersonationNamespace),
		stubServiceAccounts,
		toNamespacedNames(secrets)[0:cleanedSecrets])
	require.NoError(t, err)

	secretList, err := k8sClient.CoreV1().Secrets(impersonationNamespace).List(
		context.Background(), metav1.ListOptions{})
	require.NoError(t, err)

	require.Equal(t, totalSecrets-cleanedSecrets, len(secretList.Items))
}

func TestCleanServiceAccountSecretsDoesNotRemoveReferencedSecrets(t *testing.T) {
	var secrets []*corev1.Secret
	totalSecrets := 4
	cleanedSecrets := 3
	var resources []runtime.Object
	for i := range totalSecrets {
		s := newSecret(fmt.Sprintf("test-%v", i))
		secrets = append(secrets, s)
		resources = append(resources, s)
	}
	sa := newServiceAccount("test-sa")
	referencedSecret := newSecret(fmt.Sprintf("test-%v", 4), referencingSA(sa))
	secrets = append(secrets, referencedSecret)

	resources = append(resources, referencedSecret, sa)

	k8sClient := fake.NewSimpleClientset(resources...)

	stubServiceAccounts := newServiceAccountsFake(sa)
	err := CleanServiceAccountSecrets(
		context.Background(),
		k8sClient.CoreV1().Secrets(impersonationNamespace),
		stubServiceAccounts,
		toNamespacedNames(secrets))
	require.NoError(t, err)

	secretList, err := k8sClient.CoreV1().Secrets(impersonationNamespace).List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)

	require.Equal(t, totalSecrets-cleanedSecrets, len(secretList.Items))
}

func TestStartServiceAccountSecretCleaner(t *testing.T) {
	oldDelay := cleanCycleDelay
	oldBatchSize := cleaningBatchSize
	totalSecrets := 7
	cleanedSecrets := 5

	t.Cleanup(func() {
		cleanCycleDelay = oldDelay
		cleaningBatchSize = oldBatchSize
	})

	cleanCycleDelay = 500 * time.Millisecond
	cleaningBatchSize = 5

	var secrets []runtime.Object
	for i := range totalSecrets {
		secrets = append(secrets, newSecret(fmt.Sprintf("test-%v", i)))
	}

	stubSecrets := newStubSecrets(secrets...)
	stubServiceAccounts := newServiceAccountsFake()

	ctx, cancel := context.WithCancel(context.Background())
	k8sClient := fake.NewSimpleClientset(secrets...)
	StartServiceAccountSecretCleaner(ctx, stubSecrets, stubServiceAccounts, k8sClient.CoreV1())
	<-time.After(time.Second)
	cancel()

	secretList, err := k8sClient.CoreV1().Secrets(impersonationNamespace).List(
		context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Equal(t, totalSecrets-cleanedSecrets, len(secretList.Items))
}

func TestStartServiceAccountSecretCleanerWhenDisabled(t *testing.T) {
	oldDelay := cleanCycleDelay
	oldBatchSize := cleaningBatchSize
	totalSecrets := 7

	t.Cleanup(func() {
		cleanCycleDelay = oldDelay
		cleaningBatchSize = oldBatchSize
	})

	cleanCycleDelay = 500 * time.Millisecond
	cleaningBatchSize = 5

	var secrets []runtime.Object
	for i := range totalSecrets {
		secrets = append(secrets, newSecret(fmt.Sprintf("test-%v", i)))
	}

	ctx, cancel := context.WithCancel(context.Background())
	k8sClient := fake.NewSimpleClientset(secrets...)

	existingState := features.CleanStaleSecrets.Enabled()
	t.Cleanup(func() {
		features.CleanStaleSecrets.Set(existingState)
	})
	features.CleanStaleSecrets.Set(false)

	stubSecrets := newStubSecrets(secrets...)
	stubServiceAccounts := newServiceAccountsFake()

	StartServiceAccountSecretCleaner(ctx, stubSecrets, stubServiceAccounts, k8sClient.CoreV1())
	<-time.After(time.Second)
	cancel()

	secretList, err := k8sClient.CoreV1().Secrets(impersonationNamespace).List(
		context.Background(), metav1.ListOptions{})
	require.NoError(t, err)

	// No secrets should be deleted.
	require.Equal(t, totalSecrets, len(secretList.Items))
}

func withLabels(l map[string]string) func(*corev1.Secret) {
	return func(s *corev1.Secret) {
		s.ObjectMeta.Labels = l
	}
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

func newStubSecrets(objs ...runtime.Object) *stubSecretLister {
	l := &stubSecretLister{}
	for _, s := range objs {
		secret := s.(*corev1.Secret)
		l.secrets = append(l.secrets, secret)
	}

	return l
}

type stubSecretLister struct {
	secrets []*corev1.Secret
}

func (l *stubSecretLister) List(namespace string, selector labels.Selector) ([]*corev1.Secret, error) {
	var result []*corev1.Secret
	for _, v := range l.secrets {
		if selector.Matches(labels.Set(v.ObjectMeta.Labels)) {
			result = append(result, v)
		}
	}

	return result, nil
}

func newServiceAccountsFake(objs ...runtime.Object) serviceAccountsCache {
	c := &fakeServiceAccountCache{
		store: cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{}),
	}
	setupServiceAccountsCache(c)

	for _, s := range objs {
		sa := s.(*corev1.ServiceAccount)
		c.store.Add(sa)
	}

	return c
}

func toNamespacedNames(secrets []*corev1.Secret) []types.NamespacedName {
	var nsns []types.NamespacedName
	for _, secret := range secrets {
		nsns = append(nsns, types.NamespacedName{Name: secret.GetName(), Namespace: secret.GetNamespace()})
	}

	return nsns
}

// This wraps the client-go cache in a wrangler Cache API.
type fakeServiceAccountCache struct {
	store cache.Indexer
}

func (c *fakeServiceAccountCache) GetByIndex(indexName, key string) ([]*corev1.ServiceAccount, error) {
	var result []*corev1.ServiceAccount
	raw, err := c.store.ByIndex(indexName, key)
	if err != nil {
		return nil, err
	}

	for _, v := range raw {
		result = append(result, v.(*corev1.ServiceAccount))
	}

	return result, nil
}

func (c *fakeServiceAccountCache) AddIndexer(indexName string, indexer generic.Indexer[*corev1.ServiceAccount]) {
	c.store.AddIndexers(map[string]cache.IndexFunc{
		indexName: func(obj interface{}) ([]string, error) {
			return indexer(obj.(*corev1.ServiceAccount))
		},
	})
}
