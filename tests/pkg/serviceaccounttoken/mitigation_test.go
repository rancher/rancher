package serviceaccounttoken

import (
	"context"
	"slices"
	"sort"
	"sync"
	"testing"

	"github.com/rancher/rancher/pkg/serviceaccounttoken"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// This tests that creation of secrets for SAs doesn't result in lots of secrets
// being created.
//
// This uses envtest (which uses the kube-apiserver and etcd) to test the
// functionality.
//
// Additional tests should be run within the envtest context. You need to be
// careful around removing resources.
//
// This spawns a goroutine that fakes the Kubernetes controller that updates
// Service Account secrets with a fake token.
func TestOptimisticLocking(t *testing.T) {
	testEnv := &envtest.Environment{}
	restCfg, err := testEnv.Start()
	assert.NoError(t, err)
	defer testEnv.Stop()

	clientSet, err := kubernetes.NewForConfig(restCfg)
	assert.NoError(t, err)
	k8sClient := clientSet.CoreV1()

	t.Run("creating a secret", func(t *testing.T) {
		sa, err := k8sClient.ServiceAccounts("default").Create(context.TODO(),
			&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sa",
					Namespace: "default",
				},
			}, metav1.CreateOptions{})
		assert.NoError(t, err)

		defer func() {
			err = k8sClient.ServiceAccounts("default").Delete(context.TODO(), sa.GetName(), metav1.DeleteOptions{})
			assert.NoError(t, err)
		}()

		ctx := context.Background()

		cancelCtx, cancel := context.WithCancel(ctx) // used to shutdown the goroutine
		go fakePopulateSecret(cancelCtx, t, k8sClient, sa)
		defer cancel()

		var wg sync.WaitGroup
		createdSecrets := make(chan corev1.Secret, 10)
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				secret, err := serviceaccounttoken.EnsureSecretForServiceAccount(context.TODO(), nil, clientSet, sa)
				assert.NoError(t, err)
				assert.NotNil(t, secret)
				createdSecrets <- *secret
			}()
		}
		wg.Wait()

		secrets, err := k8sClient.Secrets("default").List(context.TODO(), metav1.ListOptions{})
		assert.NoError(t, err)
		assert.Len(t, secrets.Items, 1)
		createdSecret := &secrets.Items[0]

		sa, err = k8sClient.ServiceAccounts("default").Get(context.TODO(), sa.GetName(), metav1.GetOptions{})
		assert.NoError(t, err)

		assert.Equal(t, sa.Annotations[serviceaccounttoken.ServiceAccountSecretRefAnnotation], keyFromObject(createdSecret).String())
		assert.Equal(t, string(createdSecret.Data["token"]), "this-is-not-a-real-key")

		var createdUIDs []string
		for i := 0; i < 10; i++ {
			c := <-createdSecrets
			if c.ObjectMeta.UID != "" {
				createdUIDs = append(createdUIDs, string(c.ObjectMeta.UID))
			}
		}
		sort.Strings(createdUIDs)
		uniqueUIDs := slices.Compact(createdUIDs)
		assert.Equal(t, 1, len(uniqueUIDs))
	})
}

// this is a fake reconciler for ServiceAccount secrets.
func fakePopulateSecret(ctx context.Context, t *testing.T, k8sClient typedcorev1.CoreV1Interface, sa *corev1.ServiceAccount) {
	t.Helper()
	watcher, err := k8sClient.Secrets("default").Watch(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(map[string]string{serviceaccounttoken.ServiceAccountSecretLabel: sa.GetName()}).String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Stop()

	for {
		select {
		case e, ok := <-watcher.ResultChan():
			if !ok {
				return
			}

			switch e.Type {
			case watch.Added:
				secret := e.Object.(*corev1.Secret).DeepCopy()
				if secret.ObjectMeta.UID == "" {
					continue
				}
				secret.Data = map[string][]byte{
					corev1.ServiceAccountTokenKey: []byte("this-is-not-a-real-key"),
				}
				_, err = k8sClient.Secrets("default").Update(context.TODO(), secret, metav1.UpdateOptions{})
				if err != nil {
					if !apierrors.IsGone(err) {
						t.Logf("error updating Secret %s: %s", keyFromObject(secret), err)
					}
				}
			case watch.Error:
				t.Logf("got an error in the secret populator: %s", err)
				return
			}
		case <-ctx.Done():
			t.Log("shutting down fake secret populator")
			return
		}

	}
}

func keyFromObject(obj interface {
	GetNamespace() string
	GetName() string
}) types.NamespacedName {
	return types.NamespacedName{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}
}
