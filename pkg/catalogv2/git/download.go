package git

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

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

	return git.Update(branch)
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

func gitForRepo(secret *corev1.Secret, namespace, name, gitURL string) (*Git, error) {
	dir := gitDir(namespace, name, gitURL)
	return NewGit(dir, gitURL, &Options{
		Credential: secret,
	})
}

func hash(gitURL string) string {
	b := sha256.Sum256([]byte(gitURL))
	return hex.EncodeToString(b[:])
}
