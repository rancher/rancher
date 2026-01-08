package namespace

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/wrangler"
	wranglercorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
)

type handler struct {
	namespaces wranglercorev1.NamespaceController
}

func (h *handler) shouldManage(ns *corev1.Namespace) bool {
	if managed, ok := ns.Annotations[namespace.AnnotationManagedNamespace]; ok && managed == namespace.AnnotationManagedNamespceTrue {
		return true
	}

	if _, ok := ns.Annotations[project.ProjectIDAnnotation]; ok {
		return true
	}

	return false
}

func (h *handler) onChange(_ string, ns *corev1.Namespace) (*corev1.Namespace, error) {
	if ns == nil {
		return nil, nil
	}

	var err error

	if !h.shouldManage(ns) {
		return ns, nil
	}

	if updated := namespace.ApplyLabelsAndAnnotations(ns); updated {
		if ns, err = h.namespaces.Update(ns); err != nil {
			return nil, fmt.Errorf("failed to apply namespace labels or annotations: %w", err)
		}
	}

	return ns, nil
}

func Register(ctx context.Context, wContext *wrangler.Context) {
	nsClient := wContext.Core.Namespace()

	handler := &handler{
		namespaces: nsClient,
	}

	nsClient.OnChange(ctx, "namespace-label-and-annotation-change", handler.onChange)
}
