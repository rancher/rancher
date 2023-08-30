package adunmigration

import (
	"fmt"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3norman "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/types/config"
)

func collectTokens(workunits *[]migrateUserWorkUnit, sc *config.ScaledContext) error {
	tokenInterface := sc.Management.Tokens("")
	tokenList, err := tokenInterface.List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("[%v] unable to fetch token objects: %v", migrateAdUserOperation, err)
		return err
	}

	adWorkUnitsByPrincipal, duplicateLocalWorkUnitsByPrincipal := principalsToMigrate(workunits)

	for _, token := range tokenList.Items {
		if index, exists := adWorkUnitsByPrincipal[token.UserPrincipal.Name]; exists {
			if workUnitContainsName(&(*workunits)[index], token.UserID) {
				(*workunits)[index].activeDirectoryTokens = append((*workunits)[index].activeDirectoryTokens, token)
			} else {
				logrus.Warnf("[%v] found token for user with guid-based principal '%v' and name '%v', but no user object with that name matches the GUID or its associated DN. refusing to process",
					identifyAdUserOperation, token.UserPrincipal.Name, token.UserID)
			}
		} else if index, exists = duplicateLocalWorkUnitsByPrincipal[token.UserPrincipal.Name]; exists {
			if workUnitContainsName(&(*workunits)[index], token.UserID) {
				(*workunits)[index].duplicateLocalTokens = append((*workunits)[index].duplicateLocalTokens, token)
			} else {
				logrus.Warnf("[%v] found token for user with guid-based principal '%v' and name '%v', but no user object with that name matches the GUID or its associated DN. refusing to process",
					identifyAdUserOperation, token.UserPrincipal.Name, token.UserID)
			}
		}
	}

	return nil
}

func updateToken(tokenInterface v3norman.TokenInterface, userToken v3.Token, newPrincipalID string, guid string, targetUser *v3.User, targetPrincipal *v3.Principal) error {
	latestToken, err := tokenInterface.Get(userToken.Name, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("[%v] token %s no longer exists: %v", migrateTokensOperation, userToken.Name, err)
		return nil
	}
	if latestToken.Annotations == nil {
		latestToken.Annotations = make(map[string]string)
	}
	latestToken.Annotations[adGUIDMigrationAnnotation] = guid
	if latestToken.Labels == nil {
		latestToken.Labels = make(map[string]string)
	}
	latestToken.Labels[tokens.UserIDLabel] = targetUser.Name
	latestToken.Labels[adGUIDMigrationLabel] = migratedLabelValue
	// use the new dnPrincipalID for the token name
	latestToken.UserPrincipal.Name = newPrincipalID
	// copy over other relevant fields to match the user we are attaching this token to
	latestToken.UserPrincipal.LoginName = targetPrincipal.LoginName
	latestToken.UserPrincipal.DisplayName = targetPrincipal.DisplayName
	latestToken.UserID = targetUser.Name

	// If we get an internal error during any of these ops, there's a good chance the webhook is overwhelmed.
	// We'll take the opportunity to rate limit ourselves and try again a few times.

	backoff := wait.Backoff{
		Duration: 5 * time.Second,
		Factor:   1.1,
		Jitter:   0.1,
		Steps:    10,
	}

	err = wait.ExponentialBackoff(backoff, func() (finished bool, err error) {
		_, err = tokenInterface.Update(latestToken)
		if err != nil {
			if apierrors.IsInternalError(err) {
				logrus.Errorf("[%v] internal error while updating token, will backoff and retry: %v", migrateTokensOperation, err)
				return false, err
			}
			return true, fmt.Errorf("[%v] unable to update token: %w", migrateTokensOperation, err)
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("[%v] permanent error when updating token, giving up: %v", migrateTokensOperation, err)
	}

	return nil
}

func migrateTokens(workunit *migrateUserWorkUnit, sc *config.ScaledContext, dryRun bool) {
	tokenInterface := sc.Management.Tokens("")
	dnPrincipalID := activeDirectoryPrefix + workunit.distinguishedName
	for _, userToken := range workunit.activeDirectoryTokens {
		if dryRun {
			logrus.Infof("[%v] DRY RUN: would migrate token '%v' from GUID principal '%v' to DN principal '%v'. "+
				"Would add annotation, %v, and label, %v, to indicate migration status",
				migrateTokensOperation, userToken.Name, userToken.UserPrincipal.Name, dnPrincipalID, adGUIDMigrationAnnotation, adGUIDMigrationLabel)
		} else {
			err := updateToken(tokenInterface, userToken, dnPrincipalID, workunit.guid, workunit.originalUser, workunit.principal)
			if err != nil {
				logrus.Errorf("[%v] error while migrating tokens for user '%v': %v", migrateTokensOperation, workunit.originalUser.Name, err)
			}
		}
	}

	localPrincipalID := localPrefix + workunit.originalUser.Name
	for _, userToken := range workunit.duplicateLocalTokens {
		if dryRun {
			logrus.Infof("[%v] DRY RUN: would migrate Token '%v' from duplicate local user '%v' to original user '%v'. "+
				"Would add annotation, %v, and label, %v, to indicate migration status",
				migrateTokensOperation, userToken.Name, userToken.UserPrincipal.Name, localPrincipalID, adGUIDMigrationAnnotation, adGUIDMigrationLabel)
		} else {
			err := updateToken(tokenInterface, userToken, localPrincipalID, workunit.guid, workunit.originalUser, workunit.principal)
			if err != nil {
				logrus.Errorf("[%v] error while migrating tokens for user '%v': %v", migrateTokensOperation, workunit.originalUser.Name, err)
			}
		}
	}
}
