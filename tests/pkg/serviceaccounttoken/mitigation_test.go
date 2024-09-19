package serviceaccounttoken

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

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
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// Sends a DeepCopy because this is theoretically being updated
				// in different processes.
				secret, err := EnsureSecretForServiceAccount(context.TODO(), nil, clientSet, sa.DeepCopy())
				assert.NoError(t, err)
				assert.NotNil(t, secret)
			}()
		}
		wg.Wait()

		secrets, err := k8sClient.Secrets("default").List(context.TODO(), metav1.ListOptions{})
		assert.NoError(t, err)
		assert.Len(t, secrets.Items, 1)
	})
}

// this is a fake reconciler for ServiceAccount secrets.
func fakePopulateSecret(ctx context.Context, t *testing.T, k8sClient typedcorev1.CoreV1Interface, sa *corev1.ServiceAccount) {
	watcher, err := k8sClient.Secrets("default").Watch(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(map[string]string{ServiceAccountSecretLabel: sa.GetName()}).String(),
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
			if e.Type == watch.Error {
				t.Logf("got an error in the secret populator: %s", err)
				// TODO: log out the error? // apierrors.FromObject?!
				return
			}

			switch e.Type {
			case watch.Added:
				secret := e.Object.(*corev1.Secret)
				secret.Data = map[string][]byte{
					corev1.ServiceAccountTokenKey: []byte("this-is-not-a-real-key"),
				}
				_, err = k8sClient.Secrets("default").Update(context.TODO(), secret, metav1.UpdateOptions{})
				if err != nil {
					t.Logf("error deleting %s: %s", keyFromObject(secret), err)
				}
			}
		case <-ctx.Done():
			t.Log("shutting down fake secret populator")
			return
		}

	}
}
