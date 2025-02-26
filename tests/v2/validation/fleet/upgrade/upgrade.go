package upgrade

import (
	"fmt"
	"time"

	"github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	"github.com/rancher/rancher/tests/v2/actions/fleet"
	"github.com/rancher/shepherd/clients/rancher"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	extensionsfleet "github.com/rancher/shepherd/extensions/fleet"
	"github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/nodes"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	sshPublickey  = "ssh-publickey"
	sshPrivatekey = "ssh-privatekey"
)

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

func createLocalFleetGitRepo(client *rancher.Client, sshNode *nodes.Node, repoName string, namespaceName string, clusterName string, clusterID string, secretName string) (*v1.SteveAPIObject, error) {

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

func verifyNewGitCommit(client *rancher.Client, oldCommit string, repoObjectID string) error {
	logrus.Info("Checking Fleet Git Commit has been updated")
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

		return oldCommit != newGitStatus.Commit, nil
	})
	return err
}

func createFleetSSHSecret(client *rancher.Client, privateKey []byte) (string, error) {
	key, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return "", err
	}

	keyData := map[string][]byte{
		sshPublickey:  []byte(key.PublicKey().Marshal()),
		sshPrivatekey: privateKey,
	}

	secretName := namegenerator.AppendRandomString("fleet-ssh")
	secretTemplate := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: fleet.Namespace,
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
