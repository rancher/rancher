package git

import (
	"fmt"

	"github.com/rancher/rancher/pkg/settings"
	corev1 "k8s.io/api/core/v1"
)

// Ensure builds the configuration for a should-existing repo and makes sure it is cloned or reseted to the latest commit of given branch
func Ensure(secret *corev1.Secret, namespace, name, gitURL, branch string, insecureSkipTLS bool, caBundle []byte) error {
	git, err := gitForRepo(secret, namespace, name, gitURL, insecureSkipTLS, caBundle)
	if err != nil {
		return err
	}

	return git.EnsureClonedRepo(branch)
}

// EnsureClonedRepo will check if repo is cloned, if not the method will clone and reset to the latest commit.
// If reseting to the latest commit is not possible it will fetch and try to reset again
func (er *extendedRepo) EnsureClonedRepo(branch string) error {

	err := er.cloneOrOpen("")
	if err != nil {
		return err
	}

	// before: g.reset(branch)
	// Try to reset to the given commit, if success exit
	localBranchFullName := fmt.Sprintf("refs/heads/%s", branch)
	err = er.hardReset(localBranchFullName)
	if err == nil {
		return nil
	}

	// before: fetchAndReset(branch)
	// If we do not have the commit locally, fetch and reset
	// return er.fetchAndReset(plumbing.ZeroHash, branch)
	return er.fetchAndReset(branch)
}

// Head builds the configuration for a new repo which will be cloned for the first time
func Head(secret *corev1.Secret, namespace, name, gitURL, branch string, insecureSkipTLS bool, caBundle []byte) (string, error) {
	git, err := gitForRepo(secret, namespace, name, gitURL, insecureSkipTLS, caBundle)
	if err != nil {
		return "", err
	}

	return git.CloneHead(branch)
}

// CloneHead clones the HEAD of a git branch and return the commit hash of the HEAD.
func (er *extendedRepo) CloneHead(branch string) (string, error) {
	err := er.cloneOrOpen(branch)
	if err != nil {
		return "", err
	}

	// before: reset("HEAD")
	err = er.hardReset("HEAD")
	if err != nil {
		return "", err
	}

	commit, err := er.getCurrentCommit()
	return commit.String(), err
}

// Update builds the configuration to update an existing repository
func Update(secret *corev1.Secret, namespace, name, gitURL, branch string, insecureSkipTLS bool, caBundle []byte) (string, error) {
	git, err := gitForRepo(secret, namespace, name, gitURL, insecureSkipTLS, caBundle)
	if err != nil {
		return "", err
	}

	if isBundled(git.GetConfig()) && settings.SystemCatalog.Get() == "bundled" {
		return Head(secret, namespace, name, gitURL, branch, insecureSkipTLS, caBundle)
	}

	commit, err := git.UpdateToLatestRef(branch)

	if err != nil && isBundled(git.GetConfig()) {
		return Head(secret, namespace, name, gitURL, branch, insecureSkipTLS, caBundle)
	}

	return commit, err
}

// UpdateToLatestRef will check if repository exists, if exists will check for latest commit and update to it.
// If the repository does not exist will try cloning again.
func (er *extendedRepo) UpdateToLatestRef(branch string) (string, error) {
	err := er.cloneOrOpen(branch)
	if err != nil {
		return "", err
	}

	// before: reset("HEAD")
	err = er.hardReset("HEAD")
	if err != nil {
		return "", err
	}

	commit, err := er.getCurrentCommit()
	if err != nil {
		return commit.String(), err
	}

	lastCommit, err := er.getLastCommitHash(branch, commit)
	if err != nil || lastCommit == commit {
		return commit.String(), err
	}

	// before: g.fetchAndReset(branch)
	// err = er.fetchAndReset(lastCommit, branch)
	err = er.fetchAndReset(branch)
	if err != nil {
		return commit.String(), err
	}

	lastCommitRef, err := er.getCurrentCommit()
	lastCommitHashStr := lastCommitRef.String()

	return lastCommitHashStr, err
}
