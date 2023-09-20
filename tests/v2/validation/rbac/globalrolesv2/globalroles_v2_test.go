package globalrolesv2

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/pkg/session"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/rancher/rancher/tests/framework/extensions/kubeapi/rbac"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

type GlobalRolesV2TestSuite struct {
	suite.Suite
	client      *rancher.Client
	session     *session.Session
	cluster     *management.Cluster
	clusterName string
	clusterID   string
}

func (gr *GlobalRolesV2TestSuite) TearDownSuite() {
	gr.session.Cleanup()
}

func (gr *GlobalRolesV2TestSuite) SetupSuite() {
	testSession := session.NewSession()
	gr.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(gr.T(), err)

	gr.client = client
	gr.clusterName = client.RancherConfig.ClusterName
	require.NotEmptyf(gr.T(), gr.clusterName, "Cluster name to install should be set")
	gr.clusterID, err = clusters.GetClusterIDByName(gr.client, gr.clusterName)
	require.NoError(gr.T(), err, "Error getting cluster ID")
	gr.cluster, err = gr.client.Management.Cluster.ByID(gr.clusterID)
	require.NoError(gr.T(), err)

}

func (gr *GlobalRolesV2TestSuite) TestGlobalRolesV2() {
	subSession := gr.session.NewSession()
	defer subSession.Cleanup()

	globalRole.Name = namegen.AppendRandomString("testgr-")
	globalRole.InheritedClusterRoles = append(globalRole.InheritedClusterRoles, roleOwner)
	createdRole, err := rbac.CreateGlobalRole(gr.client, localcluster, &globalRole)
	require.NoError(gr.T(), err)
	createUser, err := users.CreateUserWithRole(gr.client, users.UserConfig(), standardUser, createdRole.Name)

	require.NoError(gr.T(), err)

	log.Info("Verify the global role binding is created for the user")

	grblist, err := rbac.ListGlobalRoleBindings(gr.client, localcluster, metav1.ListOptions{})
	require.NoError(gr.T(), err, "Failed to create global role")

	var grbOwner string
	for _, grbs := range grblist.Items {
		if grbs.GlobalRoleName == globalRole.Name && grbs.UserName == createUser.ID {
			grbOwner = grbs.Name
		}
	}

	log.Info("Verify the cluster role template binding is created for the user")

	req, err := labels.NewRequirement(crtbOwnerLabel, selection.In, []string{grbOwner})
	require.NoError(gr.T(), err)

	selector := labels.NewSelector().Add(*req)
	downstreamCRTBs, err := rbac.ListClusterRoleTemplateBindings(gr.client, localcluster, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	require.NoError(gr.T(), err)

	assert.Equal(gr.T(), 1, len(downstreamCRTBs.Items))

	log.Info("Verify the role bindings are created for the user")

	


}

func TestGlobalRolesV2TestSuite(t *testing.T) {
	suite.Run(t, new(GlobalRolesV2TestSuite))
}
