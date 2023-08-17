/*
Look for any active directory users with a GUID type principal.
Convert these users to a distinguished name instead.
*/

package adunmigration

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/activedirectory"
	"github.com/rancher/rancher/pkg/types/config"
)

const (
	migrateAdUserOperation    = "migrate-ad-user"
	identifyAdUserOperation   = "identify-ad-users"
	migrateTokensOperation    = "migrate-ad-tokens"
	migrateCrtbsOperation     = "migrate-ad-crtbs"
	migratePrtbsOperation     = "migrate-ad-prtbs"
	migrateGrbsOperation      = "migrate-ad-grbs"
	activeDirecotryName       = "activedirectory"
	activeDirectoryScope      = "activedirectory_user"
	activeDirectoryPrefix     = "activedirectory_user://"
	localPrefix               = "local://"
	adGUIDMigrationLabel      = "ad-guid-migration"
	adGUIDMigrationAnnotation = "ad-guid-migration-data"
	adGUIDMigrationPrefix     = "migration-"
	migratedLabelValue        = "migrated"
	migrationPreviousName     = "ad-guid-previous-name"
	AttributeObjectClass      = "objectClass"
	AttributeObjectGUID       = "objectGUID"
	migrateStatusSkipped      = "skippedUsers"
	migrateStatusMissing      = "missingUsers"
	migrateStatusCountSuffix  = "Count"
	migrationStatusPercentage = "percentDone"
	migrationStatusLastUpdate = "statusLastUpdated"
)

type migrateUserWorkUnit struct {
	distinguishedName string
	guid              string
	originalUser      *v3.User
	duplicateUsers    []*v3.User
	principal         *v3.Principal

	activeDirectoryCRTBs []v3.ClusterRoleTemplateBinding
	duplicateLocalCRTBs  []v3.ClusterRoleTemplateBinding

	activeDirectoryPRTBs []v3.ProjectRoleTemplateBinding
	duplicateLocalPRTBs  []v3.ProjectRoleTemplateBinding

	duplicateLocalGRBs []v3.GlobalRoleBinding

	activeDirectoryTokens []v3.Token
	duplicateLocalTokens  []v3.Token
}

type missingUserWorkUnit struct {
	guid           string
	originalUser   *v3.User
	duplicateUsers []*v3.User
}

type skippedUserWorkUnit struct {
	guid         string
	originalUser *v3.User
}

func scaledContext(restConfig *restclient.Config) (*config.ScaledContext, error) {
	sc, err := config.NewScaledContext(*restConfig, nil)
	if err != nil {
		logrus.Errorf("[%v] failed to create scaledContext: %v", migrateAdUserOperation, err)
		return nil, err
	}

	ctx := context.Background()
	err = sc.Start(ctx)
	if err != nil {
		logrus.Errorf("[%v] failed to start scaled context: %v", migrateAdUserOperation, err)
		return nil, err
	}

	return sc, nil
}

// UnmigrateAdGUIDUsersOnce will ensure that the migration script will run only once.  cycle through all users, ctrb, ptrb, tokens and migrate them to an
// appropriate DN-based PrincipalID.
func UnmigrateAdGUIDUsersOnce(sc *config.ScaledContext) error {
	migrationConfigMap, err := sc.Core.ConfigMaps(activedirectory.StatusConfigMapNamespace).GetNamespaced(activedirectory.StatusConfigMapNamespace, activedirectory.StatusConfigMapName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		logrus.Errorf("[%v] unable to check unmigration configmap: %v", migrateAdUserOperation, err)
		logrus.Errorf("[%v] cannot determine if it is safe to proceed. refusing to run", migrateAdUserOperation)
		return nil
	}
	if migrationConfigMap != nil {
		migrationStatus := migrationConfigMap.Data[activedirectory.StatusMigrationField]
		switch migrationStatus {
		case activedirectory.StatusMigrationFinished:
			logrus.Debugf("[%v] ad-guid migration has already been completed, refusing to run again at startup", migrateAdUserOperation)
			return nil
		case activedirectory.StatusMigrationFinishedWithMissing:
			logrus.Infof("[%v] ad-guid migration has already been completed. To clean-up missing users, you can run the utility manually", migrateAdUserOperation)
			return nil
		case activedirectory.StatusMigrationFinishedWithSkipped:
			logrus.Infof("[%v] ad-guid migration has already been completed. To try and resolve skipped users, you can run the utility manually", migrateAdUserOperation)
			return nil
		}

	}
	return UnmigrateAdGUIDUsers(&sc.RESTConfig, false, false)
}

// UnmigrateAdGUIDUsers will cycle through all users, ctrb, ptrb, tokens and migrate them to an
// appropriate DN-based PrincipalID.
func UnmigrateAdGUIDUsers(clientConfig *restclient.Config, dryRun bool, deleteMissingUsers bool) error {
	if dryRun {
		logrus.Infof("[%v] dryRun is true, no objects will be deleted/modified", migrateAdUserOperation)
		deleteMissingUsers = false
	} else if deleteMissingUsers {
		logrus.Infof("[%v] deleteMissingUsers is true, GUID-based users not present in Active Directory will be deleted", migrateAdUserOperation)
	}

	sc, adConfig, err := prepareClientContexts(clientConfig)
	if err != nil {
		return err
	}

	migrationConfigMap, err := sc.Core.ConfigMaps(activedirectory.StatusConfigMapNamespace).GetNamespaced(activedirectory.StatusConfigMapNamespace, activedirectory.StatusConfigMapName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		logrus.Errorf("[%v] unable to check unmigration configmap: %v", migrateAdUserOperation, err)
		logrus.Errorf("[%v] cannot determine if it is safe to proceed. refusing to run", migrateAdUserOperation)
		return nil
	}
	if migrationConfigMap != nil {
		migrationStatus := migrationConfigMap.Data[activedirectory.StatusMigrationField]
		switch migrationStatus {
		case activedirectory.StatusMigrationRunning:
			logrus.Infof("[%v] ad-guid migration is currently running, refusing to run again concurrently", migrateAdUserOperation)
			return nil
		}
	}

	finalStatus := activedirectory.StatusMigrationFinished

	// set the status to running and reset the unmigrated fields
	if !dryRun {
		err = updateMigrationStatus(sc, activedirectory.StatusMigrationField, activedirectory.StatusMigrationRunning)
		if err != nil {
			return fmt.Errorf("unable to update migration status configmap: %v", err)
		}
		updateUnmigratedUsers("", migrateStatusSkipped, true, sc)
		updateUnmigratedUsers("", migrateStatusMissing, true, sc)
		// If we return past this point, no matter how we got there, make sure we update the configmap to clear the
		// status away from "running." If we fail to do this, we block AD-based logins indefinitely.
		defer func(sc *config.ScaledContext, status string) {
			err := updateMigrationStatus(sc, status, finalStatus)
			if err != nil {
				logrus.Errorf("[%v] unable to update migration status configmap: %v", migrateAdUserOperation, err)
			}
		}(sc, activedirectory.StatusMigrationField)

		// Early bail: if the AD configuration is disabled, then we're done! Update the configmap right now and exit.
		if !adConfig.Enabled {
			logrus.Infof("[%v] during unmigration, found that Active Directory is not enabled. nothing to do", migrateAdUserOperation)
			finalStatus = activedirectory.StatusMigrationFinished
			return nil
		}
	}

	users, err := sc.Management.Users("").List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to fetch user list: %v", err)
	}

	usersToMigrate, missingUsers, skippedUsers := identifyMigrationWorkUnits(users, adConfig)
	// If any of the below functions fail, there is either a permissions problem or a more serious issue with the
	// Rancher API. We should bail in this case and not attempt to process users.
	err = collectTokens(&usersToMigrate, sc)
	if err != nil {
		finalStatus = activedirectory.StatusMigrationFailed
		return err
	}
	err = collectCRTBs(&usersToMigrate, sc)
	if err != nil {
		finalStatus = activedirectory.StatusMigrationFailed
		return err
	}
	err = collectPRTBs(&usersToMigrate, sc)
	if err != nil {
		finalStatus = activedirectory.StatusMigrationFailed
		return err
	}
	err = collectGRBs(&usersToMigrate, sc)
	if err != nil {
		finalStatus = activedirectory.StatusMigrationFailed
		return err
	}

	if len(missingUsers) > 0 {
		finalStatus = activedirectory.StatusMigrationFinishedWithMissing
	}
	if len(skippedUsers) > 0 {
		finalStatus = activedirectory.StatusMigrationFinishedWithSkipped
	}

	for _, user := range skippedUsers {
		logrus.Errorf("[%v] unable to migrate user '%v' due to a connection failure; this user will be skipped",
			migrateAdUserOperation, user.originalUser.Name)
		if !dryRun {
			updateUnmigratedUsers(user.originalUser.Name, migrateStatusSkipped, false, sc)
		}
	}
	for _, missingUser := range missingUsers {
		if deleteMissingUsers && !dryRun {
			logrus.Infof("[%v] user '%v' with GUID '%v' does not seem to exist in Active Directory. deleteMissingUsers is true, proceeding to delete this user permanently", migrateAdUserOperation, missingUser.originalUser.Name, missingUser.guid)
			updateUnmigratedUsers(missingUser.originalUser.Name, migrateStatusMissing, false, sc)
			err = sc.Management.Users("").Delete(missingUser.originalUser.Name, &metav1.DeleteOptions{})
			if err != nil {
				logrus.Errorf("[%v] failed to delete missing user '%v' with: %v", migrateAdUserOperation, missingUser.originalUser.Name, err)
			}
		} else {
			logrus.Infof("[%v] User '%v' with GUID '%v' does not seem to exist in Active Directory. this user will be skipped", migrateAdUserOperation, missingUser.originalUser.Name, missingUser.guid)
			if !dryRun {
				updateUnmigratedUsers(missingUser.originalUser.Name, migrateStatusMissing, false, sc)
			}
		}
	}

	for i, userToMigrate := range usersToMigrate {
		// Note: some resources may fail to migrate due to webhook constraints; this applies especially to bindings
		// that refer to disabled templates, as rancher won't allow us to create the replacements. We'll log these
		// errors, but do not consider them to be serious enough to stop processing the remainder of each user's work.
		migrateCRTBs(&userToMigrate, sc, dryRun)
		migratePRTBs(&userToMigrate, sc, dryRun)
		migrateGRBs(&userToMigrate, sc, dryRun)
		migrateTokens(&userToMigrate, sc, dryRun)
		replaceGUIDPrincipalWithDn(userToMigrate.originalUser, userToMigrate.distinguishedName, userToMigrate.guid, dryRun)

		if dryRun {
			describePlannedChanges(userToMigrate)
		} else {
			err = deleteDuplicateUsers(userToMigrate, sc)
			if err == nil {
				updateModifiedUser(userToMigrate, sc)
			}
			percentDone := float64(i+1) / float64(len(usersToMigrate)) * 100
			progress := fmt.Sprintf("%.0f%%", percentDone)
			err = updateMigrationStatus(sc, migrationStatusPercentage, progress)
			if err != nil {
				logrus.Errorf("unable to update migration status: %v", err)
			}
		}
	}

	err = migrateAllowedUserPrincipals(&usersToMigrate, &missingUsers, sc, dryRun, deleteMissingUsers)
	if err != nil {
		finalStatus = activedirectory.StatusMigrationFailed
		return err
	}

	return nil
}

// identifyMigrationWorkUnits locates ActiveDirectory users with GUID and DN based principal IDs and sorts them
// into work units based on whether those users can be located in the upstream Active Directory provider. Specifically:
//
//	usersToMigrate contains GUID-based original users and any duplicates (GUID or DN based) that we wish to merge
//	missingUsers contains GUID-based users who could not be found in Active Directory
//	skippedUsers contains GUID-based users that could not be processed, usually due to an LDAP connection failure
func identifyMigrationWorkUnits(users *v3.UserList, adConfig *v3.ActiveDirectoryConfig) (
	[]migrateUserWorkUnit, []missingUserWorkUnit, []skippedUserWorkUnit) {
	// Note: we *could* make the ldap connection on the spot here, but we're accepting it as a parameter specifically
	// so that this function is easier to test. This setup allows us to mock the ldap connection and thus more easily
	// test unusual Active Directory responses to our searches.

	var usersToMigrate []migrateUserWorkUnit
	var missingUsers []missingUserWorkUnit
	var skippedUsers []skippedUserWorkUnit

	// These assist with quickly identifying duplicates, so we don't have to scan the whole structure each time.
	// We key on guid/dn, and the value is the index of that work unit in the associated table
	knownGUIDWorkUnits := map[string]int{}
	knownGUIDMissingUnits := map[string]int{}
	knownDnWorkUnits := map[string]int{}

	// We'll reuse a shared ldap connection to speed up lookups. We need to declare that here, but we'll defer
	// starting the connection until the first time a lookup is performed
	sharedLConn := sharedLdapConnection{}

	// Now we'll make two passes over the list of all users. First we need to identify any GUID based users, and
	// sort them into "found" and "not found" lists. At this stage we might have GUID-based duplicates, and we'll
	// detect and sort those accordingly
	ldapPermanentlyFailed := false
	logrus.Debugf("[%v] locating GUID-based Active Directory users", identifyAdUserOperation)
	for _, user := range users.Items {
		if !isAdUser(&user) {
			logrus.Debugf("[%v] user '%v' has no AD principals, skipping", identifyAdUserOperation, user.Name)
			continue
		}
		principalID := adPrincipalID(&user)
		logrus.Debugf("[%v] processing AD User '%v' with principal ID: '%v'", identifyAdUserOperation, user.Name, principalID)
		if !isGUID(principalID) {
			logrus.Debugf("[%v] '%v' does not appear to be a GUID-based principal ID, taking no action", identifyAdUserOperation, principalID)
			continue
		}
		guid, err := getExternalID(principalID)

		if err != nil {
			// This really shouldn't be possible to hit, since isGuid will fail to parse anything that would
			// cause getExternalID to choke on the input, but for maximum safety we'll handle it anyway.
			logrus.Errorf("[%v] failed to extract GUID from principal '%v', cannot process user: '%v'", identifyAdUserOperation, err, user.Name)
			continue
		}
		// If our LDAP connection has gone sour, we still need to log this user for reporting
		userCopy := user.DeepCopy()
		if ldapPermanentlyFailed {
			skippedUsers = append(skippedUsers, skippedUserWorkUnit{guid: guid, originalUser: userCopy})
		} else {
			// Check for guid-based duplicates here. If we find one, we don't need to perform an other LDAP lookup.
			if i, exists := knownGUIDWorkUnits[guid]; exists {
				logrus.Debugf("[%v] user %v is GUID-based (%v) and a duplicate of %v",
					identifyAdUserOperation, user.Name, guid, usersToMigrate[i].originalUser.Name)
				// Make sure the oldest duplicate user is selected as the original
				if usersToMigrate[i].originalUser.CreationTimestamp.Time.After(user.CreationTimestamp.Time) {
					usersToMigrate[i].duplicateUsers = append(usersToMigrate[i].duplicateUsers, usersToMigrate[i].originalUser)
					usersToMigrate[i].originalUser = userCopy
				} else {
					usersToMigrate[i].duplicateUsers = append(usersToMigrate[i].duplicateUsers, userCopy)
				}
				continue
			}
			if i, exists := knownGUIDMissingUnits[guid]; exists {
				logrus.Debugf("[%v] user %v is GUID-based (%v) and a duplicate of %v which is known to be missing",
					identifyAdUserOperation, user.Name, guid, missingUsers[i].originalUser.Name)
				// We're less picky about the age of the oldest user here, because we aren't going to deduplicate these
				missingUsers[i].duplicateUsers = append(missingUsers[i].duplicateUsers, userCopy)
				continue
			}
			dn, principal, err := findLdapUserWithRetries(guid, &sharedLConn, adConfig)
			if errors.Is(err, LdapConnectionPermanentlyFailed{}) {
				logrus.Warnf("[%v] LDAP connection has permanently failed! will continue to migrate previously identified users", identifyAdUserOperation)
				skippedUsers = append(skippedUsers, skippedUserWorkUnit{guid: guid, originalUser: userCopy})
				ldapPermanentlyFailed = true
			} else if errors.Is(err, LdapFoundDuplicateGUID{}) {
				logrus.Errorf("[%v] LDAP returned multiple users with GUID '%v'. this should not be possible, and may indicate a configuration error! this user will be skipped", identifyAdUserOperation, guid)
				skippedUsers = append(skippedUsers, skippedUserWorkUnit{guid: guid, originalUser: userCopy})
			} else if errors.Is(err, LdapErrorNotFound{}) {
				logrus.Debugf("[%v] user %v is GUID-based (%v) and the Active Directory server doesn't know about it. marking it as missing", identifyAdUserOperation, user.Name, guid)
				knownGUIDMissingUnits[guid] = len(missingUsers)
				missingUsers = append(missingUsers, missingUserWorkUnit{guid: guid, originalUser: userCopy})
			} else {
				logrus.Debugf("[%v] user %v is GUID-based (%v) and the Active Directory server knows it by the Distinguished Name '%v'", identifyAdUserOperation, user.Name, guid, dn)
				knownGUIDWorkUnits[guid] = len(usersToMigrate)
				knownDnWorkUnits[dn] = len(usersToMigrate)
				var emptyDuplicateList []*v3.User
				usersToMigrate = append(usersToMigrate, migrateUserWorkUnit{guid: guid, distinguishedName: dn, principal: principal, originalUser: userCopy, duplicateUsers: emptyDuplicateList})
			}
		}
	}

	if sharedLConn.isOpen {
		sharedLConn.lConn.Close()
	}

	if len(usersToMigrate) == 0 {
		logrus.Debugf("[%v] found 0 users in need of migration, exiting without checking for DN-based duplicates", identifyAdUserOperation)
		return usersToMigrate, missingUsers, skippedUsers
	}

	// Now for the second pass, we need to identify DN-based users, and see if they are duplicates of any of the GUID
	// users that we found in the first pass. We'll prefer the oldest user as the originalUser object, this will be
	// the one we keep when we resolve duplicates later.
	logrus.Debugf("[%v] locating any DN-based Active Directory users", identifyAdUserOperation)
	for _, user := range users.Items {
		if !isAdUser(&user) {
			logrus.Debugf("[%v] user '%v' has no AD principals, skipping", identifyAdUserOperation, user.Name)
			continue
		}
		principalID := adPrincipalID(&user)
		logrus.Debugf("[%v] processing AD User '%v' with principal ID: '%v'", identifyAdUserOperation, user.Name, principalID)
		if isGUID(principalID) {
			logrus.Debugf("[%v] '%v' does not appear to be a DN-based principal ID, taking no action", identifyAdUserOperation, principalID)
			continue
		}
		dn, err := getExternalID(principalID)
		if err != nil {
			logrus.Errorf("[%v] failed to extract DN from principal '%v', cannot process user: '%v'", identifyAdUserOperation, err, user.Name)
			continue
		}
		if i, exists := knownDnWorkUnits[dn]; exists {
			logrus.Debugf("[%v] user %v is DN-based (%v), and a duplicate of %v",
				identifyAdUserOperation, user.Name, dn, usersToMigrate[i].originalUser.Name)
			// Make sure the oldest duplicate user is selected as the original
			userCopy := user.DeepCopy()
			if usersToMigrate[i].originalUser.CreationTimestamp.Time.After(user.CreationTimestamp.Time) {
				usersToMigrate[i].duplicateUsers = append(usersToMigrate[i].duplicateUsers, usersToMigrate[i].originalUser)
				usersToMigrate[i].originalUser = userCopy
			} else {
				usersToMigrate[i].duplicateUsers = append(usersToMigrate[i].duplicateUsers, userCopy)
			}
		}
	}

	return usersToMigrate, missingUsers, skippedUsers
}

func workUnitContainsName(workunit *migrateUserWorkUnit, name string) bool {
	if workunit.originalUser.Name == name {
		return true
	}
	for _, duplicateLocalUser := range workunit.duplicateUsers {
		if duplicateLocalUser.Name == name {
			return true
		}
	}
	return false
}

func updateMigrationStatus(sc *config.ScaledContext, status string, value string) error {
	cm, err := sc.Core.ConfigMaps(activedirectory.StatusConfigMapNamespace).Get(activedirectory.StatusConfigMapName, metav1.GetOptions{})
	if err != nil {
		// Create a new ConfigMap if it doesn't exist
		if !apierrors.IsNotFound(err) {
			return err
		}
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      activedirectory.StatusConfigMapName,
				Namespace: activedirectory.StatusConfigMapNamespace,
			},
		}
	}
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	cm.Data[status] = value
	cm.Data[migrationStatusLastUpdate] = metav1.Now().Format(time.RFC3339)

	if _, err := sc.Core.ConfigMaps(activedirectory.StatusConfigMapNamespace).Update(cm); err != nil {
		// If the ConfigMap does not exist, create it
		if apierrors.IsNotFound(err) {
			_, err = sc.Core.ConfigMaps(activedirectory.StatusConfigMapNamespace).Create(cm)
			if err != nil {
				return fmt.Errorf("[%v] unable to create migration status configmap: %v", migrateAdUserOperation, err)
			}
		}
	}
	err = updateADConfigMigrationStatus(cm.Data, sc)
	if err != nil {
		return fmt.Errorf("unable to update AuthConfig status: %v", err)
	}
	return nil
}

// updateUnmigratedUsers will add a user to the list for the specified migration status in the migration status configmap.
// If reset is set to true, it will empty the list.
func updateUnmigratedUsers(user string, status string, reset bool, sc *config.ScaledContext) {
	cm, err := sc.Core.ConfigMaps(activedirectory.StatusConfigMapNamespace).Get(activedirectory.StatusConfigMapName, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("[%v] unable to fetch configmap to update %v users: %v", migrateAdUserOperation, status, err)
	}
	var currentList string
	if reset {
		delete(cm.Data, status)
		delete(cm.Data, status+migrateStatusCountSuffix)
	} else {
		currentList = cm.Data[status]
		if currentList == "" {
			currentList = currentList + user
		} else {
			currentList = currentList + "," + user
		}
		count := strconv.Itoa(len(strings.Split(currentList, ",")))
		cm.Data[status+migrateStatusCountSuffix] = count
		cm.Data[status] = currentList
	}

	cm.Data[migrationStatusLastUpdate] = metav1.Now().Format(time.RFC3339)
	if _, err := sc.Core.ConfigMaps(activedirectory.StatusConfigMapNamespace).Update(cm); err != nil {
		if err != nil {
			logrus.Errorf("[%v] unable to update migration status configmap: %v", migrateAdUserOperation, err)
		}
	}
	err = updateADConfigMigrationStatus(cm.Data, sc)
	if err != nil {
		logrus.Errorf("unable to update AuthConfig status: %v", err)
	}
}
