package catalogv2

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/go-version"
	catalogv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io"
	"github.com/rancher/rancher/tests/framework/clients/k3d"
	"github.com/rancher/rancher/tests/framework/extensions/kubeapi/helm"
	"github.com/rancher/rancher/tests/framework/extensions/kubeconfig"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rke/types/kdm"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

var (
	PollInterval = time.Duration(500 * time.Millisecond)
	PollTimeout  = time.Duration(5 * time.Minute)
)

const (
	rancherGithubURL = "https://github.com/rancher/rancher"
)

type Release struct {
	TagName string `json:"tag_name"`
}

type ClusterRepoGitTestSuite struct {
	suite.Suite
	session   *session.Session
	clientSet *kubernetes.Clientset
}

func (c *ClusterRepoGitTestSuite) SetupSuite() {
	testSession := session.NewSession()
	c.session = testSession
}

func (c *ClusterRepoGitTestSuite) TestRancherUpgrade() {
	// Get latest version of Rancher
	rancherVersionNumber := "v2.7.6"

	// Get latest K3s version that Rancher supports
	KDMDataURL := fmt.Sprintf("%s/releases/download/%s/rancher-data.json", rancherGithubURL, rancherVersionNumber)
	retryClient := retryablehttp.NewClient()
	retryClient.Logger = nil

	req, err := retryablehttp.NewRequest("GET", KDMDataURL, nil)
	require.NoError(c.T(), err)

	resp, err := retryClient.Do(req)
	require.NoError(c.T(), err)
	defer resp.Body.Close()
	require.Equal(c.T(), resp.StatusCode, 200)

	b, err := io.ReadAll(resp.Body)
	require.NoError(c.T(), err)

	data, err := kdm.FromData(b)
	require.NoError(c.T(), err)

	InstallableK3SVersion := ""

	for _, k3svalue := range data.K3S["releases"].([]interface{}) {

		k3sversionNumber := k3svalue.(map[string]interface{})["version"].(string)
		minRancherVersionNumber := k3svalue.(map[string]interface{})["minChannelServerVersion"].(string)
		maxRancherVersionNumber := k3svalue.(map[string]interface{})["maxChannelServerVersion"].(string)
		minRancherVersion, err := version.NewVersion(minRancherVersionNumber)
		require.NoError(c.T(), err)

		maxRancherVersion, err := version.NewVersion(maxRancherVersionNumber)
		require.NoError(c.T(), err)

		rancherVersion, err := version.NewVersion(rancherVersionNumber)
		require.NoError(c.T(), err)

		if minRancherVersion.LessThan(rancherVersion) && maxRancherVersion.GreaterThan(rancherVersion) {
			InstallableK3SVersion = k3sversionNumber
			InstallableK3SVersion = strings.Replace(InstallableK3SVersion, "+", "-", 1)
		}
	}

	// Install a K3D cluster
	restConfig, err := k3d.CreateK3DCluster(c.session, "test", "", 1, 0, InstallableK3SVersion)
	require.NoError(c.T(), err)
	require.NotNil(c.T(), restConfig)

	// Setup ClientSet
	c.clientSet, err = kubernetes.NewForConfig(restConfig)
	require.NoError(c.T(), err)

	// Install Rancher Latest Version
	err = helm.InstallRancher(c.session, restConfig)
	require.NoError(c.T(), err)

	// Get Rancher Pod Names
	labelSelector := v1.LabelSelector{MatchLabels: map[string]string{"app": "rancher"}}
	listOptions := v1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
		Limit:         100,
	}
	pods, err := c.clientSet.CoreV1().Pods("cattle-system").List(context.Background(), listOptions)
	require.NoError(c.T(), err)

	// Check if the ClusterRepo commit is same as the git directory inside all Rancher pods
	catalogClient, err := catalog.NewFactoryFromConfig(restConfig)
	require.NoError(c.T(), err)

	var clusterRepo *catalogv1.ClusterRepo

	err = wait.Poll(PollInterval, PollTimeout, func() (done bool, err error) {
		clusterRepo, err = catalogClient.Catalog().V1().ClusterRepo().Get("rancher-charts", v1.GetOptions{})
		require.NoError(c.T(), err)

		return clusterRepo.Status.Commit != "", nil
	})
	require.NoError(c.T(), err)

	c.T().Log(clusterRepo.Status.Commit, "======")
	cmd := []string{
		"/bin/sh",
		"-c",
		"cd /var/lib/rancher-data/local-catalogs/v2/rancher-charts/4b40cac650031b74776e87c1a726b0484d0877c3ec137da0872547ff9b73a721 && git rev-parse HEAD",
	}
	for _, pod := range pods.Items {
		logStreamer, err := kubeconfig.KubectlExec(restConfig, pod.GetName(), "cattle-system", cmd)
		require.NoError(c.T(), err)
		c.T().Log(logStreamer.String(), "======")
		require.Equal(c.T(), logStreamer.String(), clusterRepo.Status.Commit)

	}
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
