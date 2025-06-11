package controllers

import (
	"context"
	"fmt"

	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"k8s.io/apimachinery/pkg/api/meta"
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

	relatedresource.WatchClusterScoped(
		ctx,
		"watch-scc-secret-related",
		h.resolveCredentialSecret,
		h.registrations,
		h.secrets,
	)
}

func (h *handler) resolveEntrypointSecret(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
	ret := []relatedresource.Key{}
	if namespace != h.systemNamespace {
		return []relatedresource.Key{}, nil
	}
	if name != ResourceSCCEntrypointSecretName {
		return []relatedresource.Key{}, nil
	}
	metaObj, err := meta.Accessor(obj)
	if err != nil {
		return []relatedresource.Key{}, nil
	}

	curHash, ok := metaObj.GetLabels()[LabelSccHash]
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

func (h *handler) resolveCredentialSecret(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
	ret := []relatedresource.Key{}
	if namespace != h.systemNamespace {
		return nil, nil
	}

	metaObj, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	curHash, ok := metaObj.GetLabels()[LabelSccHash]
	if !ok {
		return nil, nil
	}

	regs, err := h.registrationCache.GetByIndex(IndexRegistrationsBySccHash, curHash)
	if err != nil {
		return nil, err
	}

	for _, reg := range regs {
		ret = append(ret, relatedresource.Key{
			Name: reg.Name,
		})
	}

	return ret, nil
}
