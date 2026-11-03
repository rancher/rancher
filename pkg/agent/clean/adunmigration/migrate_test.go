package adunmigration

import (
	"errors"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	wranglerfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/sirupsen/logrus"
	logrusTest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	restclient "k8s.io/client-go/rest"
)

func TestWorkUnitContainsName(t *testing.T) {
	t.Parallel()

	originalUserName := "u-fydcaomakf"
	duplicateUserName := "u-quuknbiweg"

	workunit := migrateUserWorkUnit{
		originalUser: &v3.User{ObjectMeta: metav1.ObjectMeta{Name: originalUserName}},
		duplicateUsers: []*v3.User{
			{ObjectMeta: metav1.ObjectMeta{Name: duplicateUserName}},
		},
	}

	tests := []struct {
		name       string
		query      string
		wantResult bool
	}{
		{
			name:       "No match returns false",
			query:      "u-nonexistent",
			wantResult: false,
		},
		{
			name:       "Original user match returns true",
			query:      originalUserName,
			wantResult: true,
		},
		{
			name:       "Duplicate user match returns true",
			query:      duplicateUserName,
			wantResult: true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := workUnitContainsName(&workunit, test.query)
			assert.Equal(t, test.wantResult, result)
		})
	}
}

type mockLdapAlwaysSucceed struct {
}

func (sLConn mockLdapAlwaysSucceed) findLdapUserWithRetries(guid string) (string, *v3.Principal, error) {
	return testDn, &v3.Principal{}, nil
}

func TestWorkUnitIdentification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		users              []v3.User
		wantUsersToMigrate int
		wantDuplicateUsers int
	}{
		{
			name: "Guid-based user, which exists in AD, is identified",
			users: []v3.User{
				{ObjectMeta: metav1.ObjectMeta{Name: testGuidLocalName}, PrincipalIDs: []string{testGuidPrincipal, testGuidLocalPrincipal}},
			},
			wantUsersToMigrate: 1,
			wantDuplicateUsers: 0,
		},
		{
			name: "Guid-based duplicate users are recognized",
			users: []v3.User{
				{ObjectMeta: metav1.ObjectMeta{Name: testGuidLocalName}, PrincipalIDs: []string{testGuidPrincipal, testGuidLocalPrincipal}},
				{ObjectMeta: metav1.ObjectMeta{Name: testDuplicateGuidLocalName}, PrincipalIDs: []string{testGuidPrincipal, testDuplicateGuidLocalPrincipal}},
			},
			wantUsersToMigrate: 1,
			wantDuplicateUsers: 1,
		},
		{
			name: "Dn-based duplicate users are recognized",
			users: []v3.User{
				{ObjectMeta: metav1.ObjectMeta{Name: testGuidLocalName}, PrincipalIDs: []string{testGuidPrincipal, testGuidLocalPrincipal}},
				{ObjectMeta: metav1.ObjectMeta{Name: testDnLocalName}, PrincipalIDs: []string{testDnPrincipal, testDnLocalPrincipal}},
			},
			wantUsersToMigrate: 1,
			wantDuplicateUsers: 1,
		},
		{
			name: "Dn-based users without GUID-based duplicates are ignored",
			users: []v3.User{
				{ObjectMeta: metav1.ObjectMeta{Name: testDnLocalName}, PrincipalIDs: []string{testDnPrincipal, testDnLocalPrincipal}},
			},
			wantUsersToMigrate: 0,
			wantDuplicateUsers: 0,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ldapConnection := mockLdapAlwaysSucceed{}
			userList := v3.UserList{Items: test.users}
			usersToMigrate, missingUsers, skippedUsers := identifyMigrationWorkUnits(&userList, ldapConnection)
			assert.Equal(t, test.wantUsersToMigrate, len(usersToMigrate), "unexpected count for users to migrate")
			assert.Equal(t, 0, len(missingUsers), "unexpected count for missing users")
			assert.Equal(t, 0, len(skippedUsers), "unexpected count for skipped users")
			if len(usersToMigrate) > 0 {
				assert.Equal(t, test.wantDuplicateUsers, len(usersToMigrate[0].duplicateUsers), "unexpected duplicate users for first workunit")
			}
		})
	}
}

func TestWorkUnitDuplicateAgeResolution(t *testing.T) {
	t.Parallel()

	olderTime, err := time.Parse("2006-Jan-02", "2000-Jan-01")
	assert.NoError(t, err, "time itself cannot be trusted, all hope is lost")
	newerTime, err := time.Parse("2006-Jan-02", "2001-Jan-01")
	assert.NoError(t, err, "time itself cannot be trusted, all hope is lost")

	tests := []struct {
		name               string
		users              []v3.User
		wantUsersToMigrate int
		wantDuplicateUsers int
		wantOriginalName   string
		wantDuplicateName  string
	}{
		{
			name: "Guid-based duplicate selects oldest user as original",
			users: []v3.User{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              testGuidLocalName,
						CreationTimestamp: metav1.Time{Time: olderTime},
					},
					PrincipalIDs: []string{testGuidPrincipal, testGuidLocalPrincipal},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              testDuplicateGuidLocalName,
						CreationTimestamp: metav1.Time{Time: newerTime},
					},
					PrincipalIDs: []string{testGuidPrincipal, testDuplicateGuidLocalPrincipal},
				},
			},
			wantUsersToMigrate: 1,
			wantDuplicateUsers: 1,
			wantOriginalName:   testGuidLocalName,
			wantDuplicateName:  testDuplicateGuidLocalName,
		},
		{
			name: "Guid-based duplicate, order reversed, still selects oldest user as original",
			users: []v3.User{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              testDuplicateGuidLocalName,
						CreationTimestamp: metav1.Time{Time: newerTime},
					},
					PrincipalIDs: []string{testGuidPrincipal, testDuplicateGuidLocalPrincipal},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              testGuidLocalName,
						CreationTimestamp: metav1.Time{Time: olderTime},
					},
					PrincipalIDs: []string{testGuidPrincipal, testGuidLocalPrincipal},
				},
			},
			wantUsersToMigrate: 1,
			wantDuplicateUsers: 1,
			wantOriginalName:   testGuidLocalName,
			wantDuplicateName:  testDuplicateGuidLocalName,
		},
		{
			name: "Newer Dn-based duplicate of older GUID-based user selects GUID-based user as original",
			users: []v3.User{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              testGuidLocalName,
						CreationTimestamp: metav1.Time{Time: olderTime},
					},
					PrincipalIDs: []string{testGuidPrincipal, testGuidLocalPrincipal},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              testDnLocalName,
						CreationTimestamp: metav1.Time{Time: newerTime},
					},
					PrincipalIDs: []string{testDnPrincipal, testDnLocalPrincipal},
				},
			},
			wantUsersToMigrate: 1,
			wantDuplicateUsers: 1,
			wantOriginalName:   testGuidLocalName,
			wantDuplicateName:  testDnLocalName,
		},
		{
			name: "Newer Dn-based duplicate of older GUID-based user, order reversed, still selects GUID-based user as original",
			users: []v3.User{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              testDnLocalName,
						CreationTimestamp: metav1.Time{Time: newerTime},
					},
					PrincipalIDs: []string{testDnPrincipal, testDnLocalPrincipal},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              testGuidLocalName,
						CreationTimestamp: metav1.Time{Time: olderTime},
					},
					PrincipalIDs: []string{testGuidPrincipal, testGuidLocalPrincipal},
				},
			},
			wantUsersToMigrate: 1,
			wantDuplicateUsers: 1,
			wantOriginalName:   testGuidLocalName,
			wantDuplicateName:  testDnLocalName,
		},
		{
			name: "Newer GUID-based duplicate of older Dn-based user selects Dn-based user as original",
			users: []v3.User{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              testDnLocalName,
						CreationTimestamp: metav1.Time{Time: olderTime},
					},
					PrincipalIDs: []string{testDnPrincipal, testDnLocalPrincipal},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              testGuidLocalName,
						CreationTimestamp: metav1.Time{Time: newerTime},
					},
					PrincipalIDs: []string{testGuidPrincipal, testGuidLocalPrincipal},
				},
			},
			wantUsersToMigrate: 1,
			wantDuplicateUsers: 1,
			wantOriginalName:   testDnLocalName,
			wantDuplicateName:  testGuidLocalName,
		},
		{
			name: "Newer GUID-based duplicate of older Dn-based user, order reversed, still selects Dn-based user as original",
			users: []v3.User{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              testGuidLocalName,
						CreationTimestamp: metav1.Time{Time: newerTime},
					},
					PrincipalIDs: []string{testGuidPrincipal, testGuidLocalPrincipal},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              testDnLocalName,
						CreationTimestamp: metav1.Time{Time: olderTime},
					},
					PrincipalIDs: []string{testDnPrincipal, testDnLocalPrincipal},
				},
			},
			wantUsersToMigrate: 1,
			wantDuplicateUsers: 1,
			wantOriginalName:   testDnLocalName,
			wantDuplicateName:  testGuidLocalName,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ldapConnection := mockLdapAlwaysSucceed{}
			userList := v3.UserList{Items: test.users}
			usersToMigrate, _, _ := identifyMigrationWorkUnits(&userList, ldapConnection)
			assert.Equal(t, test.wantUsersToMigrate, len(usersToMigrate), "unexpected count for users to migrate")
			if len(usersToMigrate) > 0 {
				assert.Equal(t, test.wantDuplicateUsers, len(usersToMigrate[0].duplicateUsers), "unexpected duplicate users for first workunit")
				if len(usersToMigrate[0].duplicateUsers) > 0 {
					assert.Equal(t, test.wantOriginalName, usersToMigrate[0].originalUser.Name, "wrong user identified as original")
					assert.Equal(t, test.wantDuplicateName, usersToMigrate[0].duplicateUsers[0].Name, "wrong user identified as duplicate")
				}
			}
		})
	}
}

type mockLdapNeverSucceed struct {
}

func (sLConn mockLdapNeverSucceed) findLdapUserWithRetries(guid string) (string, *v3.Principal, error) {
	return "", nil, LdapErrorNotFound{}
}

type mockLdapConnectionFailure struct {
}

func (sLConn mockLdapConnectionFailure) findLdapUserWithRetries(guid string) (string, *v3.Principal, error) {
	return "", nil, LdapConnectionPermanentlyFailed{}
}

func TestWorkUnitConnectionFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		users              []v3.User
		ldapConn           retryableLdapConnection
		wantUsersToMigrate int
		wantMissingUsers   int
		wantSkippedUsers   int
	}{
		{
			name: "GUID-based users not found in Active Directory are counted as missing",
			users: []v3.User{
				{ObjectMeta: metav1.ObjectMeta{Name: testGuidLocalName}, PrincipalIDs: []string{testGuidPrincipal, testGuidLocalPrincipal}},
			},
			ldapConn:           mockLdapNeverSucceed{},
			wantUsersToMigrate: 0,
			wantMissingUsers:   1,
			wantSkippedUsers:   0,
		},
		{
			name: "GUID-based users are counted as skipped during LDAP connection failures",
			users: []v3.User{
				{ObjectMeta: metav1.ObjectMeta{Name: testGuidLocalName}, PrincipalIDs: []string{testGuidPrincipal, testGuidLocalPrincipal}},
			},
			ldapConn:           mockLdapConnectionFailure{},
			wantUsersToMigrate: 0,
			wantMissingUsers:   0,
			wantSkippedUsers:   1,
		},
		{
			name: "Dn-based users are not counted as missing, even if they don't exist in Active Directory",
			users: []v3.User{
				{ObjectMeta: metav1.ObjectMeta{Name: testDnLocalName}, PrincipalIDs: []string{testDnPrincipal, testDnLocalPrincipal}},
			},
			ldapConn:           mockLdapNeverSucceed{},
			wantUsersToMigrate: 0,
			wantMissingUsers:   0,
			wantSkippedUsers:   0,
		},
		{
			name: "Dn-based users are not counted as skipped, even during LDAP connection failures",
			users: []v3.User{
				{ObjectMeta: metav1.ObjectMeta{Name: testDnLocalName}, PrincipalIDs: []string{testDnPrincipal, testDnLocalPrincipal}},
			},
			ldapConn:           mockLdapConnectionFailure{},
			wantUsersToMigrate: 0,
			wantMissingUsers:   0,
			wantSkippedUsers:   0,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			userList := v3.UserList{Items: test.users}
			usersToMigrate, missingUsers, skippedUsers := identifyMigrationWorkUnits(&userList, test.ldapConn)
			assert.Equal(t, test.wantUsersToMigrate, len(usersToMigrate), "unexpected count for users to migrate")
			assert.Equal(t, test.wantMissingUsers, len(missingUsers), "unexpected count for missing users")
			assert.Equal(t, test.wantSkippedUsers, len(skippedUsers), "unexpected count for skipped users")
		})
	}
}

// stubPrepareClientContexts replaces prepareClientContextsFunc for the
// duration of the test. Tests using it must not be run in parallel.
func stubPrepareClientContexts(t *testing.T, f func(*restclient.Config) (*config.ScaledContext, *v3.ActiveDirectoryConfig, error)) {
	t.Helper()
	original := prepareClientContextsFunc
	t.Cleanup(func() { prepareClientContextsFunc = original })
	prepareClientContextsFunc = f
}

func TestUnmigrateAdGUIDUsersAuthConfigNotFound(t *testing.T) {
	hook := logrusTest.NewGlobal()
	stubPrepareClientContexts(t, func(*restclient.Config) (*config.ScaledContext, *v3.ActiveDirectoryConfig, error) {
		return nil, nil, apierrors.NewNotFound(schema.GroupResource{Group: "management.cattle.io", Resource: "authconfigs"}, activeDirectoryName)
	})

	err := UnmigrateAdGUIDUsers(&restclient.Config{}, true, true)

	require.NoError(t, err)
	require.NotNil(t, hook.LastEntry())
	assert.Equal(t, logrus.InfoLevel, hook.LastEntry().Level)
	assert.Contains(t, hook.LastEntry().Message, "activedirectory auth config not found, nothing to unmigrate")
}

func TestUnmigrateAdGUIDUsersPrepareClientContextsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	secrets := wranglerfake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
	secrets.EXPECT().Get("cattle-global-data", "activedirectoryconfig-serviceaccountpassword", gomock.Any()).
		Return(nil, apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, "activedirectoryconfig-serviceaccountpassword")).AnyTimes()
	authConfigs := &fakeAuthConfigClient{obj: &unstructured.Unstructured{Object: map[string]interface{}{
		"metadata":               map[string]interface{}{"name": activeDirectoryName},
		"enabled":                true,
		"serviceAccountPassword": "cattle-global-data:activedirectoryconfig-serviceaccountpassword",
	}}}

	tests := []struct {
		name    string
		prepare func(*restclient.Config) (*config.ScaledContext, *v3.ActiveDirectoryConfig, error)
		wantErr string
	}{
		{
			name: "generic error",
			prepare: func(*restclient.Config) (*config.ScaledContext, *v3.ActiveDirectoryConfig, error) {
				return nil, nil, errors.New("failed to create scaled context")
			},
			wantErr: "failed to create scaled context",
		},
		{
			// The error produced by loadADConfiguration when the service account
			// Secret is missing must not be swallowed as a missing AuthConfig.
			name: "missing service account secret",
			prepare: func(*restclient.Config) (*config.ScaledContext, *v3.ActiveDirectoryConfig, error) {
				adConfig, err := loadADConfiguration(authConfigs, secrets)
				return nil, adConfig, err
			},
			wantErr: "activedirectoryconfig-serviceaccountpassword",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubPrepareClientContexts(t, tt.prepare)

			err := UnmigrateAdGUIDUsers(&restclient.Config{}, false, false)

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
