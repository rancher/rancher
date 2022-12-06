package rbac

import (
	"fmt"
	"strings"

	"github.com/rancher/rancher/pkg/api/steve/projectresources"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	projectpkg "github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/settings"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	projectNamespaceAnnotation = "management.cattle.io/system-namespace"
)

func newProjectLifecycle(r *manager) *pLifecycle {
	return &pLifecycle{m: r}
}

type pLifecycle struct {
	m *manager
}

// Create runs lifecycle operations for a newly created project.
func (p *pLifecycle) Create(project *v3.Project) (runtime.Object, error) {
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

	err := p.ensureNamespacesAssigned(project)
	if err != nil {
		return project, err
	}

	err = p.ensureProjectNS(project)
	return project, err
}

// Updated runs lifecycle operations for an updated project.
func (p *pLifecycle) Updated(project *v3.Project) (runtime.Object, error) {
	err := p.ensureNamespacesAssigned(project)
	if err != nil {
		return project, err
	}
	err = p.ensureProjectNS(project)
	return project, err
}

// Remove runs lifecycle operations for a deleted project.
func (p *pLifecycle) Remove(project *v3.Project) (runtime.Object, error) {
	for _, suffix := range projectNSVerbToSuffix {
		roleName := fmt.Sprintf(projectNSGetClusterRoleNameFmt, project.Name, suffix)

		err := p.m.clusterRoles.Delete(roleName, &metav1.DeleteOptions{})
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
			err := p.m.namespaces.Delete(namespace.Name, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return project, err
			}
		} else {
			namespace = namespace.DeepCopy()
			if namespace.Annotations != nil {
				delete(namespace.Annotations, projectIDAnnotation)
				_, err := p.m.namespaces.Update(namespace)
				if err != nil {
					return project, err
				}
			}
		}
	}

	projectNamespace, err := p.m.nsLister.Get("", project.Name)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, ok := projectNamespace.Annotations[projectNamespaceAnnotation]; !ok { // let management controller handle local namespace projects
		err = p.m.namespaces.Delete(project.Name, &metav1.DeleteOptions{})
		return nil, err
	}

	return nil, nil
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
	_, err := v32.ProjectConditionDefaultNamespacesAssigned.DoUntilTrue(project, func() (runtime.Object, error) {
		return nil, p.assignNamespacesToProject(project, projectpkg.Default)
	})
	return err
}

func (p *pLifecycle) ensureSystemNamespaceAssigned(project *v3.Project) error {
	_, err := v32.ProjectConditionSystemNamespacesAssigned.DoUntilTrue(project, func() (runtime.Object, error) {
		return nil, p.assignNamespacesToProject(project, projectpkg.System)
	})
	return err
}

func (p *pLifecycle) assignNamespacesToProject(project *v3.Project, projectName string) error {
	initialProjectsToNamespaces, err := getDefaultAndSystemProjectsToNamespaces()
	if err != nil {
		return err
	}
	for _, nsName := range initialProjectsToNamespaces[projectName] {
		ns, err := p.m.nsLister.Get("", nsName)
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

// ensureProjectNS creates a namespace representing the project in the downstream cluster
// for the purpose of assign roles on it. This allows the resources.project.cattle.io API to work.
func (p *pLifecycle) ensureProjectNS(project *v3.Project) error {
	if p.m.clusterName == "local" {
		// the management context will create the namespace backing for the project, don't interfere
		return nil
	}

	namespace, err := p.m.nsLister.Get("", project.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error looking up namespace for project %s: %w", project.Name, err)
	}
	if namespace != nil {
		labels := namespace.GetLabels() // downstream cluster namespace has the parent label
		if _, ok := labels[projectresources.ParentLabel]; ok {
			return nil
		}
		annotations := namespace.GetAnnotations() // local cluster namespace has the system-namespace annotation
		if _, ok := annotations[projectNamespaceAnnotation]; ok {
			return nil
		}
		return fmt.Errorf("failed to create namespace for project %s, a namespace with that name already exists", project.Name)
	}
	_, err = p.m.namespaces.Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   project.Name,
			Labels: map[string]string{projectresources.ParentLabel: "true"},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create namespace for project %s: %w", project.Name, err)
	}
	return nil
}
