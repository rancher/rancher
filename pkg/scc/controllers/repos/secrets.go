package repos

import (
	jsonpatch "github.com/evanphx/json-patch/v5"
	wranglercorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/util/retry"
)

type SecretRepo struct {
	Secrets      wranglercorev1.SecretController
	SecretsCache wranglercorev1.SecretCache
}

func (sr *SecretRepo) PatchUpdate(incoming, desired *corev1.Secret) (*corev1.Secret, error) {
	incomingJson, err := json.Marshal(incoming)
	if err != nil {
		return incoming, err
	}
	newJson, err := json.Marshal(desired)
	if err != nil {
		return incoming, err
	}

	patch, err := jsonpatch.CreateMergePatch(incomingJson, newJson)
	if err != nil {
		return incoming, err
	}
	updated, err := sr.Secrets.Patch(incoming.Namespace, incoming.Name, types.MergePatchType, patch)
	if err != nil {
		return incoming, err
	}

	return updated, nil
}

// RetryingPatchUpdate wraps PatchUpdate in logic to retry if it hits a conflict on first patch
func (sr *SecretRepo) RetryingPatchUpdate(incoming, desired *corev1.Secret) (*corev1.Secret, error) {
	initialPatched, err := sr.PatchUpdate(incoming, desired)
	if err == nil {
		return initialPatched, nil
	}

	var updated *corev1.Secret
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		currentSecret, getErr := sr.Secrets.Get(incoming.Namespace, incoming.Name, metav1.GetOptions{})
		if getErr != nil && !apierrors.IsNotFound(getErr) {
			return getErr
		}

		var updateErr error
		updated, updateErr = sr.PatchUpdate(currentSecret, desired)
		return updateErr
	})

	return updated, retryErr
}

func (sr *SecretRepo) CreateOrUpdateSecret(secret *corev1.Secret) (*corev1.Secret, error) {
	existingSecret, getErr := sr.SecretsCache.Get(secret.Namespace, secret.Name)
	if getErr != nil && apierrors.IsNotFound(getErr) {
		return sr.Secrets.Create(secret)
	}

	return sr.RetryingPatchUpdate(existingSecret, secret)
}
