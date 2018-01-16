package authz

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (r *roleHandler) syncNS(key string, obj *v1.Namespace) error {
	if obj == nil {
		return nil
	}

	if obj.DeletionTimestamp != nil {
		return nil
	}

	if err := r.ensureDefaultNamespaceAssigned(key, obj); err != nil {
		return err
	}

	return r.ensurePRTBAddToNamespace(key, obj)
}

func (r *roleHandler) ensureDefaultNamespaceAssigned(key string, ns *v1.Namespace) error {
	if ns.Name != "default" {
		return nil
	}

	cluster, err := r.clusterLister.Get("", r.clusterName)
	if err != nil {
		return err
	}
	if cluster == nil {
		return errors.Errorf("couldn't find cluster %v", r.clusterName)
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
		ns.Annotations[projectIDAnnotation] = fmt.Sprintf("%v:rancher-default", r.clusterName)
		if _, err := r.workload.Core.Namespaces(r.clusterName).Update(ns); err != nil {
			return nil, err
		}

		return nil, nil
	})
	if updateCluster {
		if _, err := r.workload.Management.Management.Clusters("").ObjectClient().Update(cluster.Name, c); err != nil {
			return err
		}
	}
	return err
}

func (r *roleHandler) ensurePRTBAddToNamespace(key string, obj *v1.Namespace) error {
	// Get project that contain this namespace
	projectID := obj.Annotations[projectIDAnnotation]
	if len(projectID) == 0 {
		return nil
	}

	prtbs, err := r.prtbIndexer.ByIndex(prtbIndex, projectID)
	if err != nil {
		return errors.Wrapf(err, "couldn't get project role binding templates associated with project id %s", projectID)
	}
	for _, prtb := range prtbs {
		prtb, ok := prtb.(*v3.ProjectRoleTemplateBinding)
		if !ok {
			return errors.Wrapf(err, "object %v is not valid project role template binding", prtb)
		}

		if prtb.RoleTemplateName == "" {
			logrus.Warnf("ProjectRoleTemplateBinding %v has no role template set. Skipping.", prtb.Name)
			continue
		}

		rt, err := r.rtLister.Get("", prtb.RoleTemplateName)
		if err != nil {
			return errors.Wrapf(err, "couldn't get role template %v", prtb.RoleTemplateName)
		}

		roles := map[string]*v3.RoleTemplate{}
		if err := r.gatherRoles(rt, roles); err != nil {
			return err
		}

		if err := r.ensureRoles(roles); err != nil {
			return errors.Wrap(err, "couldn't ensure roles")
		}

		for _, role := range roles {
			if err := r.ensureBinding(obj.Name, role.Name, prtb); err != nil {
				return errors.Wrapf(err, "couldn't ensure binding %v %v in %v", role.Name, prtb.Subject.Name, obj.Name)
			}
		}
	}
	return nil
}
