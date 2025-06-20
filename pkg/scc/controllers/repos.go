package controllers

import (
	jsonpatch "github.com/evanphx/json-patch/v5"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/util/retry"
)

func (h *handler) updateSecret(incoming, target *corev1.Secret) (*corev1.Secret, error) {
	incomingJson, err := json.Marshal(incoming)
	if err != nil {
		return incoming, err
	}
	newJson, err := json.Marshal(target)
	if err != nil {
		return incoming, err
	}

	patch, err := jsonpatch.CreateMergePatch(incomingJson, newJson)
	if err != nil {
		return incoming, err
	}
	if _, err := h.secrets.Patch(incoming.Namespace, incoming.Name, types.MergePatchType, patch); err != nil {
		return incoming, err
	}

	return incoming, nil
}

func (h *handler) createOrUpdateSecret(secret *corev1.Secret) error {
	if _, err := h.secretCache.Get(secret.Namespace, secret.Name); err != nil {
		if apierrors.IsNotFound(err) {
			_, createErr := h.secrets.Create(secret)
			return createErr
		}
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		currentSecret, err := h.secrets.Get(secret.Namespace, secret.Name, metav1.GetOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
		}

		_, updateErr := h.updateSecret(currentSecret, secret)
		return updateErr
	})
}

func (h *handler) updateRegistration(incoming, target *v1.Registration) (*v1.Registration, error) {
	incomingJson, err := json.Marshal(incoming)
	if err != nil {
		return incoming, err
	}
	newJson, err := json.Marshal(target)
	if err != nil {
		return incoming, err
	}

	patch, err := jsonpatch.CreateMergePatch(incomingJson, newJson)
	if err != nil {
		return incoming, err
	}
	if _, err := h.registrations.Patch(incoming.Name, types.MergePatchType, patch); err != nil {
		return incoming, err
	}
	return incoming, nil
}

func (h *handler) createOrUpdateRegistration(reg *v1.Registration) error {
	if _, err := h.registrations.Get(reg.Name, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			_, createErr := h.registrations.Create(reg)
			return createErr
		}
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		currentReg, err := h.registrations.Get(reg.Name, metav1.GetOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
		}

		_, updateErr := h.updateRegistration(currentReg, reg)
		return updateErr
	})
}
