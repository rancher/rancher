package controllers

import (
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/consts"
)

const (
	IndexSecretsBySccHash       = "scc.io/secret-refs-by-scc-hash"
	IndexRegistrationsBySccHash = "scc.io/secret-refs-by-hash"
)

func (h *handler) initIndexers() {
	h.registrationCache.AddIndexer(
		IndexRegistrationsBySccHash,
		h.registrationToHash,
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
