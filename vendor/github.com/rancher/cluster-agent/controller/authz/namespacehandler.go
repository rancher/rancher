package authz

import (
	"github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/api/core/v1"
)

func (r *roleHandler) syncNS(key string, obj *v1.Namespace) error {
	if obj == nil {
		return nil
	}

	if obj.DeletionTimestamp != nil {
		return nil
	}

	return r.ensurePRTBAddToNamespace(key, obj)
}

func (r *roleHandler) ensurePRTBAddToNamespace(key string, obj *v1.Namespace) error {
	// Get project that contain this namespace
	projectID := obj.Labels[projectIDLabel]
	if len(projectID) == 0 {
		return nil
	}

	prtbs, err := r.indexer.ByIndex(prtbIndex, projectID)
	if err != nil {
		return errors.Wrapf(err, "couldn't get project role binding templates associated with project id %s", projectID)
	}
	for _, prtb := range prtbs {
		prtb, ok := prtb.(*v3.ProjectRoleTemplateBinding)
		if !ok {
			return errors.Wrapf(err, "object %v is not valid project role template binding", prtb)
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
