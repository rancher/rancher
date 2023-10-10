package git

import (
	"fmt"

	plumbing "github.com/go-git/go-git/v5/plumbing"
)

// Ensure will check if repo is cloned, if not the method will clone and checkout to the ClusterRepo.Status.Commit.
// If checking out to the given commit is not possible it will fetch and reset to the commit.
func (r *Repository) Ensure(commit string) error {
	// fresh clone or open
	// if the HEAD pointing commit is supposed to change, then it should be done at Update or Head method
	err := r.cloneOrOpen("")
	if err != nil {
		return fmt.Errorf("failed to clone or open repository: %w", err)
	}

	// Try to checkout to the given commit, if success exit
	err = r.checkoutCommit(commit)
	if err == nil {
		return nil
	}

	// If we do not have the commit locally, fetch and reset to it
	err = r.fetchAndReset(commit)
	if err != nil {
		return fmt.Errorf("failed to fetch and/or reset at branch: %w", err)
	}
	return nil
}

// Head clones or sets a local repository to the HEAD of a git branch and return its commit hash.
func (r *Repository) Head(branch string) (string, error) {
	err := r.cloneOrOpen(branch)
	if err != nil {
		return "", fmt.Errorf("failed to clone or open repository: %w", err)
	}

	err = r.hardReset(string(plumbing.HEAD))
	if err != nil {
		return "", fmt.Errorf("failed to hard reset at HEAD: %w", err)
	}

	commit, err := r.getCurrentCommit()
	if err != nil {
		return commit.String(), fmt.Errorf("failed to get current commit: %w", err)
	}
	return commit.String(), nil
}

// CheckUpdate will check if rancher is in bundled mode,
// if it is not in bundled mode, it will make an update.
// if it is in bundled mode, will just call Head method since we never update on this mode.
func (r *Repository) CheckUpdate(branch, systemCatalogMode string) (string, error) {
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
func (r *Repository) Update(branch string) (string, error) {
	err := r.cloneOrOpen(branch)
	if err != nil {
		return "", fmt.Errorf("failed to clone or open: %w", err)
	}

	err = r.hardReset(string(plumbing.HEAD))
	if err != nil {
		return "", fmt.Errorf("failed to hard reset at HEAD: %w", err)
	}

	commit, err := r.getCurrentCommit()
	if err != nil {
		return commit.String(), fmt.Errorf("failed to get current commit: %w", err)
	}

	lastCommit, err := r.getLastCommitHash(branch, commit)
	if err != nil {
		return commit.String(), fmt.Errorf("failed to retrieve latest commit hash: %w", err)
	}
	if lastCommit == commit {
		return commit.String(), nil
	}

	err = r.fetchAndReset(branch)
	if err != nil {
		return "", fmt.Errorf("failed to fetch and/or reset at branch: %w", err)
	}

	lastCommitRef, err := r.getCurrentCommit()
	if err != nil {
		return lastCommitRef.String(), fmt.Errorf("failed to get current commit: %w", err)
	}
	return lastCommitRef.String(), nil
}
