package git

import (
	"fmt"
	"net/url"

	"github.com/rancher/rancher/pkg/settings"
	corev1 "k8s.io/api/core/v1"
)

func Head(secret *corev1.Secret, namespace, name, gitURL, branch string, insecureSkipTLS bool, caBundle []byte) (string, error) {
	git, err := gitForRepo(secret, namespace, name, gitURL, insecureSkipTLS, caBundle)
	if err != nil {
		return "", err
	}

	return git.Head(branch)
}

func Update(secret *corev1.Secret, namespace, name, gitURL, branch string, insecureSkipTLS bool, caBundle []byte) (string, error) {
	git, err := gitForRepo(secret, namespace, name, gitURL, insecureSkipTLS, caBundle)
	if err != nil {
		return "", err
	}

	if isBundled(git) && settings.SystemCatalog.Get() == "bundled" {
		return Head(secret, namespace, name, gitURL, branch, insecureSkipTLS, caBundle)
	}

	commit, err := git.Update(branch)
	if err != nil && isBundled(git) {
		return Head(secret, namespace, name, gitURL, branch, insecureSkipTLS, caBundle)
	}
	return commit, err
}

func Ensure(secret *corev1.Secret, namespace, name, gitURL, commit string, insecureSkipTLS bool, caBundle []byte) error {
	if commit == "" {
		return nil
	}
	git, err := gitForRepo(secret, namespace, name, gitURL, insecureSkipTLS, caBundle)
	if err != nil {
		return err
	}

	// If the repositories are rancher managed and if bundled is set
	// don't fetch anything from upstream.
	if isBundled(git) && settings.SystemCatalog.Get() == "bundled" {
		return nil
	}

	return git.Ensure(commit)
}

func gitForRepo(secret *corev1.Secret, namespace, name, gitURL string, insecureSkipTLS bool, caBundle []byte) (*git, error) {
	isGitSSH, err := isGitSSH(gitURL)
	if err != nil {
		return nil, fmt.Errorf("failed to verify the type of URL %s: %w", gitURL, err)
	}
	if !isGitSSH {
		u, err := url.Parse(gitURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse URL %s: %w", gitURL, err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return nil, fmt.Errorf("invalid git URL scheme %s, only http(s) and git supported", u.Scheme)
		}
	}
	dir := gitDir(namespace, name, gitURL)
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
