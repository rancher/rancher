package rbac

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/managementuser/secret"
	"github.com/rancher/rancher/pkg/namespace"
	projectpkg "github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/settings"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func newProjectLifecycle(r *manager, secretClient wcorev1.SecretClient) *pLifecycle {
	return &pLifecycle{
		m:            r,
		secretClient: secretClient,
	}
}

type pLifecycle struct {
	m            *manager
	secretClient wcorev1.SecretClient
}

func (p *pLifecycle) Create(project *v3.Project) (runtime.Object, error) {
	for verb, suffix := range projectNSVerbToSuffix {
		roleName := fmt.Sprintf(projectNSGetClusterRoleNameFmt, project.Name, suffix)
		_, err := p.m.crLister.Get(roleName)
		if err == nil || !apierrors.IsNotFound(err) {
			continue
		}

		err = p.m.createProjectNSRole(roleName, verb, "")
		if err != nil {
			return project, err
		}

	}

	if err := p.ensureManageNSRole(project.Name); err != nil {
		return project, err
	}

	err := p.ensureNamespacesAssigned(project)
	return project, err
}

func (p *pLifecycle) Updated(project *v3.Project) (runtime.Object, error) {
	err := p.ensureNamespaceRolesUpdated(project)
	if err != nil {
		return project, err
	}
	err = p.ensureNamespacesAssigned(project)
	return project, err
}

func (p *pLifecycle) Remove(project *v3.Project) (runtime.Object, error) {
	suffixes := append(slices.Collect(maps.Values(projectNSVerbToSuffix)), projectNSManageSuffix)
	for _, suffix := range suffixes {
		roleName := fmt.Sprintf(projectNSGetClusterRoleNameFmt, project.Name, suffix)

		err := p.m.workload.RBACw.ClusterRole().Delete(roleName, &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		}
	}

	projectID := project.Namespace + ":" + project.Name
	namespaces, err := p.m.nsIndexer.ByIndex(namespace.NsByProjectIndex, projectID)
	if err != nil {
		return nil, err
	}

	var returnErrors error

	for _, o := range namespaces {
		namespace, _ := o.(*corev1.Namespace)
		if _, ok := namespace.Annotations["field.cattle.io/creatorId"]; ok {
			err := p.m.namespaces.Delete(namespace.Name, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return nil, err
			}
		} else {
			namespace = namespace.DeepCopy()
			if namespace.Annotations != nil {
				delete(namespace.Annotations, projectIDAnnotation)
				_, err := p.m.namespaces.Update(namespace)
				if err != nil {
					return nil, err
				}
			}
		}

		// remove project scoped secrets if they exist
		secrets, err := p.secretClient.List(namespace.Name, metav1.ListOptions{LabelSelector: secret.ProjectScopedSecretLabel + "=" + project.Name})
		if err != nil {
			return nil, fmt.Errorf("failed to list project scoped secrets: %w", err)
		}
		for _, secret := range secrets.Items {
			err := p.secretClient.Delete(namespace.Name, secret.Name, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				logrus.Errorf("failed to delete project scoped secret %s/%s: %v", namespace.Name, secret.Name, err)
				returnErrors = errors.Join(returnErrors, err)
			}
		}
	}

	return nil, returnErrors
}

func (p *pLifecycle) ensureNamespacesAssigned(project *v3.Project) error {
	projectName := ""
	if _, ok := project.Labels["authz.management.cattle.io/default-project"]; ok {
		projectName = projectpkg.Default
	} else if _, ok := project.Labels["authz.management.cattle.io/system-project"]; ok {
		projectName = projectpkg.System
	}
	if projectName == "" {
		return nil
	}

	switch projectName {
	case projectpkg.Default:
		if err := p.ensureDefaultNamespaceAssigned(project); err != nil {
			return err
		}
	case projectpkg.System:
		if err := p.ensureSystemNamespaceAssigned(project); err != nil {
			return err
		}
	default:
		return nil
	}

	_, err := p.m.workload.Management.Management.Projects(p.m.workload.ClusterName).Update(project)
	return err
}

func (p *pLifecycle) ensureDefaultNamespaceAssigned(project *v3.Project) error {
	_, err := v3.ProjectConditionDefaultNamespacesAssigned.DoUntilTrue(project, func() (runtime.Object, error) {
		return nil, p.assignNamespacesToProject(project, projectpkg.Default)
	})
	return err
}

func (p *pLifecycle) ensureSystemNamespaceAssigned(project *v3.Project) error {
	_, err := v3.ProjectConditionSystemNamespacesAssigned.DoUntilTrue(project, func() (runtime.Object, error) {
		return nil, p.assignNamespacesToProject(project, projectpkg.System)
	})
	return err
}

// ensureNamespaceRolesUpdated makes sure the project's namespaces-manage and namespaces-edit roles exist.
func (p *pLifecycle) ensureNamespaceRolesUpdated(project *v3.Project) error {
	if err := p.ensureManageNSRole(project.Name); err != nil {
		return err
	}

	// The namespace handler maintains the rules of the edit and readonly roles.
	suffix := projectNSVerbToSuffix[projectNSEditVerb]
	roleName := fmt.Sprintf(projectNSGetClusterRoleNameFmt, project.Name, suffix)
	_, err := p.m.crLister.Get(roleName)
	if apierrors.IsNotFound(err) {
		return p.m.createProjectNSRole(roleName, projectNSEditVerb, "")
	} else if err != nil {
		return fmt.Errorf("unable to get backing cluster role for project %s: %w", project.Name, err)
	}
	return nil
}

// ensureManageNSRole makes sure the project's namespaces-manage ClusterRole exists and only grants manage-namespaces on
// the project.
func (p *pLifecycle) ensureManageNSRole(projectName string) error {
	desired := buildProjectNSManageRole(projectName)
	cr, err := p.m.crLister.Get(desired.Name)
	if apierrors.IsNotFound(err) {
		if _, err := p.m.clusterRoles.Create(desired); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create namespaces manage cluster role for project %s: %w", projectName, err)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("unable to get namespaces manage cluster role for project %s: %w", projectName, err)
	}

	if reflect.DeepEqual(cr.Rules, desired.Rules) {
		return nil
	}
	cr = cr.DeepCopy()
	cr.Rules = desired.Rules
	if _, err := p.m.clusterRoles.Update(cr); err != nil {
		return fmt.Errorf("unable to update namespaces manage cluster role for project %s: %w", projectName, err)
	}
	return nil
}

func (p *pLifecycle) assignNamespacesToProject(project *v3.Project, projectName string) error {
	initialProjectsToNamespaces, err := getDefaultAndSystemProjectsToNamespaces()
	if err != nil {
		return err
	}
	for _, nsName := range initialProjectsToNamespaces[projectName] {
		ns, err := p.m.nsLister.Get(nsName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		projectID := ns.Annotations[projectIDAnnotation]
		if projectID != "" {
			continue
		}

		ns = ns.DeepCopy()
		if ns.Annotations == nil {
			ns.Annotations = map[string]string{}
		}
		ns.Annotations[projectIDAnnotation] = fmt.Sprintf("%v:%v", p.m.clusterName, project.Name)
		if _, err := p.m.namespaces.Update(ns); err != nil {
			return err
		}
	}
	return nil
}

func getDefaultAndSystemProjectsToNamespaces() (map[string][]string, error) {
	systemNamespacesStr := settings.SystemNamespaces.Get()
	var systemNamespaces []string
	if systemNamespacesStr == "" {
		return nil, fmt.Errorf("failed to load setting %v", settings.SystemNamespaces)
	}

	splitted := strings.Split(systemNamespacesStr, ",")
	for _, s := range splitted {
		systemNamespaces = append(systemNamespaces, strings.TrimSpace(s))
	}

	return map[string][]string{
		projectpkg.Default: {"default"},
		projectpkg.System:  systemNamespaces,
	}, nil
}
