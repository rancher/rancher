package adunmigration

import (
	"bytes"
	"crypto/x509"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	ldapv3 "github.com/go-ldap/ldap/v3"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/common/ldap"
	v3client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/types/config"
)

// Rancher 2.7.5 serialized binary GUIDs from LDAP using this pattern, so this
// is what we should match. Notably this differs from Active Directory GUID
// strings, which have dashes and braces as delimiters.
var validRancherGUIDPattern = regexp.MustCompile("^[0-9a-f]+$")

type LdapErrorNotFound struct{}

// Error provides a string representation of an LdapErrorNotFound
func (e LdapErrorNotFound) Error() string {
	return "ldap query returned no results"
}

// LdapFoundDuplicateGUID indicates either a configuration error or
// a corruption on the Active Directory side. In theory it should never
// be possible when talking to a real Active Directory server, but just
// in case we detect and handle it anyway.
type LdapFoundDuplicateGUID struct{}

// Error provides a string representation of an LdapErrorNotFound
func (e LdapFoundDuplicateGUID) Error() string {
	return "ldap query returned multiple users for the same GUID"
}

type LdapConnectionPermanentlyFailed struct{}

// Error provides a string representation of an LdapConnectionPermanentlyFailed
func (e LdapConnectionPermanentlyFailed) Error() string {
	return "ldap search failed to connect after exhausting maximum retry attempts"
}

type sharedLdapConnection struct {
	lConn    *ldapv3.Conn
	isOpen   bool
	adConfig *v3.ActiveDirectoryConfig
}

type retryableLdapConnection interface {
	findLdapUserWithRetries(guid string) (string, *v3.Principal, error)
}

func (sLConn sharedLdapConnection) findLdapUserWithRetries(guid string) (string, *v3.Principal, error) {
	// These settings range from 2 seconds for minor blips to around a full minute for repeated failures
	backoff := wait.Backoff{
		Duration: 2 * time.Second,
		Factor:   1.5, // duration multiplied by this for each retry
		Jitter:   0.1, // random variance, just in case other parts of rancher are using LDAP while we work
		Steps:    10,  // number of retries before we consider this failure to be permanent
	}

	var distinguishedName string
	var principal *v3.Principal
	err := wait.ExponentialBackoff(backoff, func() (finished bool, err error) {
		if !sLConn.isOpen {
			sLConn.lConn, err = ldapConnection(sLConn.adConfig)
			if err != nil {
				logrus.Warnf("[%v] LDAP connection failed: '%v', retrying...", migrateAdUserOperation, err)
				return false, err
			}
			sLConn.isOpen = true
		}

		distinguishedName, principal, err = findLdapUser(guid, sLConn.lConn, sLConn.adConfig)
		if err == nil || errors.Is(err, LdapErrorNotFound{}) || errors.Is(err, LdapFoundDuplicateGUID{}) {
			return true, err
		}

		// any other error type almost certainly indicates a connection failure. Close and re-open the connection
		// before retrying
		logrus.Warnf("[%v] LDAP connection failed: '%v', retrying...", migrateAdUserOperation, err)
		sLConn.lConn.Close()
		sLConn.isOpen = false

		return false, err
	})

	return distinguishedName, principal, err
}

func ldapConnection(config *v3.ActiveDirectoryConfig) (*ldapv3.Conn, error) {
	caPool, err := newCAPool(config.Certificate)
	if err != nil {
		return nil, fmt.Errorf("unable to create caPool: %v", err)
	}

	servers := config.Servers
	TLS := config.TLS
	port := config.Port
	connectionTimeout := config.ConnectionTimeout
	startTLS := config.StartTLS

	ldapConn, err := ldap.NewLDAPConn(servers, TLS, startTLS, port, connectionTimeout, caPool)
	if err != nil {
		return nil, err
	}

	serviceAccountUsername := ldap.GetUserExternalID(config.ServiceAccountUsername, config.DefaultLoginDomain)
	err = ldapConn.Bind(serviceAccountUsername, config.ServiceAccountPassword)
	if err != nil {
		return nil, err
	}
	return ldapConn, nil
}

// EscapeUUID will take a UUID string in string form and will add backslashes to every 2nd character.
// The returned result is the string that needs to be added to the LDAP filter to properly filter
// by objectGUID, which is stored as binary data.
func escapeUUID(s string) string {
	var buffer bytes.Buffer
	var n1 = 1
	var l1 = len(s) - 1
	buffer.WriteRune('\\')
	for i, r := range s {
		buffer.WriteRune(r)
		if i%2 == n1 && i != l1 {
			buffer.WriteRune('\\')
		}
	}
	return buffer.String()
}

func findLdapUser(guid string, lConn *ldapv3.Conn, adConfig *v3.ActiveDirectoryConfig) (string, *v3.Principal, error) {
	query := fmt.Sprintf("(&(%v=%v)(%v=%v))", AttributeObjectClass, adConfig.UserObjectClass, AttributeObjectGUID, escapeUUID(guid))
	search := ldapv3.NewSearchRequest(adConfig.UserSearchBase, ldapv3.ScopeWholeSubtree, ldapv3.NeverDerefAliases,
		0, 0, false,
		query, adConfig.GetUserSearchAttributes("memberOf", "objectClass"), nil)

	result, err := lConn.Search(search)
	if err != nil {
		return "", nil, err
	}

	if len(result.Entries) < 1 {
		return "", nil, LdapErrorNotFound{}
	} else if len(result.Entries) > 1 {
		return "", nil, LdapFoundDuplicateGUID{}
	}

	entry := result.Entries[0]
	distinguishedName := entry.DN
	principal, err := ldap.AttributesToPrincipal(entry.Attributes, distinguishedName, activeDirectoryScope, activeDirecotryName,
		adConfig.UserObjectClass, adConfig.UserNameAttribute, adConfig.UserLoginAttribute, adConfig.GroupObjectClass, adConfig.GroupNameAttribute)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate principal from ldap attributes")
	}

	return entry.DN, principal, nil
}

func adConfiguration(sc *config.ScaledContext) (*v3.ActiveDirectoryConfig, error) {
	authConfigs := sc.Management.AuthConfigs("")
	secrets := sc.Core.Secrets("")

	authConfigObj, err := authConfigs.ObjectClient().UnstructuredClient().Get("activedirectory", metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("[%v] failed to obtain activedirectory authConfigObj: %v", migrateAdUserOperation, err)
		return nil, err
	}

	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		logrus.Errorf("[%v] failed to retrieve ActiveDirectoryConfig, cannot read k8s Unstructured data %v", migrateAdUserOperation, err)
		return nil, err
	}
	storedADConfigMap := u.UnstructuredContent()

	storedADConfig := &v3.ActiveDirectoryConfig{}
	err = common.Decode(storedADConfigMap, storedADConfig)
	if err != nil {
		logrus.Errorf("[%v] errors while decoding stored AD config: %v", migrateAdUserOperation, err)
		return nil, err
	}

	metadataMap, ok := storedADConfigMap["metadata"].(map[string]interface{})
	if !ok {
		logrus.Errorf("[%v] failed to retrieve ActiveDirectoryConfig, (second step), cannot read k8s Unstructured data %v", migrateAdUserOperation, err)
		return nil, err
	}

	typemeta := &metav1.ObjectMeta{}
	err = common.Decode(metadataMap, typemeta)
	if err != nil {
		logrus.Errorf("[%v] errors while decoding typemeta: %v", migrateAdUserOperation, err)
		return nil, err
	}

	storedADConfig.ObjectMeta = *typemeta

	if storedADConfig.ServiceAccountPassword != "" {
		value, err := common.ReadFromSecret(secrets, storedADConfig.ServiceAccountPassword,
			strings.ToLower(v3client.ActiveDirectoryConfigFieldServiceAccountPassword))
		if err != nil {
			return nil, err
		}
		storedADConfig.ServiceAccountPassword = value
	}

	return storedADConfig, nil
}

func newCAPool(cert string) (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}
	pool.AppendCertsFromPEM([]byte(cert))
	return pool, nil
}

// prepareClientContexts sets up a scaled context with the ability to read users and AD configuration data
func prepareClientContexts(clientConfig *restclient.Config) (*config.ScaledContext, *v3.ActiveDirectoryConfig, error) {
	var restConfig *restclient.Config
	var err error
	if clientConfig != nil {
		restConfig = clientConfig
	} else {
		restConfig, err = clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
		if err != nil {
			logrus.Errorf("[%v] failed to build the cluster config: %v", migrateAdUserOperation, err)
			return nil, nil, err
		}
	}

	sc, err := scaledContext(restConfig)
	if err != nil {
		logrus.Errorf("[%v] failed to create scaled context: %v", migrateAdUserOperation, err)
		return nil, nil, err
	}
	adConfig, err := adConfiguration(sc)
	if err != nil {
		logrus.Errorf("[%v] failed to acquire ad configuration: %v", migrateAdUserOperation, err)
		return nil, nil, err
	}

	return sc, adConfig, nil
}

func isGUID(principalID string) bool {
	parts := strings.Split(principalID, "://")
	if len(parts) != 2 {
		logrus.Errorf("[%v] failed to parse invalid PrincipalID: %v", identifyAdUserOperation, principalID)
		return false
	}
	return validRancherGUIDPattern.MatchString(parts[1])
}

func updateADConfigMigrationStatus(status map[string]string, sc *config.ScaledContext) error {
	authConfigObj, err := sc.Management.AuthConfigs("").ObjectClient().UnstructuredClient().Get("activedirectory", metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("[%v] failed to obtain activedirecotry authConfigObj: %v", migrateAdUserOperation, err)
		return err
	}

	storedADConfig, ok := authConfigObj.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("[%v] expected unstructured authconfig, got %T", migrateAdUserOperation, authConfigObj)
	}

	// Update annotations with migration status
	annotations := storedADConfig.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	for annotation, value := range status {
		// We do not mirror the actual user lists to the AuthConfig
		if annotation != migrateStatusSkipped && annotation != migrateStatusMissing {
			annotations[adGUIDMigrationPrefix+annotation] = value
		}
	}
	storedADConfig.SetAnnotations(annotations)

	// Update the AuthConfig object using the unstructured client
	_, err = sc.Management.AuthConfigs("").ObjectClient().UnstructuredClient().Update(storedADConfig.GetName(), storedADConfig)
	if err != nil {
		return fmt.Errorf("failed to update authConfig object: %v", err)
	}

	return nil
}

func migrateAllowedUserPrincipals(workunits *[]migrateUserWorkUnit, missingUsers *[]missingUserWorkUnit, sc *config.ScaledContext, dryRun bool, deleteMissingUsers bool) error {
	// this needs its own copy of the ad config, decoded with the ldap credentials fetched, so do that here
	originalAdConfig, err := adConfiguration(sc)
	if err != nil {
		return fmt.Errorf("[%v] failed to obtain activedirectory config: %v", migrateAdUserOperation, err)
	}

	authConfigObj, err := sc.Management.AuthConfigs("").ObjectClient().UnstructuredClient().Get("activedirectory", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("[%v] failed to obtain activedirectory authConfigObj: %v", migrateAdUserOperation, err)
	}

	// Create an empty unstructured object to hold the decoded JSON
	storedADConfig, ok := authConfigObj.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("[%v] expected unstructured authconfig, got %T", migrateAdUserOperation, authConfigObj)
	}

	unstructuredMap := storedADConfig.UnstructuredContent()
	unstructuredMaybeList := unstructuredMap["allowedPrincipalIds"]
	listOfMaybeStrings, ok := unstructuredMaybeList.([]interface{})
	if !ok {
		return fmt.Errorf("[%v] expected list for allowed principal ids, got %T", migrateAdUserOperation, unstructuredMaybeList)
	}

	adWorkUnitsByPrincipal := map[string]int{}
	for i, workunit := range *workunits {
		adWorkUnitsByPrincipal[activeDirectoryPrefix+workunit.guid] = i
	}
	missingWorkUnitsByPrincipal := map[string]int{}
	for i, workunit := range *missingUsers {
		adWorkUnitsByPrincipal[activeDirectoryPrefix+workunit.guid] = i
	}

	// we can deduplicate this list while we're at it, so we don't accidentally end up with twice the DNs
	var newPrincipalIDs []string
	var knownDnIDs = map[string]string{}

	// because we might process users in this list that have never logged in, we may need to perform LDAP
	// lookups on the spot to see what their associated DN should be
	sharedLConn := sharedLdapConnection{adConfig: originalAdConfig}

	for _, item := range listOfMaybeStrings {
		principalID, ok := item.(string)
		if !ok {
			// ... what? we got a non-string?
			// this is weird enough that we should consider it a hard failure for investigation
			return fmt.Errorf("[%v] expected string for allowed principal id, found instead %T", migrateAdUserOperation, item)
		}

		scope, err := getScope(principalID)
		if err != nil {
			logrus.Errorf("[%v] found invalid principal ID in allowed user list, refusing to process: %v", migrateAdUserOperation, err)
			newPrincipalIDs = append(newPrincipalIDs, principalID)
		}
		if scope != activeDirectoryScope {
			newPrincipalIDs = append(newPrincipalIDs, principalID)
		} else {
			if !isGUID(principalID) {
				// This must be a DN-based principal; add it to the new list
				knownDnIDs[principalID] = principalID
			} else {
				if j, exists := adWorkUnitsByPrincipal[principalID]; exists {
					// This user is known and was just migrated to DN, so add their DN-based principal to the list
					newPrincipalID := activeDirectoryPrefix + (*workunits)[j].distinguishedName
					knownDnIDs[newPrincipalID] = newPrincipalID
				} else if _, exists := missingWorkUnitsByPrincipal[principalID]; exists {
					// This user is known to be missing, so we don't need to perform an LDAP lookup, we can just
					// action accordingly
					if !deleteMissingUsers {
						newPrincipalIDs = append(newPrincipalIDs, principalID)
					}
				} else {
					// We didn't process a user object for this GUID-based user. We need to perform an ldap
					// lookup on the spot and figure out if they have an associated DN
					guid, err := getExternalID(principalID)
					if err != nil {
						// this shouldn't be reachable, as getScope will fail first, but just for consistency...
						logrus.Errorf("[%v] found invalid principal ID in allowed user list, refusing to process: %v", migrateAdUserOperation, err)
						newPrincipalIDs = append(newPrincipalIDs, principalID)
					} else {
						dn, _, err := sharedLConn.findLdapUserWithRetries(guid)
						if errors.Is(err, LdapErrorNotFound{}) {
							if !deleteMissingUsers {
								newPrincipalIDs = append(newPrincipalIDs, principalID)
							}
						} else if err != nil {
							// Whelp; keep this one as-is and yell about it
							logrus.Errorf("[%v] ldap error when checking distinguished name for guid-based principal %v, skipping: %v", migrateAdUserOperation, principalID, err)
							newPrincipalIDs = append(newPrincipalIDs, principalID)
						} else {
							newPrincipalID := activeDirectoryPrefix + dn
							knownDnIDs[newPrincipalID] = newPrincipalID
						}
					}
				}
			}
		}
	}

	// Now that we're through processing the list and dealing with any duplicates, append the new DN-based principals
	// to the end of the list
	for _, principalID := range knownDnIDs {
		newPrincipalIDs = append(newPrincipalIDs, principalID)
	}

	if !dryRun {
		unstructuredMap["allowedPrincipalIds"] = newPrincipalIDs
		storedADConfig.SetUnstructuredContent(unstructuredMap)

		_, err = sc.Management.AuthConfigs("").ObjectClient().UnstructuredClient().Update("activedirectory", storedADConfig)
	} else {
		logrus.Infof("[%v] DRY RUN: new allowed user list will contain these principal IDs:", migrateAdUserOperation)
		for _, principalID := range newPrincipalIDs {
			logrus.Infof("[%v] DRY RUN:   '%v'", migrateAdUserOperation, principalID)
		}
	}
	return err
}
