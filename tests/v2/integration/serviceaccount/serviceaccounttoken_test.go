package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/serviceaccounttoken"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type ServiceAccountSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (s *ServiceAccountSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *ServiceAccountSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	client, err := rancher.NewClient("", testSession)
	s.Require().NoError(err)
	s.client = client
}

func (s *ServiceAccountSuite) TestSingleSecretForServiceAccount() {
	localCluster, err := s.client.Management.Cluster.ByID("local")
	s.Require().NoError(err)
	s.Require().NotEmpty(localCluster)
	localClusterKubeconfig, err := s.client.Management.Cluster.ActionGenerateKubeconfig(localCluster)
	s.Require().NoError(err)
	c, err := clientcmd.NewClientConfigFromBytes([]byte(localClusterKubeconfig.Config))
	s.Require().NoError(err)
	cc, err := c.ClientConfig()
	s.Require().NoError(err)
	clientset, err := kubernetes.NewForConfig(cc)
	s.Require().NoError(err)

	testNS := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ns",
		},
	}
	testNS, err = clientset.CoreV1().Namespaces().Create(context.Background(), testNS, metav1.CreateOptions{})
	s.Require().NoError(err)

	serviceAccount := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: testNS.Name,
		},
	}
	serviceAccount, err = clientset.CoreV1().ServiceAccounts(testNS.Name).Create(context.Background(), serviceAccount, metav1.CreateOptions{})
	s.Require().NoError(err)

	// mimic a scenario where multiple func calls for the same SA, and check the resulting Secrets
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := serviceaccounttoken.EnsureSecretForServiceAccount(context.Background(), nil, clientset, serviceAccount.DeepCopy())
			s.Require().NoError(err)
		}()
	}
	wg.Wait()

	pollInterval := 500 * time.Millisecond
	err = wait.Poll(pollInterval, 5*time.Second, func() (done bool, err error) {
		secrets, err := clientset.CoreV1().Secrets(testNS.Name).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return false, err
		}

		return len(secrets.Items) > 0, nil
	})

	secrets, err := clientset.CoreV1().Secrets(testNS.Name).List(context.Background(), metav1.ListOptions{})
	s.Require().NoError(err)
	s.Assert().Equal(1, len(secrets.Items))
}

func TestSATestSuite(t *testing.T) {
	suite.Run(t, new(ServiceAccountSuite))
}
