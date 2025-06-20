package controllers

import (
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/consts"
	corev1 "k8s.io/api/core/v1"
)

const (
	IndexSecretsBySccHash        = "scc.io/secret-refs-by-scc-hash"
	IndexRegistrationsBySccHash  = "scc.io/reg-refs-by-scc-hash"
	IndexRegistrationsByNameHash = "scc.io/reg-refs-by-name-hash"
)

func (h *handler) initIndexers() {
	//h.secretCache.AddIndexer(
	//	IndexSecretsBySccHash,
	//	h.secretToHash,
	//)
	h.registrationCache.AddIndexer(
		IndexRegistrationsBySccHash,
		h.registrationToHash,
	)
	h.registrationCache.AddIndexer(
		IndexRegistrationsByNameHash,
		h.registrationToNameHash,
	)
}

func (h *handler) secretToHash(secret *corev1.Secret) ([]string, error) {
	if secret == nil {
		return nil, nil
	}

	hash, ok := secret.Labels[consts.LabelSccHash]
	if !ok {
		return []string{}, nil
	}
	return []string{hash}, nil
}

func (h *handler) registrationToHash(reg *v1.Registration) ([]string, error) {
	if reg == nil {
		return []string{}, nil
	}

	hash, ok := reg.Labels[consts.LabelSccHash]
	if !ok {
		return []string{}, nil
	}
	return []string{hash}, nil
}

func (h *handler) registrationToNameHash(reg *v1.Registration) ([]string, error) {
	if reg == nil {
		return []string{}, nil
	}

	hash, ok := reg.Labels[consts.LabelNameSuffix]
	if !ok {
		return []string{}, nil
	}
	return []string{hash}, nil
}
