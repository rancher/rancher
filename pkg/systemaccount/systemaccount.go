package systemaccount

import (
	"fmt"

	"github.com/rancher/rancher/pkg/randomtoken"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/rancher/types/user"
	errors2 "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	clusterOwnerRole  = "cluster-owner"
	projectMemberRole = "project-member"
)

func NewManager(management *config.ManagementContext) *Manager {
	return &Manager{
		userManager: management.UserManager,
		crtbs:       management.Management.ClusterRoleTemplateBindings(""),
		crts:        management.Management.ClusterRegistrationTokens(""),
		prtbs:       management.Management.ProjectRoleTemplateBindings(""),
		tokens:      management.Management.Tokens(""),
	}
}

func NewManagerFromScale(management *config.ScaledContext) *Manager {
	return &Manager{
		userManager: management.UserManager,
		crtbs:       management.Management.ClusterRoleTemplateBindings(""),
		crts:        management.Management.ClusterRegistrationTokens(""),
		prtbs:       management.Management.ProjectRoleTemplateBindings(""),
		tokens:      management.Management.Tokens(""),
	}
}

type Manager struct {
	userManager user.Manager
	crtbs       v3.ClusterRoleTemplateBindingInterface
	crts        v3.ClusterRegistrationTokenInterface
	prtbs       v3.ProjectRoleTemplateBindingInterface
	tokens      v3.TokenInterface
}

func (s *Manager) CreateSystemAccount(cluster *v3.Cluster) error {
	user, err := s.GetSystemUser(cluster)
	if err != nil {
		return err
	}

	bindingName := user.Name + "-admin"
	_, err = s.crtbs.GetNamespaced(cluster.Name, bindingName, v1.GetOptions{})
	if err == nil {
		return nil
	}

	_, err = s.crtbs.Create(&v3.ClusterRoleTemplateBinding{
		ObjectMeta: v1.ObjectMeta{
			Name:      bindingName,
			Namespace: cluster.Name,
		},
		ClusterName:      cluster.Name,
		UserName:         user.Name,
		RoleTemplateName: clusterOwnerRole,
	})

	return err
}

func (s *Manager) GetSystemUser(cluster *v3.Cluster) (*v3.User, error) {
	return s.userManager.EnsureUser(fmt.Sprintf("system://%s", cluster.Name), "System account for Cluster "+cluster.Name)
}

func (s *Manager) GetOrCreateSystemClusterToken(clusterName string) (string, error) {
	token := ""

	crt, err := s.crts.GetNamespaced(clusterName, "system", v1.GetOptions{})
	if errors2.IsNotFound(err) {
		token, err = randomtoken.Generate()
		if err != nil {
			return "", err
		}
		crt = &v3.ClusterRegistrationToken{
			ObjectMeta: v1.ObjectMeta{
				Name:      "system",
				Namespace: clusterName,
			},
			Spec: v3.ClusterRegistrationTokenSpec{
				ClusterName: clusterName,
			},
			Status: v3.ClusterRegistrationTokenStatus{
				Token: token,
			},
		}

		if _, err := s.crts.Create(crt); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	} else {
		token = crt.Status.Token
	}

	return token, nil
}

func (s *Manager) GetOrCreateProjectSystemAccount(projectID string) error {
	_, projectName := ref.Parse(projectID)

	user, err := s.GetProjectSystemUser(projectName)
	if err != nil {
		return err
	}

	bindingName := user.Name + "-member"
	_, err = s.prtbs.GetNamespaced(projectName, bindingName, v1.GetOptions{})
	if err == nil {
		return nil
	} else if errors2.IsNotFound(err) {
		_, err = s.prtbs.Create(&v3.ProjectRoleTemplateBinding{
			ObjectMeta: v1.ObjectMeta{
				Name:      bindingName,
				Namespace: projectName,
			},
			ProjectName:      projectID,
			UserName:         user.Name,
			RoleTemplateName: projectMemberRole,
		})
	}

	return err
}

func (s *Manager) GetProjectSystemUser(projectName string) (*v3.User, error) {
	return s.userManager.EnsureUser(fmt.Sprintf("system://%s", projectName), "System account for Project "+projectName)
}

func (s *Manager) GetOrCreateProjectSystemToken(projectName string) (string, error) {
	user, err := s.GetProjectSystemUser(projectName)
	if err != nil {
		return "", err
	}
	return s.userManager.EnsureToken(projectName+"-pipeline", "Pipeline token for project "+projectName, user.Name)
}
