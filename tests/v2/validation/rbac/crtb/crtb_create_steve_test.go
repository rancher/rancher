//go:build (validation || infra.any || cluster.any || sanity) && !stress && !extended

package crtb

import (
	"testing"

	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type CRTBGenTestSuite struct {
	suite.Suite
	client    *rancher.Client
	session   *session.Session
	clusterID string
	newUser   *management.User
}

func (crtb *CRTBGenTestSuite) TearDownSuite() {
	crtb.session.Cleanup()
}

func (crtb *CRTBGenTestSuite) SetupSuite() {
	testSession := session.NewSession()
	crtb.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(crtb.T(), err)

	crtb.client = client

	crtb.clusterID, err = clusters.GetClusterIDByName(crtb.client, client.RancherConfig.ClusterName)
	require.NoError(crtb.T(), err, "Error getting cluster ID")

	crtb.newUser, err = users.CreateUserWithRole(client, users.UserConfig())
	require.NoError(crtb.T(), err)
}

func (crtb *CRTBGenTestSuite) TestCreateCrtbWithAllRequiredInputValues() {
	client := crtb.client

	tests := []struct {
		roleTemplateName string
	}{
		{clusterOwner},
		{clusterMember},
		{crtbManage},
	}
	for _, rtn := range tests {
		crtbName := namegen.RandStringLower(crtbNameWithTenChar)
		clusterRoleTemplateBinding := clusterRoleTemplateBindingTemplate(crtb.clusterID, crtbName, crtb.newUser.ID, rtn.roleTemplateName)

		resp, err := client.Steve.SteveType(crtbAPIEndPoint).Create(clusterRoleTemplateBinding)
		require.NoError(crtb.T(), err)
		assert.Contains(crtb.T(), (*resp).JSONResp["id"], crtbName)
		assert.Contains(crtb.T(), (*resp).JSONResp["clusterName"], crtb.clusterID)
		assert.Contains(crtb.T(), (*resp).JSONResp["userName"], crtb.newUser.ID)
		assert.Contains(crtb.T(), (*resp).JSONResp["roleTemplateName"], rtn.roleTemplateName)
	}
}

func (crtb *CRTBGenTestSuite) TestCreateCrtbWithUserPrincipalNameAsInput() {
	client := crtb.client
	crtbName := namegen.RandStringLower(crtbNameWithTenChar)

	clusterRoleTemplateBinding := clusterRoleTemplateBindingTemplate(crtb.clusterID, crtbName, crtb.newUser.ID, clusterMember)
	clusterRoleTemplateBinding.UserPrincipalName = "local://" + crtb.newUser.ID

	resp, err := client.Steve.SteveType(crtbAPIEndPoint).Create(clusterRoleTemplateBinding)
	require.NoError(crtb.T(), err)

	assert.Contains(crtb.T(), (*resp).JSONResp["id"], crtbName)
	assert.Contains(crtb.T(), (*resp).JSONResp["clusterName"], crtb.clusterID)
	assert.Contains(crtb.T(), (*resp).JSONResp["userName"], crtb.newUser.ID)
	assert.Contains(crtb.T(), (*resp).JSONResp["roleTemplateName"], clusterMember)
	assert.Equal(crtb.T(), (*resp).JSONResp["userPrincipalName"], clusterRoleTemplateBinding.UserPrincipalName)
}

func (crtb *CRTBGenTestSuite) TestCreateCrtbAndInputLabels() {
	client := crtb.client
	crtbName := namegen.RandStringLower(crtbNameWithTenChar)

	clusterRoleTemplateBinding := clusterRoleTemplateBindingTemplate(crtb.clusterID, crtbName, crtb.newUser.ID, clusterMember)
	clusterRoleTemplateBinding.ObjectMeta.Labels = map[string]string{"Hello": "World"}

	resp, err := client.Steve.SteveType(crtbAPIEndPoint).Create(clusterRoleTemplateBinding)
	require.NoError(crtb.T(), err)

	assert.Contains(crtb.T(), (*resp).JSONResp["id"], crtbName)
	assert.Contains(crtb.T(), (*resp).JSONResp["clusterName"], crtb.clusterID)
	assert.Contains(crtb.T(), (*resp).JSONResp["userName"], crtb.newUser.ID)
	assert.Contains(crtb.T(), (*resp).JSONResp["roleTemplateName"], clusterMember)
	assert.NotContains(crtb.T(), (*resp).JSONResp, clusterRoleTemplateBinding.Labels)
}

func (crtb *CRTBGenTestSuite) TestValidateErrorAlreadyCrtbExisted() {
	client := crtb.client
	crtbName := namegen.RandStringLower(crtbNameWithTenChar)

	clusterRoleTemplateBinding := clusterRoleTemplateBindingTemplate(crtb.clusterID, crtbName, crtb.newUser.ID, clusterMember)

	resp, err := client.Steve.SteveType(crtbAPIEndPoint).Create(clusterRoleTemplateBinding)
	require.NoError(crtb.T(), err)
	assert.Contains(crtb.T(), (*resp).JSONResp["id"], crtbName)

	_, err = client.Steve.SteveType(crtbAPIEndPoint).Create(clusterRoleTemplateBinding)
	require.Error(crtb.T(), err)
	require.ErrorContains(crtb.T(), err, crtbConflictError)
	require.ErrorContains(crtb.T(), err, crtbAlreadyExisted)
}

func (crtb *CRTBGenTestSuite) TestValidateErrorRoleTemplateNameDoesNotExist() {
	client := crtb.client
	crtbName := namegen.RandStringLower(crtbNameWithTenChar)

	clusterRoleTemplateBinding := clusterRoleTemplateBindingTemplate(crtb.clusterID, crtbName, crtb.newUser.ID, clusterMember)
	clusterRoleTemplateBinding.RoleTemplateName = "roleTemplateNameNotExisted"

	_, err := client.Steve.SteveType(crtbAPIEndPoint).Create(clusterRoleTemplateBinding)
	require.Error(crtb.T(), err)
	require.ErrorContains(crtb.T(), err, badRequest)
}

func (crtb *CRTBGenTestSuite) TestValidateErrorBadRequestByPassingDifferentClusterAndNamespace() {
	client := crtb.client
	crtbName := namegen.RandStringLower(crtbNameWithTenChar)

	clusterRoleTemplateBinding := clusterRoleTemplateBindingTemplate(crtb.clusterID, crtbName, crtb.newUser.ID, clusterMember)
	clusterRoleTemplateBinding.ClusterName = localCluster

	_, err := client.Steve.SteveType(crtbAPIEndPoint).Create(clusterRoleTemplateBinding)
	require.Error(crtb.T(), err)
	require.ErrorContains(crtb.T(), err, badRequest)
}

func (crtb *CRTBGenTestSuite) TestValidateErrorNamespaceNameDoesNotExist() {
	client := crtb.client
	crtbName := namegen.RandStringLower(crtbNameWithTenChar)

	clusterRoleTemplateBinding := clusterRoleTemplateBindingTemplate(crtb.clusterID, crtbName, crtb.newUser.ID, clusterMember)
	clusterRoleTemplateBinding.ClusterName = "namespacenotexisted"

	_, err := client.Steve.SteveType(crtbAPIEndPoint).Create(clusterRoleTemplateBinding)
	require.Error(crtb.T(), err)
	require.ErrorContains(crtb.T(), err, badRequest)
}

func (crtb *CRTBGenTestSuite) TestValidateErrorClusterNameAndNamespaceNameDifferent() {
	client := crtb.client
	crtbName := namegen.RandStringLower(crtbNameWithTenChar)

	clusterRoleTemplateBinding := clusterRoleTemplateBindingTemplate(crtb.clusterID, crtbName, crtb.newUser.ID, clusterMember)
	clusterRoleTemplateBinding.ClusterName = localCluster

	_, err := client.Steve.SteveType(crtbAPIEndPoint).Create(clusterRoleTemplateBinding)
	require.Error(crtb.T(), err)
	require.ErrorContains(crtb.T(), err, cluserNameAndNamespaceSameValue)
}

func TestCRTBGenTestSuite(t *testing.T) {
	suite.Run(t, new(CRTBGenTestSuite))
}
