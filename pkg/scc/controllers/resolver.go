package controllers

import (
	"context"
	"fmt"

	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (h *handler) initResolvers(ctx context.Context) {
	relatedresource.WatchClusterScoped(
		ctx,
		"watch-scc-secret-entrypoint",
		h.resolveEntrypointSecret,
		h.registrations,
		h.secrets,
	)
}

func (h *handler) resolveEntrypointSecret(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
	ret := []relatedresource.Key{}
	if namespace != h.systemNamespace {
		return nil, nil
	}
	if name != ResourceSCCEntrypointSecretName {
		return nil, nil
	}

	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil, nil
	}

	curHash, ok := secret.GetLabels()[LabelSccHash]
	if !ok {
		return nil, fmt.Errorf("expected an SCC processed hash")
	}
	regs, err := h.registrationCache.GetByIndex(IndexRegistrationsBySccHash, curHash)
	if err != nil {
		return nil, err
	}
	for _, reg := range regs {
		ret = append(ret, relatedresource.Key{
			Name: reg.GetName(),
		})
	}
	return ret, nil
}
