package git

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/pkg/git"
	corev1 "k8s.io/api/core/v1"
)

const (
	stateDir  = "management-state/git-repo"
	staticDir = "/var/lib/rancher-data/local-catalogs/v2"
)

func gitDir(namespace, name, gitURL string) string {
	staticDir := filepath.Join(staticDir, namespace, name)
	if s, err := os.Stat(staticDir); err == nil && s.IsDir() {
		return staticDir
	}
	return filepath.Join(stateDir, namespace, name, hash(gitURL))
}

func Head(secret *corev1.Secret, namespace, name, gitURL, branch string) (string, error) {
	if branch == "" {
		branch = "master"
	}

	git, err := gitForRepo(secret, namespace, name, gitURL)
	if err != nil {
		return "", err
	}

	return git.Head(branch)
}

func Update(secret *corev1.Secret, namespace, name, gitURL, branch string) (string, error) {
	if branch == "" {
		branch = "master"
	}

	git, err := gitForRepo(secret, namespace, name, gitURL)
	if err != nil {
		return "", err
	}

	if isBundled(git) && settings.SystemCatalog.Get() == "bundled" {
		return Head(secret, namespace, name, gitURL, branch)
	}

	commit, err := git.Update(branch)
	if err != nil && isBundled(git) {
		return Head(secret, namespace, name, gitURL, branch)
	}
	return commit, err
}

func Ensure(secret *corev1.Secret, namespace, name, gitURL, commit string) error {
	if commit == "" {
		return nil
	}
	git, err := gitForRepo(secret, namespace, name, gitURL)
	if err != nil {
		return err
	}

	return git.Ensure(commit)
}

func isBundled(git *git.Git) bool {
	return strings.HasPrefix(git.Directory, staticDir)
}

func gitForRepo(secret *corev1.Secret, namespace, name, gitURL string) (*git.Git, error) {
	if !strings.HasPrefix(gitURL, "git@") {
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
	return git.NewGit(dir, gitURL, &git.Options{
		Credential: secret,
		Headers:    headers,
	})
}

func hash(gitURL string) string {
	b := sha256.Sum256([]byte(gitURL))
	return hex.EncodeToString(b[:])
}
