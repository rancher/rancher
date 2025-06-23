package node

import (
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/sirupsen/logrus"
)

const (
	cleanupPodLabel                    = "rke.cattle.io/cleanup-node"
	userNodeRemoveAnnotationPrefix     = "lifecycle.cattle.io/create.user-node-remove_"
	userNodeRemoveCleanupAnnotationOld = "nodes.management.cattle.io/user-node-remove-cleanup"
)

func (m *Lifecycle) userNodeRemoveCleanup(obj *v3.Node) *v3.Node {
	obj = obj.DeepCopy()
	obj.SetFinalizers(removeFinalizerWithPrefix(obj.GetFinalizers(), userNodeRemoveFinalizerPrefix))

	if obj.DeletionTimestamp == nil {
		annos := obj.GetAnnotations()
		if annos == nil {
			annos = make(map[string]string)
		} else {
			annos = removeAnnotationWithPrefix(annos, userNodeRemoveAnnotationPrefix)
			delete(annos, userNodeRemoveCleanupAnnotationOld)
		}

		annos[userNodeRemoveCleanupAnnotation] = "true"
		obj.SetAnnotations(annos)
	}
	return obj
}

func removeFinalizerWithPrefix(finalizers []string, prefix string) []string {
	var nf []string
	for _, finalizer := range finalizers {
		if strings.HasPrefix(finalizer, prefix) {
			logrus.Debugf("[node-cleanup] finalizer with prefix [%s] will be removed", prefix)
			continue
		}
		nf = append(nf, finalizer)
	}
	return nf
}

func removeAnnotationWithPrefix(annotations map[string]string, prefix string) map[string]string {
	for k := range annotations {
		if strings.HasPrefix(k, prefix) {
			logrus.Debugf("[node-cleanup] annotation with prefix [%s] will be removed", prefix)
			delete(annotations, k)
		}
	}
	return annotations
}
