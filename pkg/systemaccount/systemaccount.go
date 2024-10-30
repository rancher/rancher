package systemaccount

import (
	"fmt"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/types/config/systemtokens"
	"github.com/rancher/rancher/pkg/user"
	"github.com/rancher/wrangler/v3/pkg/randomtoken"
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
		prtbLister:   management.Management.ProjectRoleTemplateBindings("").Controller().Lister(),
		tokens:       management.Management.Tokens(""),
		tokenLister:  management.Management.Tokens("").Controller().Lister(),
		users:        management.Management.Users(""),
	}
}

type Manager struct {
	userManager  user.Manager
	systemTokens systemtokens.Interface
	crtbs        v3.ClusterRoleTemplateBindingInterface
	crts         v3.ClusterRegistrationTokenInterface
	prtbLister   v3.ProjectRoleTemplateBindingLister
	tokens       v3.TokenInterface
	tokenLister  v3.TokenLister
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
			Spec: v32.ClusterRegistrationTokenSpec{
				ClusterName: clusterName,
			},
			Status: v32.ClusterRegistrationTokenStatus{
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

func (s *Manager) GetProjectSystemUser(projectName string) (*v3.User, error) {
	return s.userManager.EnsureUser(fmt.Sprintf("system://%s", projectName), ProjectSystemAccountPrefix+projectName)
}

func (s *Manager) CreateProjectHelmSystemToken(projectName string) (string, error) {
	user, err := s.GetProjectSystemUser(projectName)
	if err != nil {
		return "", err
	}
	return s.systemTokens.EnsureSystemToken(projectName+"-helm", "Helm token for project "+projectName, "helm", user.Name, nil, true)
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
