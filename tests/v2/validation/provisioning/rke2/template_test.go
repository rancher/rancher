package rke2

import (
	"testing"
	"time"

	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/components/components"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	password "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	repoType = "catalog.cattle.io.clusterrepo"
)

type ClusterTemplateTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	repoSpec           *v1.RepoSpec
}

func (r *ClusterTemplateTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *ClusterTemplateTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession
	r.repoSpec = new(v1.RepoSpec)
	config.LoadConfig("repospec", r.repoSpec)

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)
	r.client = client

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
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-repo",
		},
		Spec: *r.repoSpec,
	}
	//created, err := components.GenericCreate(r.client, repo, repoType)
	var interfaceSlice []interface{} = make([]interface{}, 1)
	interfaceSlice[0] = repo
	mycomponent := components.GenericCreate{
		ObjSpecs: interfaceSlice,
		ObjType:  repoType,
		Client:   r.client,
	}
	err := mycomponent.Apply(true, 500*time.Millisecond, 30*time.Minute)
	if err != nil {
		log.Info(err)
	}
	time.Sleep(10 * time.Second)
	err = mycomponent.Revert(false, 500*time.Millisecond, 30*time.Minute)
	if err != nil {
		log.Info(err)
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestClusterTemplatesTestSuite(t *testing.T) {
	suite.Run(t, new(ClusterTemplateTestSuite))
}
