package adunmigration

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
)

func describePlannedChanges(workunit migrateUserWorkUnit) {
	logrus.Infof("DRY RUN: changes to user '%v' have NOT been saved.", workunit.originalUser.Name)
	if len(workunit.duplicateUsers) > 0 {
		logrus.Infof("[%v] DRY RUN: duplicate users were identified", migrateAdUserOperation)
		for _, duplicateUser := range workunit.duplicateUsers {
			logrus.Infof("[%v] DRY RUN: would DELETE user %v", migrateAdUserOperation, duplicateUser.Name)
		}
	}
}

func deleteDuplicateUsers(workunit migrateUserWorkUnit, sc *config.ScaledContext) error {
	for _, duplicateUser := range workunit.duplicateUsers {
		err := sc.Management.Users("").Delete(duplicateUser.Name, &metav1.DeleteOptions{})
		if err != nil {
			logrus.Errorf("[%v] failed to delete dupliate user '%v' with: %v", migrateAdUserOperation, workunit.originalUser.Name, err)
			// If the duplicate deletion has failed for some reason, it is NOT safe to save the modified user, as
			// this may result in a duplicate AD principal ID. Notify and skip.

			logrus.Errorf("[%v] cannot safely save modifications to user %v, skipping", migrateAdUserOperation, workunit.originalUser.Name)
			return errors.Errorf("failed to delete duplicate users")
		}
		logrus.Infof("[%v] deleted duplicate user %v", migrateAdUserOperation, duplicateUser.Name)
	}
	return nil
}

func updateModifiedUser(workunit migrateUserWorkUnit, sc *config.ScaledContext) {
	workunit.originalUser.Annotations[adGUIDMigrationAnnotation] = workunit.guid
	workunit.originalUser.Labels[adGUIDMigrationLabel] = migratedLabelValue
	_, err := sc.Management.Users("").Update(workunit.originalUser)
	if err != nil {
		logrus.Errorf("[%v] failed to save modified user '%v' with: %v", migrateAdUserOperation, workunit.originalUser.Name, err)
	}
	logrus.Infof("[%v] user %v was successfully migrated", migrateAdUserOperation, workunit.originalUser.Name)
}

func replaceGUIDPrincipalWithDn(user *v3.User, dn string, guid string, dryRun bool) {
	// It's weird for a single user to have more than just an AD and a Local principal ID, but it *can* happen
	// if Rancher has used more than one auth provider over its history. Here we'll keep all principal IDs
	// that are unrelated to AD
	var principalIDs []string
	for _, principalID := range user.PrincipalIDs {
		if !strings.HasPrefix(principalID, activeDirectoryPrefix) {
			principalIDs = append(principalIDs, principalID)
		}
	}
	principalIDs = append(principalIDs, activeDirectoryPrefix+dn)

	if dryRun {
		// In dry run mode we will merely print the computed list and leave the original user object alone
		logrus.Infof("[%v] DRY RUN: User '%v' with GUID '%v' would have new principals:", migrateAdUserOperation,
			guid, user.Name)
		for _, principalID := range principalIDs {
			logrus.Infof("[%v] DRY RUN:    '%v'", migrateAdUserOperation, principalID)
		}
	} else {
		user.PrincipalIDs = principalIDs
		logrus.Debugf("[%v] User '%v' with GUID %v will have new principals:", migrateAdUserOperation,
			guid, user.Name)
		for _, principalID := range user.PrincipalIDs {
			logrus.Debugf("[%v]     '%v'", migrateAdUserOperation, principalID)
		}
	}
}

func isAdUser(user *v3.User) bool {
	for _, principalID := range user.PrincipalIDs {
		if strings.HasPrefix(principalID, activeDirectoryPrefix) {
			return true
		}
	}
	return false
}

func adPrincipalID(user *v3.User) string {
	for _, principalID := range user.PrincipalIDs {
		if strings.HasPrefix(principalID, activeDirectoryPrefix) {
			return principalID
		}
	}
	return ""
}

func localPrincipalID(user *v3.User) string {
	for _, principalID := range user.PrincipalIDs {
		if strings.HasPrefix(principalID, localPrefix) {
			return principalID
		}
	}
	return ""
}

func getExternalID(principalID string) (string, error) {
	parts := strings.Split(principalID, "://")
	if len(parts) != 2 {
		return "", fmt.Errorf("[%v] failed to parse invalid principalID: %v", identifyAdUserOperation, principalID)
	}
	return parts[1], nil
}

func getScope(principalID string) (string, error) {
	parts := strings.Split(principalID, "://")
	if len(parts) != 2 {
		return "", fmt.Errorf("[%v] failed to parse invalid principalID: %v", identifyAdUserOperation, principalID)
	}
	return parts[0], nil
}
