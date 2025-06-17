package controllers

import (
	"context"
	"github.com/rancher/rancher/pkg/scc/consts"
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
	var ret []relatedresource.Key
	if namespace != h.systemNamespace {
		return nil, nil
	}
	if name != consts.ResourceSCCEntrypointSecretName {
		return nil, nil
	}

	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil, nil
	}

	curHash, ok := secret.GetLabels()[consts.LabelSccHash]
	if !ok {
		h.log.Warnf("failed to find hash for secret %s/%s", namespace, name)
		return nil, nil
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
