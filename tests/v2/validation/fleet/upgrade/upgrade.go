package update

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/clusters"
	"github.com/rancher/rancher/tests/v2/actions/fleet"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/shepherd/clients/rancher"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	extensionsClusters "github.com/rancher/shepherd/extensions/clusters"
	extensionsfleet "github.com/rancher/shepherd/extensions/fleet"
	"github.com/rancher/shepherd/extensions/kubectl"
	"github.com/rancher/shepherd/extensions/sshkeys"
	"github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/nodes"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

func createSSHNode(client *rancher.Client, steveClient *steveV1.Client, clusterName string, clusterID string) (*nodes.Node, error) {

	provisioningClusterID, err := extensionsClusters.GetV1ProvisioningClusterByName(client, client.RancherConfig.ClusterName)
	if err != nil {
		return nil, err
	}

	cluster, err := client.Steve.SteveType(extensionsClusters.ProvisioningSteveResourceType).ByID(provisioningClusterID)
	if err != nil {
		return nil, err
	}

	newCluster := &provv1.Cluster{}
	err = steveV1.ConvertToK8sType(cluster, newCluster)
	if err != nil {
		return nil, err
	}

	sshNode := &nodes.Node{}
	if strings.Contains(newCluster.Spec.KubernetesVersion, "rke2") || strings.Contains(newCluster.Spec.KubernetesVersion, "k3s") {
		_, stevecluster, err := extensionsClusters.GetProvisioningClusterByName(client, clusterName, provisioninginput.Namespace)
		if err != nil {
			return nil, err
		}

		sshUser, err := sshkeys.GetSSHUser(client, stevecluster)
		if err != nil {
			return nil, err
		}

		logrus.Infof("Getting the node using the label [%v]", clusters.LabelWorker)
		query, err := url.ParseQuery(clusters.LabelWorker)
		if err != nil {
			return nil, err
		}

		nodeList, err := steveClient.SteveType("node").List(query)
		if err != nil {
			return nil, err
		}

		if len(nodeList.Data) == 0 {
			return nil, errors.New("node list is empty")
		}

		firstMachine := nodeList.Data[0]

		sshNode, err = sshkeys.GetSSHNodeFromMachine(client, sshUser, &firstMachine)
		if err != nil {
			return nil, err
		}
	} else {
		v3NodeList, err := client.WranglerContext.Mgmt.Node().List(clusterID, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		if len(v3NodeList.Items) == 0 {
			return nil, errors.New("node list is empty")
		}

		v3Node := v3NodeList.Items[0]

		externalIP := v3Node.Status.NodeAnnotations["rke.cattle.io/external-ip"]

		sshKey, err := downloadRKESSHKey(client, clusterID, &v3Node)
		if err != nil {
			return nil, err
		}

		sshNode = &nodes.Node{
			NodeID:          v3Node.Name,
			PublicIPAddress: externalIP,
			SSHUser:         v3Node.Status.NodeConfig.User,
			SSHKey:          []byte(sshKey),
		}
	}

	return sshNode, nil
}

func downloadRKESSHKey(client *rancher.Client, clusterID string, v3Node *v3.Node) (string, error) {
	downloadUrl := fmt.Sprintf("https://%s/v3/nodes/%s:%s/nodeconfig", client.RancherConfig.Host, clusterID, v3Node.Name)
	autorizationBearer := fmt.Sprintf("Authorization:Bearer %s", client.RancherConfig.AdminToken)

	fileName := namegenerator.AppendRandomString("file-name")
	zipFile := fmt.Sprintf("%s.zip", fileName)

	curlCommand := fmt.Sprintf("curl -s -sSL -k -H '%s' %s --output %s", autorizationBearer, downloadUrl, zipFile)
	unzipCommand := fmt.Sprintf("unzip -q %s -d %s", zipFile, fileName)
	catCommand := fmt.Sprintf("cat %s/%s/id_rsa", fileName, client.RancherConfig.ClusterName)
	bashCommand := fmt.Sprintf("%s && %s && %s", curlCommand, unzipCommand, catCommand)

	execCmd := []string{"bash", "-c", bashCommand}
	return kubectl.Command(client, nil, clusterID, execCmd, "")
}

func gitPushCommit(client *rancher.Client, sshNode *nodes.Node, repoName string) error {

	gitUserName := namegenerator.AppendRandomString("git-user")

	logrus.Info("Configuring Git User Name")
	_, err := sshNode.ExecuteCommand(fmt.Sprintf("git -C %s config --local user.name %s", repoName, gitUserName))
	if err != nil {
		return err
	}

	logrus.Info("Configuring Git User Email")
	_, err = sshNode.ExecuteCommand(fmt.Sprintf("git -C %s config --local user.email %s", repoName, gitUserName))
	if err != nil {
		return err
	}

	logrus.Info("Git Commit")
	_, err = sshNode.ExecuteCommand(fmt.Sprintf("git -C %s commit --allow-empty -m 'Trigger fleet update'", repoName))
	return err
}

func createFleetGitRepo(client *rancher.Client, sshNode *nodes.Node, repoName string, namespaceName string, clusterName string, clusterID string, secretName string) (*v1.SteveAPIObject, error) {

	gitSSHRepo := fmt.Sprintf(fmt.Sprintf("%s@%s:/home/%s/%s", sshNode.SSHUser, sshNode.PublicIPAddress, sshNode.SSHUser, repoName))

	fleetGitRepo := &v1alpha1.GitRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fleet.FleetMetaName + namegenerator.RandStringLower(5),
			Namespace: fleet.Namespace,
		},
		Spec: v1alpha1.GitRepoSpec{
			Repo:            gitSSHRepo,
			Branch:          fleet.BranchName,
			TargetNamespace: namespaceName,
			Paths:           []string{fleet.GitRepoPathLinux},
			Targets: []v1alpha1.GitTarget{
				{
					ClusterName: clusterName,
				},
			},
			ClientSecretName: secretName,
		},
	}

	logrus.Info("Creating a fleet git repo")
	repoObject, err := extensionsfleet.CreateFleetGitRepo(client, fleetGitRepo)
	if err != nil {
		return nil, err
	}

	logrus.Info("Verifying the Git repository")
	err = fleet.VerifyGitRepo(client, repoObject.ID, clusterID, fmt.Sprintf("%s/%s", fleet.Namespace, clusterName))
	if err != nil {
		return nil, err
	}

	return repoObject, nil
}

func verifyGitCommit(client *rancher.Client, commit string, repoObjectID string) error {
	backoff := kwait.Backoff{
		Duration: 1 * time.Second,
		Factor:   1.1,
		Jitter:   0.1,
		Steps:    20,
	}

	err := kwait.ExponentialBackoff(backoff, func() (finished bool, err error) {
		gitRepo, err := client.Steve.SteveType(extensionsfleet.FleetGitRepoResourceType).ByID(repoObjectID)
		if err != nil {
			return true, err
		}

		newGitStatus := &v1alpha1.GitRepoStatus{}
		err = steveV1.ConvertToK8sType(gitRepo.Status, newGitStatus)
		if err != nil {
			return true, err
		}

		return commit != newGitStatus.Commit, nil
	})
	return err
}

func createFleetSSHSecret(client *rancher.Client, privateKey string) (string, error) {
	key, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		return "", err
	}

	keyData := map[string][]byte{
		"ssh-publickey":  []byte(key.PublicKey().Marshal()),
		"ssh-privatekey": []byte(privateKey),
	}

	secretName := namegenerator.AppendRandomString("fleet-ssh")
	secretTemplate := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: "fleet-default",
		},
		Data: keyData,
		Type: corev1.SecretTypeSSHAuth,
	}

	secretResp, err := client.WranglerContext.Core.Secret().Create(&secretTemplate)

	if err != nil {
		return "", err
	}

	return secretResp.Name, nil
}
