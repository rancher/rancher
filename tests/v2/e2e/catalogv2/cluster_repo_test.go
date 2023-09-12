package catalogv2

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/k3d"
	"github.com/rancher/rancher/tests/framework/extensions/kubeapi/helm"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	rancherGithubURL = "https://github.com/rancher/rancher"
)

type Release struct {
	TagName string `json:"tag_name"`
}

type ClusterRepoGitTestSuite struct {
	suite.Suite
	session *session.Session
}

func (c *ClusterRepoGitTestSuite) SetupSuite() {
	testSession := session.NewSession()
	c.session = testSession
}

func (c *ClusterRepoGitTestSuite) TestRancherAirGapUpgrade() {
	// Install a K3D cluster
	restConfig, err := k3d.CreateK3DCluster(c.session, "test", "", 1, 0)
	require.NoError(c.T(), err)
	require.NotNil(c.T(), restConfig)

	// Install Rancher Latest Version
	err = helm.InstallRancher(c.session, restConfig)
	require.NoError(c.T(), err)
}

func (c *ClusterRepoGitTestSuite) TearDownSuite() {
	c.session.Cleanup()
}

func TestClusterRepoTestSuite(t *testing.T) {
	suite.Run(t, new(ClusterRepoGitTestSuite))
}

/*package kube_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/rancher/tests/framework/clients/helm"
	"github.com/rancher/rancher/tests/framework/clients/k3d"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"k8s.io/client-go/rest"
)

var _ = Describe("ClusterRepo", func() {
	testSession := session.NewSession()

	var (
		err        error
		restConfig *rest.Config
	)

	BeforeEach(func() {
		// Create a K3D cluster
		restConfig, err = k3d.CreateK3DCluster(testSession, "e2e-kube", "", 1, 0)
		Expect(err).To(BeNil())
	})

	It("should install rancher and upgrade rancher in airgap and then git directories should get updated after", func() {
		//  ClientSet of kubernetes
		clientset, err := kubernetes.NewForConfig(restConfig)
		Expect(err).To(BeNil())

		fmt.Printf("RestConfig %v", restConfig)

		// Create namespace cattle-system
		namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "cattle-system"}}
		_, err = clientset.CoreV1().Namespaces().Create(context.Background(), namespace, metav1.CreateOptions{})
		Expect(err).To(BeNil())

		err = helm.InstallRancher(restConfig)
		Expect(err).To(BeNil())

		// Get Rancher pod Name
		output, err := kubectl.RunOutput("-n",
			"cattle-system",
			"get",
			"pod",
			"-l",
			"app=rancher",
			"-o",
			"jsonpath={.items..metadata.name}")
		Expect(err).To(BeNil())

		// Exec into Pod and check for commits in Git directory and compare it with ClusterRepo status commit
		output, err = kubectl.RunOutput("-n",
			"cattle-system",
			"exec",
			string(output),
			"--",
			"sh",
			"-c",
			"cd /var/lib/rancher-data/local-catalogs/v2/rancher-charts/4b40cac650031b74776e87c1a726b0484d0877c3ec137da0872547ff9b73a721 && git rev-parse HEAD")
		Expect(err).To(BeNil())
		commit := string(output)
		commit = strings.TrimSuffix(commit, "\n")

		// Get clusterrepo and check if it matches with it ?
		output, err = kubectl.RunOutput("get",
			"clusterrepo",
			"rancher-charts",
			"-o",
			"jsonpath={.status.commit}")
		crCommit := string(output)
		Expect(err).To(BeNil())
		Expect(commit).Should(Equal(crCommit))

		tag := os.Getenv("TAG")
		fmt.Println(tag)

		// Upgrade Rancher to the current one [I need docker image, helm chart in Github Actions]
		err = helm.HelmUpgradeRancher(tag)
		Expect(err).To(BeNil())

		// Use that helm chart to upgrade to Rancher
		// Exec into Pod and check for commits in Git directory and compare it with ClusterRepo status commit
	})

	AfterEach(func() {
		err := k3d.DeleteK3DCluster("e2e-kube")
		Expect(err).To(BeNil())
	})
})
*/
