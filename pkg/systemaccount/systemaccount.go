package systemaccount

import (
	"fmt"

	"github.com/rancher/rancher/pkg/randomtoken"
	"github.com/rancher/rancher/pkg/ref"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/rancher/types/config/systemtokens"
	"github.com/rancher/types/user"
	errors2 "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	clusterOwnerRole           = "cluster-owner"
	projectMemberRole          = "project-member"
	ClusterSystemAccountPrefix = "System account for Cluster "
	ProjectSystemAccountPrefix = "System account for Project "
)

func NewManager(management *config.ManagementContext) *Manager {
	return &Manager{
		userManager:  management.UserManager,
		systemTokens: management.SystemTokens,
		crtbs:        management.Management.ClusterRoleTemplateBindings(""),
		crts:         management.Management.ClusterRegistrationTokens(""),
		prtbs:        management.Management.ProjectRoleTemplateBindings(""),
		prtbLister:   management.Management.ProjectRoleTemplateBindings("").Controller().Lister(),
		tokens:       management.Management.Tokens(""),
		users:        management.Management.Users(""),
	}
}

func NewManagerFromScale(management *config.ScaledContext) *Manager {
	return &Manager{
		userManager:  management.UserManager,
		systemTokens: management.SystemTokens,
		crtbs:        management.Management.ClusterRoleTemplateBindings(""),
		crts:         management.Management.ClusterRegistrationTokens(""),
		prtbs:        management.Management.ProjectRoleTemplateBindings(""),
		prtbLister:   management.Management.ProjectRoleTemplateBindings("").Controller().Lister(),
		tokens:       management.Management.Tokens(""),
		users:        management.Management.Users(""),
	}
}

type Manager struct {
	userManager  user.Manager
	systemTokens systemtokens.Interface
	crtbs        v3.ClusterRoleTemplateBindingInterface
	crts         v3.ClusterRegistrationTokenInterface
	prtbs        v3.ProjectRoleTemplateBindingInterface
	prtbLister   v3.ProjectRoleTemplateBindingLister
	tokens       v3.TokenInterface
	users        v3.UserInterface
}

func (s *Manager) CreateSystemAccount(cluster *v3.Cluster) error {
	user, err := s.GetSystemUser(cluster.Name)
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

func (s *Manager) GetSystemUser(clusterName string) (*v3.User, error) {
	return s.userManager.EnsureUser(fmt.Sprintf("system://%s", clusterName), ClusterSystemAccountPrefix+clusterName)
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

	u, err := s.GetProjectSystemUser(projectName)
	if err != nil {
		return err
	}

	bindingName := u.Name + "-member"
	_, err = s.prtbLister.Get(projectName, bindingName)
	if err != nil {
		if !errors2.IsNotFound(err) {
			return err
		}
		// prtb does not exist in cache, attempt to create it
		prtb := &v3.ProjectRoleTemplateBinding{
			ObjectMeta: v1.ObjectMeta{
				Name:      bindingName,
				Namespace: projectName,
			},
			ProjectName:      projectID,
			UserName:         u.Name,
			RoleTemplateName: projectMemberRole,
		}
		if _, err := s.prtbs.Create(prtb); err != nil && !errors2.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func (s *Manager) GetProjectSystemUser(projectName string) (*v3.User, error) {
	return s.userManager.EnsureUser(fmt.Sprintf("system://%s", projectName), ProjectSystemAccountPrefix+projectName)
}

func (s *Manager) GetOrCreateProjectSystemToken(projectName string) (string, error) {
	user, err := s.GetProjectSystemUser(projectName)
	if err != nil {
		return "", err
	}
	return s.systemTokens.EnsureSystemToken(projectName+"-pipeline", "Pipeline token for project "+projectName, "pipeline", user.Name, nil)
}

func (s *Manager) RemoveSystemAccount(userID string) error {
	u, err := s.userManager.GetUserByPrincipalID(fmt.Sprintf("system://%s", userID))
	if err != nil {
		return err
	}
	if u == nil {
		// user not found, must have been removed
		return nil
	}
	if err := s.users.Delete(u.Name, &v1.DeleteOptions{}); err != nil && !errors2.IsNotFound(err) && !errors2.IsGone(err) {
		return err
	}
	return nil
}
