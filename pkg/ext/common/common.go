package stores

import (
	"fmt"
	"time"

	v1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// EnsureNamespace tries to ensure that the namespace exists.
func EnsureNamespace(nsCache v1.NamespaceCache, nsClient v1.NamespaceClient, name string) error {
	var backoff = wait.Backoff{
		Duration: 100 * time.Millisecond,
		Factor:   2,
		Jitter:   .2,
		Steps:    7,
	}

	return wait.ExponentialBackoff(backoff, func() (bool, error) {
		_, err := nsCache.Get(name)
		if err == nil {
			return true, nil
		}

		if !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("error getting namespace %s: %w", name, err)
		}

		_, err = nsClient.Create(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return false, fmt.Errorf("error creating namespace %s: %w", name, err)
		}

		return true, nil
	})
}
