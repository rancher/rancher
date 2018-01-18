package authz

import (
	"fmt"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newProjectLifecycle(r *manager) *pLifecycle {
	return &pLifecycle{m: r}
}

type pLifecycle struct {
	m *manager
}

func (p *pLifecycle) Create(project *v3.Project) (*v3.Project, error) {
	for verb, suffix := range projectNSVerbToSuffix {
		roleName := fmt.Sprintf(projectNSGetClusterRoleNameFmt, project.Name, suffix)
		_, err := p.m.crLister.Get("", roleName)
		if err == nil || !apierrors.IsNotFound(err) {
			continue
		}

		err = p.m.createProjectNSRole(roleName, verb, "")
		if err != nil {
			return project, err
		}

	}
	return project, nil
}

func (p *pLifecycle) Updated(project *v3.Project) (*v3.Project, error) {
	return nil, nil
}

func (p *pLifecycle) Remove(project *v3.Project) (*v3.Project, error) {
	for _, suffix := range projectNSVerbToSuffix {
		roleName := fmt.Sprintf(projectNSGetClusterRoleNameFmt, project.Name, suffix)

		err := p.m.workload.RBAC.ClusterRoles("").Delete(roleName, &v1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return project, err
		}
	}

	projectID := project.Namespace + ":" + project.Name
	namespaces, err := p.m.nsIndexer.ByIndex(nsByProjectIndex, projectID)
	if err != nil {
		return project, err
	}

	for _, o := range namespaces {
		namespace, _ := o.(*corev1.Namespace)
		if _, ok := namespace.Annotations["field.cattle.io/creatorId"]; ok {
			err := p.m.workload.Core.Namespaces("").Delete(namespace.Name, &v1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return project, err
			}
		} else {
			namespace = namespace.DeepCopy()
			if namespace.Annotations != nil {
				delete(namespace.Annotations, projectIDAnnotation)
				_, err := p.m.workload.Core.Namespaces("").Update(namespace)
				if err != nil {
					return project, err
				}
			}
		}
	}

	return nil, nil
}
