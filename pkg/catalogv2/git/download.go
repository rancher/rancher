package git

import (
	"fmt"
)

// Ensure will check if repo is cloned, if not the method will clone and reset to the latest commit.
// If reseting to the latest commit is not possible it will fetch and try to reset again
func (r *repository) Ensure(branch string) error {
	err := r.cloneOrOpen("")
	if err != nil {
		return err
	}

	// Try to reset to the given commit, if success exit
	localBranchFullName := fmt.Sprintf("refs/heads/%s", branch)
	err = r.hardReset(localBranchFullName)
	if err == nil {
		return nil
	}

	// If we do not have the commit locally, fetch and reset
	return r.fetchAndReset(branch)
}

// Head clones or sets a local repository to the HEAD of a git branch and return its commit hash.
func (r *repository) Head(branch string) (string, error) {
	err := r.cloneOrOpen(branch)
	if err != nil {
		return "", err
	}

	err = r.hardReset("HEAD")
	if err != nil {
		return "", err
	}

	commit, err := r.getCurrentCommit()
	return commit.String(), err
}

// CheckUpdate will check if rancher is in bundled mode,
// if it is not in bundled mode, will make an update.
// if it is in bundled mode, will just call Head method.
func (r *repository) CheckUpdate(branch, systemCatalogMode string) (string, error) {
	if isBundled(r.Directory) && systemCatalogMode == "bundled" {
		return r.Head(branch)
	}

	commit, err := r.Update(branch)
	if err != nil && isBundled(r.Directory) {
		return r.Head(branch)
	}

	return commit, err
}

// Update will check if repository exists, if exists will check for latest commit and update to it.
// If the repository does not exist will try cloning again.
func (r *repository) Update(branch string) (string, error) {
	err := r.cloneOrOpen(branch)
	if err != nil {
		return "", err
	}

	err = r.hardReset("HEAD")
	if err != nil {
		return "", err
	}

	commit, err := r.getCurrentCommit()
	if err != nil {
		return commit.String(), err
	}

	lastCommit, err := r.getLastCommitHash(branch, commit)
	if err != nil || lastCommit == commit {
		return commit.String(), err
	}

	err = r.fetchAndReset(branch)
	if err != nil {
		return commit.String(), err
	}

	lastCommitRef, err := r.getCurrentCommit()

	return lastCommitRef.String(), err
}
