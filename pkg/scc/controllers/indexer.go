package controllers

import (
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	IndexSecretsBySccHash       = "scc.io/secret-refs-by-scc-hash"
	IndexRegistrationsBySccHash = "scc.io/secret-refs-by-hash"
)

func (h *handler) initIndexers() {
	h.secretCache.AddIndexer(
		IndexSecretsBySccHash,
		h.secretToHash,
	)

	h.registrationCache.AddIndexer(
		IndexRegistrationsBySccHash,
		h.registrationToHash,
	)

}

// secretToHash specifically retrieves the scc managed hash constructed from the initial registration code
// from a secret
func (h *handler) secretToHash(secret *corev1.Secret) ([]string, error) {
	if secret == nil {
		return nil, nil
	}
	if secret.Namespace != h.systemNamespace {
		return nil, nil
	}

	if h.isRancherEntrypointSecret(secret) {
		return nil, nil
	}

	hash, ok := secret.Labels[LabelSccHash]
	if !ok {
		return nil, nil
	}
	return []string{hash}, nil
}

func (h *handler) registrationToHash(reg *v1.Registration) ([]string, error) {
	if reg == nil {
		return nil, nil
	}

	hash, ok := reg.Labels[LabelSccHash]
	if !ok {
		return nil, nil
	}
	return []string{hash}, nil
}
