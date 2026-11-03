package systemaccount

import (
	"fmt"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/dashboard/clusterregistrationtoken"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/types/config/systemtokens"
	"github.com/rancher/rancher/pkg/user"
	"github.com/rancher/wrangler/v3/pkg/randomtoken"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
		secretLister: management.Core.Secrets("").Controller().Lister(),
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
		secretLister: management.Core.Secrets("").Controller().Lister(),
	}
}

type SystemAccountManager interface {
	RemoveSystemAccount(userID string) error
}

type Manager struct {
	userManager  user.Manager
	systemTokens systemtokens.Interface
	crtbs        mgmtcontrollers.ClusterRoleTemplateBindingInterface
	crts         mgmtcontrollers.ClusterRegistrationTokenInterface
	prtbLister   mgmtcontrollers.ProjectRoleTemplateBindingLister
	tokens       mgmtcontrollers.TokenInterface
	tokenLister  mgmtcontrollers.TokenLister
	users        mgmtcontrollers.UserInterface
	secretLister corev1.SecretLister
}

func (s *Manager) CreateSystemAccount(cluster *apimgmtv3.Cluster) error {
	user, err := s.GetSystemUser(cluster.Name)
	if err != nil {
		return err
	}

	bindingName := user.Name + "-admin"
	_, err = s.crtbs.GetNamespaced(cluster.Name, bindingName, v1.GetOptions{})
	if err == nil {
		return nil
	}

	_, err = s.crtbs.Create(&apimgmtv3.ClusterRoleTemplateBinding{
		Name:             bindingName,
		Namespace:        cluster.Name,
		ClusterName:      cluster.Name,
		UserName:         user.Name,
		RoleTemplateName: clusterOwnerRole,
	})

	return err
}

func (s *Manager) GetSystemUser(clusterName string) (*apimgmtv3.User, error) {
	return s.userManager.EnsureUser(fmt.Sprintf("system://%s", clusterName), ClusterSystemAccountPrefix+clusterName)
}

func (s *Manager) GetOrCreateSystemClusterToken(clusterName string) (string, error) {
	token := ""

	crt, err := s.crts.GetNamespaced(clusterName, "system", v1.GetOptions{})
	if apierrors.IsNotFound(err) {
		token, err = randomtoken.Generate()
		if err != nil {
			return "", err
		}
		crt = &apimgmtv3.ClusterRegistrationToken{
			Name:      "system",
			Namespace: clusterName,
			Spec: apimgmtv3.ClusterRegistrationTokenSpec{
				ClusterName: clusterName,
			},
			Status: apimgmtv3.ClusterRegistrationTokenStatus{
				Token: token,
			},
		}

		if _, err = s.crts.Create(crt); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}

	token, err = clusterregistrationtoken.GetTokenFromSecret(s.secretLister, crt)
	if err != nil {
		return "", err
	}

	return token, nil
}

func (s *Manager) GetProjectSystemUser(projectName string) (*apimgmtv3.User, error) {
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
	if err := s.users.Delete(u.Name, &v1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) && !apierrors.IsGone(err) {
		return err
	}
	return nil
}
