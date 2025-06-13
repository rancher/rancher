package controllers

import (
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/consts"
)

const (
	IndexSecretsBySccHash        = "scc.io/secret-refs-by-scc-hash"
	IndexRegistrationsBySccHash  = "scc.io/secret-refs-by-hash"
	IndexRegistrationsByNameHash = "scc.io/secret-refs-by-name-hash"
)

func (h *handler) initIndexers() {
	h.registrationCache.AddIndexer(
		IndexRegistrationsBySccHash,
		h.registrationToHash,
	)
	h.registrationCache.AddIndexer(
		IndexRegistrationsByNameHash,
		h.registrationToNameHash,
	)
}

func (h *handler) registrationToHash(reg *v1.Registration) ([]string, error) {
	if reg == nil {
		return nil, nil
	}

	hash, ok := reg.Labels[consts.LabelSccHash]
	if !ok {
		return nil, nil
	}
	return []string{hash}, nil
}

func (h *handler) registrationToNameHash(reg *v1.Registration) ([]string, error) {
	if reg == nil {
		return nil, nil
	}

	hash, ok := reg.Labels[consts.LabelNameSuffix]
	if !ok {
		return nil, nil
	}
	return []string{hash}, nil
}
