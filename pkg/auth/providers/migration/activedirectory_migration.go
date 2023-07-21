package migration

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"

	"k8s.io/client-go/tools/cache"
)

const (
	crtbsByPrincipalAndUserIndex = "auth.management.cattle.io/crtbByPrincipalAndUser"
	prtbsByPrincipalAndUserIndex = "auth.management.cattle.io/prtbByPrincipalAndUser"
	tokenByUserRefKey            = "auth.management.cattle.io/token-by-user-ref"
)

type adMigration struct {
	users        v3.UserInterface
	userLister   v3.UserLister
	tokens       v3.TokenInterface
	prtbs        v3.ProjectRoleTemplateBindingInterface
	crtbs        v3.ClusterRoleTemplateBindingInterface
	crtbIndexer  cache.Indexer
	prtbIndexer  cache.Indexer
	tokenIndexer cache.Indexer
}

// MigrateActiveDirectoryDNToGUID will go through all Rancher users and check to see if the principalID
// is an LDAP distinguished name, which was the way we used to map Rancher users to their LDAP entries.
// If a principalID is based on DN, this will update the user's principalID along with the tokens,
// CRTBs, and PRTBs to use a principalID that is the objectGUID for the user.
func MigrateActiveDirectoryDNToGUID(ctx context.Context, management *config.ManagementContext) {
	m := adMigration{
		users:        management.Management.Users(""),
		userLister:   management.Management.Users("").Controller().Lister(),
		tokens:       management.Management.Tokens(""),
		tokenIndexer: management.Management.Tokens("").Controller().Informer().GetIndexer(),
		crtbs:        management.Management.ClusterRoleTemplateBindings(""),
		crtbIndexer:  management.Management.ClusterRoleTemplateBindings("").Controller().Informer().GetIndexer(),
		prtbs:        management.Management.ProjectRoleTemplateBindings(""),
		prtbIndexer:  management.Management.ProjectRoleTemplateBindings("").Controller().Informer().GetIndexer(),
	}

	migrateCtx, migrateCancel := context.WithCancel(ctx)
	go func(context.Context, context.CancelFunc) {
		defer migrateCancel()
		err := wait.PollImmediate(time.Hour*24, 0, func() (bool, error) {
			logrus.Debugf("Starting active directory principalID migration with exponentialBackoff")
			steps := 5
			backOffDuration := time.Minute * 10
			var err error
			for steps > 0 {
				logrus.Infof("Active directory principalID migration starting")
				err = m.migrate()
				if err != nil {
					time.Sleep(backOffDuration)
					backOffDuration = 2 * backOffDuration
				} else {
					logrus.Infof("Active directory principalID migration complete")
					break
				}
				steps--
			}
			if err != nil {
				// returning false & nil because PollImmediate terminates on error
				logrus.Errorf("problem in migrating active directory user principalIds %v", err)
				return false, nil
			}
			// no error returned, user cleanup done, calling the child context's cancelfunc to terminate child context
			return true, nil
		})
		if err != nil {
			logrus.Errorf("problem in migrating active directory user principalIds %v", err)
			return
		}
	}(migrateCtx, migrateCancel)
}

func (m *adMigration) migrate() error {
	users, err := m.userLister.List("", labels.Everything())
	if err != nil {
		return fmt.Errorf("error listing users during active directory migration: %w", err)
	}
	for _, user := range users {
		for _, principal := range user.PrincipalIDs {
			if strings.HasPrefix(principal, "activedirectory_user://") && strings.Contains(principal, ",") {
				err = m.migrateUser(user, principal)
				if err != nil {
					return fmt.Errorf("error migrating user: %w", err)
				}
			}
		}
	}
	return nil
}

func (m *adMigration) migrateUser(user *v32.User, dn string) error {
	adProvider, err := providers.GetProvider("activedirectory")
	if err != nil {
		return fmt.Errorf("unable to fetch activedirectory provider: %w", err)
	}
	token := v3.Token{}
	userPrincipal, err := adProvider.GetPrincipal(dn, token)
	if err != nil {
		return fmt.Errorf("failed to fetch principal: %w", err)
	}

	if err = m.migrateTokens(user.Name, userPrincipal.Name); err != nil {
		return fmt.Errorf("failed to migrate tokens for user: %w", err)
	}
	if err = m.migrateCRTB(userPrincipal.Name, dn); err != nil {
		return fmt.Errorf("failed to migrate CRTBs for user: %w", err)
	}
	if err = m.migratePRTB(userPrincipal.Name, dn); err != nil {
		return fmt.Errorf("failed to migrate PRTBs for user: %w", err)
	}
	if err = m.updateUserObject(user, userPrincipal.Name, dn); err != nil {
		return fmt.Errorf("failed to migrate user: %w", err)
	}
	return nil
}

func (m *adMigration) updateUserObject(user *v32.User, newPrincipalID string, dn string) error {
	updatedUser := user.DeepCopy()
	for i, pID := range updatedUser.PrincipalIDs {
		if strings.HasPrefix(pID, dn) {
			updatedUser.PrincipalIDs[i] = newPrincipalID
		}
	}
	if _, err := m.users.Update(updatedUser); err != nil {
		return fmt.Errorf("failed updating user object: %w", err)
	}
	return nil
}

func (m *adMigration) migrateTokens(userName string, newPrincipalID string) error {
	userTokens, err := m.tokenIndexer.ByIndex(tokenByUserRefKey, userName)
	if err != nil {
		return fmt.Errorf("failed to fetch tokens: %w", err)
	}

	for _, obj := range userTokens {
		token, ok := obj.(*v3.Token)
		if !ok {
			return errors.Errorf("failed to convert object to Token for user %v principalId %v", userName, newPrincipalID)
		}
		token.UserPrincipal.Name = newPrincipalID
		_, e := m.tokens.Update(token)
		if e != nil {
			logrus.Errorf("unable to update token %v for principalId %v", token.Name, newPrincipalID)
		}
	}
	return nil
}

func (m *adMigration) migrateCRTB(newPrincipalID string, dn string) error {
	userCRTBs, err := m.crtbIndexer.ByIndex(crtbsByPrincipalAndUserIndex, dn)
	if err != nil {
		return fmt.Errorf("failed to fetch CRTBs: %w", err)
	}
	for _, crtb := range userCRTBs {
		oldCrtb, ok := crtb.(*v3.ClusterRoleTemplateBinding)
		if !ok {
			return fmt.Errorf("failed to convert object to CRTB for principalId %v", newPrincipalID)
		}
		newCrtb := &v3.ClusterRoleTemplateBinding{
			ObjectMeta: v1.ObjectMeta{
				Name:         "",
				Namespace:    oldCrtb.ObjectMeta.Namespace,
				GenerateName: "crtb-",
			},
			ClusterName:       oldCrtb.ClusterName,
			UserName:          oldCrtb.UserName,
			RoleTemplateName:  oldCrtb.RoleTemplateName,
			UserPrincipalName: newPrincipalID,
		}
		_, err := m.crtbs.Create(newCrtb)
		if err != nil {
			return fmt.Errorf("unable to create new CRTB: %w", err)
		}
		err = m.crtbs.DeleteNamespaced(oldCrtb.Namespace, oldCrtb.Name, &metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("unable to delete CRTB: %w", err)
		}
	}
	return nil
}

func (m *adMigration) migratePRTB(newPrincipalID string, dn string) error {
	userPRTBs, err := m.prtbIndexer.ByIndex(prtbsByPrincipalAndUserIndex, dn)
	if err != nil {
		return fmt.Errorf("failed to fetch PRTBs: %w", err)
	}
	for _, prtb := range userPRTBs {
		oldPrtb, ok := prtb.(*v3.ProjectRoleTemplateBinding)
		if !ok {
			return fmt.Errorf("failed to convert object to PRTB for principalId %v: %w", newPrincipalID, err)
		}
		newPrtb := &v3.ProjectRoleTemplateBinding{
			ObjectMeta: v1.ObjectMeta{
				Name:         "",
				Namespace:    oldPrtb.ObjectMeta.Namespace,
				GenerateName: "prtb-",
			},
			ProjectName:       oldPrtb.ProjectName,
			UserName:          oldPrtb.UserName,
			RoleTemplateName:  oldPrtb.RoleTemplateName,
			UserPrincipalName: newPrincipalID,
		}
		_, err := m.prtbs.Create(newPrtb)
		if err != nil {
			return fmt.Errorf("unable to create new PRTB: %w", err)
		}
		err = m.prtbs.DeleteNamespaced(oldPrtb.Namespace, oldPrtb.Name, &metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("unable to delete PRTB: %w", err)
		}
	}
	return nil
}
