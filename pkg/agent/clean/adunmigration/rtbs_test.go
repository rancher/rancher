package adunmigration

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testGuid                        = "953d82a03d47a5498330293e386dfce1"
	testDn                          = "CN=testuser1,CN=Users,DC=qa,DC=rancher,DC=space"
	testDnLocalName                 = "user-vhhxd"
	testGuidLocalName               = "u-fydcaomakf"
	testDuplicateGuidLocalName      = "u-quuknbiweg"
	testGuidPrincipal               = "activedirectory_user://" + testGuid
	testDnPrincipal                 = "activedirectory_user://" + testDn
	testDnLocalPrincipal            = "local://" + testDnLocalName
	testGuidLocalPrincipal          = "local://" + testGuidLocalName
	testDuplicateGuidLocalPrincipal = "local://" + testDuplicateGuidLocalName
)

func guidOriginalWorkunit() migrateUserWorkUnit {
	// The "success" case for the original migration logic: only the GUID is left, with no extra copies
	return migrateUserWorkUnit{
		guid:              testGuid,
		distinguishedName: testDn,
		originalUser: &v3.User{
			ObjectMeta:   metav1.ObjectMeta{Name: testGuidLocalName},
			PrincipalIDs: []string{testGuidLocalPrincipal, testGuidPrincipal}},
	}
}

func guidOriginalGuidDuplicateWorkunit() migrateUserWorkUnit {
	// An uncommon case caused by a race condition: the older GUID-based user was migrated, but not
	// before a newer GUID-based duplicate was created. After this, the affected user can no longer log in
	// due to both users having the same principal ID
	return migrateUserWorkUnit{
		guid:              testGuid,
		distinguishedName: testDn,
		originalUser: &v3.User{
			ObjectMeta:   metav1.ObjectMeta{Name: testGuidLocalName},
			PrincipalIDs: []string{testGuidLocalPrincipal, testGuidPrincipal},
		},
		duplicateUsers: []*v3.User{{
			ObjectMeta:   metav1.ObjectMeta{Name: testDuplicateGuidLocalName},
			PrincipalIDs: []string{testDuplicateGuidLocalPrincipal, testGuidPrincipal}}},
	}
}

func dnOriginalGuidDuplicateWorkunit() migrateUserWorkUnit {
	// Caused by a failure to migrate the original user. A new GUID-based user is
	// then created at the next login with none of the original permissions.
	return migrateUserWorkUnit{
		guid:              testGuid,
		distinguishedName: testDn,
		originalUser: &v3.User{
			ObjectMeta:   metav1.ObjectMeta{Name: testDnLocalName},
			PrincipalIDs: []string{testDnLocalPrincipal, testDnPrincipal}},
		duplicateUsers: []*v3.User{{
			ObjectMeta:   metav1.ObjectMeta{Name: testGuidLocalName},
			PrincipalIDs: []string{testGuidLocalPrincipal, testGuidPrincipal}}},
	}
}

func TestIdentifyCRTBs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                    string
		workunit                migrateUserWorkUnit
		crtbs                   []v3.ClusterRoleTemplateBinding
		wantAdCrtbs             int
		wantDuplicateLocalCrtbs int
	}{
		{
			name:     "Guid-based CRTB referencing Original GUID-based user will be migrated",
			workunit: guidOriginalWorkunit(),
			crtbs: []v3.ClusterRoleTemplateBinding{
				{UserName: testGuidLocalName, UserPrincipalName: testGuidPrincipal},
			},
			wantAdCrtbs:             1,
			wantDuplicateLocalCrtbs: 0,
		},
		{
			name:     "Local-based CRTB referencing Original GUID-based user will not be migrated",
			workunit: guidOriginalWorkunit(),
			crtbs: []v3.ClusterRoleTemplateBinding{
				{UserName: testGuidLocalName, UserPrincipalName: testGuidLocalPrincipal},
			},
			wantAdCrtbs:             0,
			wantDuplicateLocalCrtbs: 0,
		},
		{
			name:     "Guid-based CRTB referencing Duplicate GUID-based user will be migrated",
			workunit: guidOriginalGuidDuplicateWorkunit(),
			crtbs: []v3.ClusterRoleTemplateBinding{
				{UserName: testDuplicateGuidLocalName, UserPrincipalName: testGuidPrincipal},
			},
			wantAdCrtbs:             1,
			wantDuplicateLocalCrtbs: 0,
		},
		{
			name:     "Local-based CRTB referencing Duplicate GUID-based user will be migrated",
			workunit: guidOriginalGuidDuplicateWorkunit(),
			crtbs: []v3.ClusterRoleTemplateBinding{
				{UserName: testDuplicateGuidLocalName, UserPrincipalName: testDuplicateGuidLocalPrincipal},
			},
			wantAdCrtbs:             0,
			wantDuplicateLocalCrtbs: 1,
		},
		{
			name:     "DN-based CRTB referencing Original DN-based user will not be migrated",
			workunit: dnOriginalGuidDuplicateWorkunit(),
			crtbs: []v3.ClusterRoleTemplateBinding{
				{UserName: testDnLocalName, UserPrincipalName: testDnLocalPrincipal},
			},
			wantAdCrtbs:             0,
			wantDuplicateLocalCrtbs: 0,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			crtbList := v3.ClusterRoleTemplateBindingList{Items: test.crtbs}
			workunitList := []migrateUserWorkUnit{test.workunit}
			identifyCRTBs(&workunitList, &crtbList)

			assert.Equal(t, test.wantAdCrtbs, len(workunitList[0].activeDirectoryCRTBs), "expected AD-based CRTBs must match")
			assert.Equal(t, test.wantDuplicateLocalCrtbs, len(workunitList[0].duplicateLocalCRTBs), "expected duplicate Local-based CRTBs must match")
		})
	}
}

func TestIdentifyPRTBs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                    string
		workunit                migrateUserWorkUnit
		prtbs                   []v3.ProjectRoleTemplateBinding
		wantAdPrtbs             int
		wantDuplicateLocalPrtbs int
	}{
		{
			name:     "Guid-based PRTB referencing Original GUID-based user will be migrated",
			workunit: guidOriginalWorkunit(),
			prtbs: []v3.ProjectRoleTemplateBinding{
				{UserName: testGuidLocalName, UserPrincipalName: testGuidPrincipal},
			},
			wantAdPrtbs:             1,
			wantDuplicateLocalPrtbs: 0,
		},
		{
			name:     "Local-based PRTB referencing Original GUID-based user will not be migrated",
			workunit: guidOriginalWorkunit(),
			prtbs: []v3.ProjectRoleTemplateBinding{
				{UserName: testGuidLocalName, UserPrincipalName: testGuidLocalPrincipal},
			},
			wantAdPrtbs:             0,
			wantDuplicateLocalPrtbs: 0,
		},
		{
			name:     "Guid-based PRTB referencing Duplicate GUID-based user will be migrated",
			workunit: guidOriginalGuidDuplicateWorkunit(),
			prtbs: []v3.ProjectRoleTemplateBinding{
				{UserName: testDuplicateGuidLocalName, UserPrincipalName: testGuidPrincipal},
			},
			wantAdPrtbs:             1,
			wantDuplicateLocalPrtbs: 0,
		},
		{
			name:     "Local-based PRTB referencing Duplicate GUID-based user will be migrated",
			workunit: guidOriginalGuidDuplicateWorkunit(),
			prtbs: []v3.ProjectRoleTemplateBinding{
				{UserName: testDuplicateGuidLocalName, UserPrincipalName: testDuplicateGuidLocalPrincipal},
			},
			wantAdPrtbs:             0,
			wantDuplicateLocalPrtbs: 1,
		},
		{
			name:     "DN-based PRTB referencing Original DN-based user will not be migrated",
			workunit: dnOriginalGuidDuplicateWorkunit(),
			prtbs: []v3.ProjectRoleTemplateBinding{
				{UserName: testDnLocalName, UserPrincipalName: testDnLocalPrincipal},
			},
			wantAdPrtbs:             0,
			wantDuplicateLocalPrtbs: 0,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			prtbList := v3.ProjectRoleTemplateBindingList{Items: test.prtbs}
			workunitList := []migrateUserWorkUnit{test.workunit}
			identifyPRTBs(&workunitList, &prtbList)

			assert.Equal(t, test.wantAdPrtbs, len(workunitList[0].activeDirectoryPRTBs), "expected AD-based PRTBs must match")
			assert.Equal(t, test.wantDuplicateLocalPrtbs, len(workunitList[0].duplicateLocalPRTBs), "expected duplicate Local-based PRTBs must match")
		})
	}
}

func TestIdentifyGRBs(t *testing.T) {
	//t.Parallel()

	tests := []struct {
		name                   string
		workunit               migrateUserWorkUnit
		grbs                   []v3.GlobalRoleBinding
		wantDuplicateLocalGrbs int
	}{
		{
			name:     "GRB referencing Original user will not be migrated",
			workunit: guidOriginalGuidDuplicateWorkunit(),
			grbs: []v3.GlobalRoleBinding{
				{UserName: testGuidLocalName},
			},
			wantDuplicateLocalGrbs: 0,
		},
		{
			name:     "GRB referencing Duplicate user will be migrated",
			workunit: guidOriginalGuidDuplicateWorkunit(),
			grbs: []v3.GlobalRoleBinding{
				{UserName: testDuplicateGuidLocalName},
			},
			wantDuplicateLocalGrbs: 1,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			grbList := v3.GlobalRoleBindingList{Items: test.grbs}
			workunitList := []migrateUserWorkUnit{test.workunit}
			identifyGRBs(&workunitList, &grbList)

			assert.Equal(t, test.wantDuplicateLocalGrbs, len(workunitList[0].duplicateLocalGRBs), "expected duplicate GRBs must match")
		})
	}
}
