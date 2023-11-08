//go:build (validation || extended) && !infra.any && !infra.aks && !infra.eks && !infra.gke && !infra.rke2k3s && !cluster.any && !cluster.custom && !cluster.nodedriver && !sanity && !stress

package rke2

import (
	"testing"

	components "github.com/components/tests/framework/extensions/components"
	"github.com/rancher/machine/libmachine/log"
	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/clusters/kubernetesversions"
	"github.com/rancher/rancher/tests/framework/extensions/provisioninginput"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	password "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ClusterTemplateTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	repoConfig         *RepoSpec
}

func (r *ClusterTemplateTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *ClusterTemplateTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession
	r.provisioningConfig = new(provisioninginput.Config)
	config.LoadConfig("repo", r.repoConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)
	r.client = client

	r.provisioningConfig.RKE2KubernetesVersions, err = kubernetesversions.Default(
		r.client, clusters.RKE2ClusterType.String(), r.provisioningConfig.RKE2KubernetesVersions)
	require.NoError(r.T(), err)

	enabled := true
	var testuser = namegen.AppendRandomString("testuser-")
	var testpassword = password.GenerateUserPassword("testpass-")
	user := &management.User{
		Username: testuser,
		Password: testpassword,
		Name:     testuser,
		Enabled:  &enabled,
	}

	newUser, err := users.CreateUserWithRole(client, user, "user")
	require.NoError(r.T(), err)

	newUser.Password = user.Password

	standardUserClient, err := client.AsUser(newUser)
	require.NoError(r.T(), err)

	r.standardUserClient = standardUserClient
}

func (r *ClusterTemplateTestSuite) TestProvisionClusterTemplate() {
	repo := v1.ClusterRepo{
		Spec: v1.RepoSpec{
			GitRepo:               "https://github.com/susesgartner/cluster-template-examples.git",
			GitBranch:             "main",
			InsecureSkipTLSverify: true,
		},
	}
	created, err := components.GenericCreate(&rancher.Client{}, repo, repotype)
	if err != nil {
		log.Info(err)
	}
	log.Info(created)
	log.Info("Entering main")
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestClusterTemplatesTestSuite(t *testing.T) {
	suite.Run(t, new(ClusterTemplateTestSuite))
}
