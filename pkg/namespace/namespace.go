package namespace

import (
	"fmt"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

const (
	NsByProjectIndex        = "authz.cluster.cattle.io/ns-by-project"
	PrtbByRoleTemplateIndex = "prtb-rt-index"
	projectIDAnnotation     = "field.cattle.io/projectId"
)

// NsByProjectID is an index that selects namespaces based on the projectIDAnnotation
func NsByProjectID(obj interface{}) ([]string, error) {
	ns, ok := obj.(*v1.Namespace)
	if !ok {
		return []string{}, nil
	}

	if id, ok := ns.Annotations[projectIDAnnotation]; ok {
		return []string{id}, nil
	}

	return []string{}, nil
}

// PrtbByRoleTemplateName is an index that selects PRTBs by the Role Template name
func PrtbByRoleTemplateName(obj interface{}) ([]string, error) {
	prtb, ok := obj.(*v3.ProjectRoleTemplateBinding)
	if !ok {
		return []string{}, nil
	}
	if prtb == nil {
		return []string{}, nil
	}
	return []string{prtb.RoleTemplateName}, nil
}

// NsEnqueuer is a helper struct for enqueuing Namespaces
type NsEnqueuer struct {
	PrtbCache cache.Indexer
	NsIndexer cache.Indexer
}

// RoleTemplateEnqueueNamespace enqueues namespaces when a Role Template with PRTBs in that namespace is updated
func (n *NsEnqueuer) RoleTemplateEnqueueNamespace(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	if obj == nil {
		return nil, nil
	}
	rt, ok := obj.(*v3.RoleTemplate)
	if !ok {
		logrus.Errorf("unable to convert object: %[1]v, type %[1]T to a cluster", obj)
		return nil, nil
	}

	// Get PRTBs by field RoleTemplateName
	prtbs, err := n.PrtbCache.ByIndex(PrtbByRoleTemplateIndex, rt.Name)
	if err != nil {
		return nil, fmt.Errorf("unable to get prtbs for rt %s from indexer: %w", rt.Name, err)
	}

	// Get Namespaces of PRTBs
	var namespaceKeys = []relatedresource.Key{}
	var namespaceMap = map[string]bool{}
	for _, obj := range prtbs {
		prtb, ok := obj.(*v3.ProjectRoleTemplateBinding)
		if !ok {
			logrus.Errorf("unable to convert object: %[1]v, type %[1]T to a cluster", obj)
			return nil, nil
		}

		namespaces, err := n.NsIndexer.ByIndex(NsByProjectIndex, prtb.ProjectName)
		if err != nil {
			return nil, fmt.Errorf("unable to get namespaces for prtb %s from indexer: %w", prtb.Name, err)
		}
		for _, ns := range namespaces {
			if namespace, ok := ns.(*v1.Namespace); ok {
				// Only queue each namespace once
				if _, ok := namespaceMap[namespace.Name]; !ok {
					namespaceKeys = append(namespaceKeys, relatedresource.Key{Name: namespace.Name})
					namespaceMap[namespace.Name] = true
				}
			}
		}
	}

	return namespaceKeys, nil
}
