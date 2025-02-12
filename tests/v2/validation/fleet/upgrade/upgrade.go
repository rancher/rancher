package update

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/git"
	"github.com/rancher/rancher/tests/v2/actions/fleet"
	"github.com/rancher/shepherd/clients/rancher"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	extensionsfleet "github.com/rancher/shepherd/extensions/fleet"
	"github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	gitSecret = "gitsecret"
)

func gitPushCommit(client *rancher.Client, repoName string, repoUrl string, username string, accessToken string) error {
	logrus.Info("Cloning Repository")
	err := git.Clone(repoName, repoUrl, fleet.BranchName)
	if err != nil {
		return err
	}

	logrus.Info("Configuring Git User Name")
	cmd := exec.Command("git", "-C", repoName, "config", "--local", "user.name", username)
	err = cmd.Run()
	if err != nil {
		return err
	}

	logrus.Info("Configuring Git User Email")
	cmd = exec.Command("git", "-C", repoName, "config", "--local", "user.email", username)
	err = cmd.Run()
	if err != nil {
		return err
	}

	logrus.Info("Git Commit")
	cmd = exec.Command("git", "-C", repoName, "commit", "--allow-empty", "-m", "Trigger fleet update")
	err = cmd.Run()
	if err != nil {
		return err
	}

	logrus.Info("Git Push")
	repoWithoutHttps := strings.Replace(repoUrl, "https://", "", 1)
	repoWithToken := fmt.Sprintf("https://%s:%s@%s", username, accessToken, repoWithoutHttps)
	cmd = exec.Command("git", "-C", repoName, "push", "--force", "-u", repoWithToken)
	err = cmd.Run()

	return err
}

func createFleetGitRepo(client *rancher.Client, repo string, namespaceName string, clusterName string, clusterID string) (*v1.SteveAPIObject, error) {
	fleetGitRepo := &v1alpha1.GitRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fleet.FleetMetaName + namegenerator.RandStringLower(5),
			Namespace: fleet.Namespace,
		},
		Spec: v1alpha1.GitRepoSpec{
			Repo:            repo,
			Branch:          fleet.BranchName,
			TargetNamespace: namespaceName,
			Paths:           []string{fleet.GitRepoPathLinux},
			Targets: []v1alpha1.GitTarget{
				{
					ClusterName: clusterName,
				},
			},
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
