package git

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rancher/rancher/pkg/settings"
	corev1 "k8s.io/api/core/v1"
)

const (
	stateDir  = "management-state/git-repo"
	staticDir = "/var/lib/rancher-data/local-catalogs/v2"
)

func gitDir(namespace, name, gitURL string) string {
	staticDir := filepath.Join(staticDir, namespace, name, hash(gitURL))
	if s, err := os.Stat(staticDir); err == nil && s.IsDir() {
		return staticDir
	}
	return filepath.Join(stateDir, namespace, name, hash(gitURL))
}

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

	return git.Ensure(commit)
}

func isBundled(git *git) bool {
	return strings.HasPrefix(git.Directory, staticDir)
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

func isGitSSH(gitURL string) (bool, error) {
	// Matches URLs with the format [anything]@[anything]:[anything]
	return regexp.MatchString("(.+)@(.+):(.+)", gitURL)
}

func hash(gitURL string) string {
	b := sha256.Sum256([]byte(gitURL))
	return hex.EncodeToString(b[:])
}

// convertDERToPEM converts a src DER certificate into PEM with line breaks, header, and footer.
func convertDERToPEM(src []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:    "CERTIFICATE",
		Headers: map[string]string{},
		Bytes:   src,
	})
}
