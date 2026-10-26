package adunmigration

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIdentifyTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                     string
		workunit                 migrateUserWorkUnit
		tokens                   []v3.Token
		wantAdTokens             int
		wantDuplicateLocalTokens int
	}{
		{
			name:     "Guid-based token referencing Original GUID-based user will be migrated",
			workunit: guidOriginalWorkunit(),
			tokens: []v3.Token{
				{UserID: testGuidLocalName, UserPrincipal: v3.Principal{ObjectMeta: metav1.ObjectMeta{Name: testGuidPrincipal}}},
			},
			wantAdTokens:             1,
			wantDuplicateLocalTokens: 0,
		},
		{
			name:     "Local-based token referencing Original GUID-based user will not be migrated",
			workunit: guidOriginalWorkunit(),
			tokens: []v3.Token{
				{UserID: testGuidLocalName, UserPrincipal: v3.Principal{ObjectMeta: metav1.ObjectMeta{Name: testGuidLocalPrincipal}}},
			},
			wantAdTokens:             0,
			wantDuplicateLocalTokens: 0,
		},
		{
			name:     "Guid-based token referencing Duplicate GUID-based user will be migrated",
			workunit: guidOriginalGuidDuplicateWorkunit(),
			tokens: []v3.Token{
				{UserID: testDuplicateGuidLocalName, UserPrincipal: v3.Principal{ObjectMeta: metav1.ObjectMeta{Name: testGuidPrincipal}}},
			},
			wantAdTokens:             1,
			wantDuplicateLocalTokens: 0,
		},
		{
			name:     "Local-based token referencing Duplicate GUID-based user will be migrated",
			workunit: guidOriginalGuidDuplicateWorkunit(),
			tokens: []v3.Token{
				{UserID: testDuplicateGuidLocalName, UserPrincipal: v3.Principal{ObjectMeta: metav1.ObjectMeta{Name: testDuplicateGuidLocalPrincipal}}},
			},
			wantAdTokens:             0,
			wantDuplicateLocalTokens: 1,
		},
		{
			name:     "DN-based token referencing Original DN-based user will not be migrated",
			workunit: dnOriginalGuidDuplicateWorkunit(),
			tokens: []v3.Token{
				{UserID: testDnLocalName, UserPrincipal: v3.Principal{ObjectMeta: metav1.ObjectMeta{Name: testDnLocalPrincipal}}},
			},
			wantAdTokens:             0,
			wantDuplicateLocalTokens: 0,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			tokenList := v3.TokenList{Items: test.tokens}
			workunitList := []migrateUserWorkUnit{test.workunit}
			identifyTokens(&workunitList, &tokenList)

			assert.Equal(t, test.wantAdTokens, len(workunitList[0].activeDirectoryTokens), "expected AD-based tokens must match")
			assert.Equal(t, test.wantDuplicateLocalTokens, len(workunitList[0].duplicateLocalTokens), "expected duplicate Local-based tokens must match")
		})
	}
}

func TestUpdateToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		principalID      string
		newPrincipalID   string
		wantAuthProvider string
	}{
		{
			name:             "token migrated to a DN-based principal keeps its auth provider",
			principalID:      testGuidPrincipal,
			newPrincipalID:   activeDirectoryPrefix + testDn,
			wantAuthProvider: activeDirectoryName,
		},
		{
			name:             "token migrated to a local principal gets the local auth provider",
			principalID:      testDuplicateGuidLocalPrincipal,
			newPrincipalID:   localPrefix + testGuidLocalName,
			wantAuthProvider: localName,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			token := v3.Token{
				ObjectMeta:    metav1.ObjectMeta{Name: "token-abcde"},
				AuthProvider:  activeDirectoryName,
				UserID:        testDuplicateGuidLocalName,
				UserPrincipal: v3.Principal{ObjectMeta: metav1.ObjectMeta{Name: test.principalID}, Provider: activeDirectoryName},
			}
			var updated *v3.Token
			tokenInterface := &fakes.TokenInterfaceMock{
				GetFunc: func(_ string, _ metav1.GetOptions) (*v3.Token, error) {
					return token.DeepCopy(), nil
				},
				UpdateFunc: func(token *v3.Token) (*v3.Token, error) {
					updated = token
					return token, nil
				},
			}
			targetUser := &v3.User{ObjectMeta: metav1.ObjectMeta{Name: testGuidLocalName}}
			targetPrincipal := &v3.Principal{LoginName: "testuser", DisplayName: "Test User"}

			err := updateToken(tokenInterface, token, test.newPrincipalID, testGuid, targetUser, targetPrincipal)
			require.NoError(t, err)
			require.NotNil(t, updated)
			assert.Equal(t, test.newPrincipalID, updated.UserPrincipal.Name)
			assert.Equal(t, targetUser.Name, updated.UserID)
			// The deprecated AuthProvider is still written for external consumers.
			assert.Equal(t, test.wantAuthProvider, updated.AuthProvider)
			assert.Equal(t, test.wantAuthProvider, updated.UserPrincipal.Provider)
		})
	}
}
