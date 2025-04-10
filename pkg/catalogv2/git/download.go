package git

import (
	"fmt"
	"regexp"

	"github.com/rancher/rancher/pkg/settings"
	corev1 "k8s.io/api/core/v1"
)

// Ensure runs git clone, clean DIRTY contents and fetch the latest commit
func Ensure(secret *corev1.Secret, namespace, name, gitURL, commit string, insecureSkipTLS bool, caBundle []byte) error {
	git, err := gitForRepo(secret, namespace, name, gitURL, insecureSkipTLS, caBundle)
	if err != nil {
		return fmt.Errorf("ensure failure: %w", err)
	}

	// If the repositories are rancher managed and if bundled is set
	// don't fetch anything from upstream.
	if IsBundled(git.Directory) && settings.SystemCatalog.Get() == "bundled" {
		return nil
	}

	if err := git.clone(""); err != nil {
		return fmt.Errorf("ensure failure: %w", err)
	}

	if err := git.reset(commit); err == nil {
		return nil
	}

	if err := git.fetchAndReset(commit); err != nil {
		return fmt.Errorf("ensure failure: %w", err)
	}
	return nil
}

// Head runs git clone on directory(if not exist), reset dirty content and return the HEAD commit
func Head(secret *corev1.Secret, namespace, name, gitURL, branch string, insecureSkipTLS bool, caBundle []byte) (string, error) {
	git, err := gitForRepo(secret, namespace, name, gitURL, insecureSkipTLS, caBundle)
	if err != nil {
		return "", fmt.Errorf("head failure: %w", err)
	}

	if err := git.clone(branch); err != nil {
		return "", fmt.Errorf("head failure: %w", err)
	}

	if err := git.reset("HEAD"); err != nil {
		return "", fmt.Errorf("head failure: %w", err)
	}

	commit, err := git.currentCommit()
	if err != nil {
		return "", fmt.Errorf("head failure: %w", err)
	}

	return commit, nil
}

// Update updates git repo if remote sha has changed. It also skips the update if in bundled mode and the git dir has a certain prefix stateDir(utils.go).
// If there is an error updating the repo especially stateDir(utils.go) repositories, it ignores the error and returns the current commit in the local pod directory.
// except when the error is `branch not found`. It specifically checks for `couldn't find remote ref` & `Could not find remote branch` in the error message.
func Update(secret *corev1.Secret, namespace, name, gitURL, branch string, insecureSkipTLS bool, caBundle []byte) (string, error) {
	git, err := gitForRepo(secret, namespace, name, gitURL, insecureSkipTLS, caBundle)
	if err != nil {
		return "", fmt.Errorf("update failure: %w", err)
	}
	if IsBundled(git.Directory) && settings.SystemCatalog.Get() == "bundled" {
		return Head(secret, namespace, name, gitURL, branch, insecureSkipTLS, caBundle)
	}

	commit, err := git.Update(branch)
	if err != nil && IsBundled(git.Directory) {
		// We don't report an error unless the branch is invalid
		// The reason being it would break airgap environments in downstream
		// cluster. A new issue is created to tackle this in the forthcoming.
		if checkInvalidBranch(err) {
			return "", err
		}
		return Head(secret, namespace, name, gitURL, branch, insecureSkipTLS, caBundle)
	}
	return commit, err
}

func gitForRepo(secret *corev1.Secret, namespace, name, gitURL string, insecureSkipTLS bool, caBundle []byte) (*git, error) {
	err := validateURL(gitURL)
	if err != nil {
		return nil, fmt.Errorf("%w: only http(s) or ssh:// supported", err)
	}

	dir := RepoDir(namespace, name, gitURL)
	headers := map[string]string{}
	if settings.InstallUUID.Get() != "" {
		headers["X-Install-Uuid"] = settings.InstallUUID.Get()
	}
	// convert caBundle to PEM format because git requires correct line breaks, header and footer.
	if len(caBundle) > 0 {
		caBundle = convertDERToPEM(caBundle)
		insecureSkipTLS = false
	}
	return newGit(dir, gitURL, &Options{
		Credential:        secret,
		Headers:           headers,
		InsecureTLSVerify: insecureSkipTLS,
		CABundle:          caBundle,
	})
}

// checkInvalidBranch checks if the error indicates that the branch does not exist.
// There is no errorCode that we could use here and so falling back to regex checking.
func checkInvalidBranch(err error) bool {
	regexPatterns := []string{
		".*couldn't find remote ref.*",
		".*Could not find remote branch.*",
		".*Remote branch .* not found in upstream origin.*",
	}

	for _, pattern := range regexPatterns {
		matched, _ := regexp.MatchString(pattern, err.Error())
		if matched {
			return true
		}
	}

	return false
}
