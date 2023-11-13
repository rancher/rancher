//go:build (validation || infra.any || cluster.any || sanity) && !stress && !extended

package globalroles

import (
	"context"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"testing"

	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/pkg/session"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	fleetLocal       = "fleet-local"
	localCluster     = "local"
	applyOption      = "apply"
	deleteOption     = "delete"
	expectCreated    = "created"
	expectConfigured = "configured"
	expectDeleted    = "deleted"
	expectDenied     = "denied"
	adminRole        = "admin"
)

type GlobalRolesTestSuite struct {
	suite.Suite
	client    *rancher.Client
	session   *session.Session
	cluster   *management.Cluster
	clusterV1 *v1.Cluster
}

func (gr *GlobalRolesTestSuite) TearDownSuite() {
	gr.session.Cleanup()
}

func (gr *GlobalRolesTestSuite) SetupSuite() {
	testSession := session.NewSession()
	gr.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(gr.T(), err)

	gr.client = client

	require.NoError(gr.T(), err)

	kubeClient, err := gr.client.GetKubeAPIProvisioningClient()
	require.NoError(gr.T(), err)

	gr.clusterV1, err = kubeClient.Clusters(fleetLocal).Get(context.TODO(), localCluster, metav1.GetOptions{})
	require.NoError(gr.T(), err)
}

func (gr *GlobalRolesTestSuite) crudGlobalRole() {

	roles := []string{"cluster-admin"}
	validGR := setupGlobalRole(namegen.AppendRandomString("valid"), false, roles)
	invalidGR := setupGlobalRole(namegen.AppendRandomString("invalid"), true, roles)

	gr.Run("Create GlobalRole", func() {
		kubectlApplyOrDelete(gr.T(), gr.client, gr.clusterV1, validGR, localCluster, applyOption, expectCreated)
	})

	gr.Run("Edit GlobalRole", func() {
		validGR.DisplayName = "changed"
		kubectlApplyOrDelete(gr.T(), gr.client, gr.clusterV1, validGR, localCluster, applyOption, expectConfigured)
	})

	gr.Run("Invalid Edit GlobalRole Builtin", func() {
		validGR.Builtin = true
		kubectlApplyOrDelete(gr.T(), gr.client, gr.clusterV1, validGR, localCluster, applyOption, expectDenied)
	})

	gr.Run("Delete GlobalRole By File", func() {
		kubectlApplyOrDelete(gr.T(), gr.client, gr.clusterV1, validGR, localCluster, deleteOption, expectDeleted)
	})

	gr.Run("Invalid Create GlobalRole", func() {
		kubectlApplyOrDelete(gr.T(), gr.client, gr.clusterV1, invalidGR, localCluster, applyOption, expectDenied)
	})

	gr.Run("Delete Builtin GlobalRole", func() {
		deleteGlobalRole(gr.T(), gr.client, gr.clusterV1, localCluster, adminRole, expectDenied)
	})

	gr.Run("Read GlobalRole", func() {
		readGlobalRole(gr.T(), gr.client, gr.clusterV1, localCluster, adminRole, adminRole)
	})
}

func (gr *GlobalRolesTestSuite) TestGlobalRoles() {
	subSession := gr.session.NewSession()
	defer subSession.Cleanup()
	gr.crudGlobalRole()
}

func TestGlobalRolesTestSuite(t *testing.T) {
	suite.Run(t, new(GlobalRolesTestSuite))
}
