package rbac

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

	err := p.ensureDefaultNamespaceAssigned(project)
	return project, err
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

func (p *pLifecycle) ensureDefaultNamespaceAssigned(project *v3.Project) error {
	if _, ok := project.Labels["authz.management.cattle.io/default-project"]; !ok {
		return nil
	}
	ns, err := p.m.nsLister.Get("", "default")
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	cluster, err := p.m.clusterLister.Get("", p.m.clusterName)
	if err != nil {
		return err
	}
	if cluster == nil {
		return errors.Errorf("couldn't find cluster %v", p.m.clusterName)
	}

	updateCluster := false
	c, err := v3.ClusterConditionDefaultNamespaceAssigned.DoUntilTrue(cluster.DeepCopy(), func() (runtime.Object, error) {
		updateCluster = true
		projectID := ns.Annotations[projectIDAnnotation]
		if projectID != "" {
			return nil, nil
		}

		ns = ns.DeepCopy()
		if ns.Annotations == nil {
			ns.Annotations = map[string]string{}
		}
		ns.Annotations[projectIDAnnotation] = fmt.Sprintf("%v:%v", p.m.clusterName, project.Name)
		if _, err := p.m.workload.Core.Namespaces(p.m.clusterName).Update(ns); err != nil {
			return nil, err
		}

		return nil, nil
	})
	if updateCluster {
		if _, err := p.m.workload.Management.Management.Clusters("").ObjectClient().Update(cluster.Name, c); err != nil {
			return err
		}
	}
	return err
}
