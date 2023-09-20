package git

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	stateDir             = "management-state/git-repo" // used in development environment only
	staticDir            = "/var/lib/rancher-data/local-catalogs/v2"
	localDir             = "../rancher-data/local-catalogs/v2" // identical to helm.InternalCatalog
	localReferenceBranch = "refs/heads/"
)

// gitDir calculates the directory path where a Git repository's data should be stored or retrieved based on
// the given namespace, name, and Git URL.
// It uses a hash of the Git URL to ensure a unique directory structure for each repository.
func gitDir(namespace, name, gitURL string) string {
	// Default Absolute path for git helm repositories in a rancher production environment
	staticDir := filepath.Join(staticDir, namespace, name, hash(gitURL))
	if s, err := os.Stat(staticDir); err == nil && s.IsDir() {
		return staticDir
	}
	localDir := filepath.Join(localDir, namespace, name, hash(gitURL))
	if s, err := os.Stat(localDir); err == nil && s.IsDir() {
		return localDir
	}
	return filepath.Join(stateDir, namespace, name, hash(gitURL))
}

// hash takes a Git URL as input and returns its SHA-256 hash as a hexadecimal string.
// This function is used to generate a unique identifier for a Git URL.
func hash(gitURL string) string {
	b := sha256.Sum256([]byte(gitURL))
	return hex.EncodeToString(b[:])
}

// isBundled check if the repositories are bundled on local static directory
func isBundled(directory string) bool {
	return strings.HasPrefix(directory, staticDir) || strings.HasPrefix(directory, localDir)
}

// isLocalBranch checks if a given branch reference is a local branch.
//
// In Git, branches can be categorized into two main types: local branches and
// remote branches.
//
// This function determines if the provided branch is a local branch or not.
// Local branches in Git typically have references that start with "refs/heads/".
func isLocalBranch(branch string) bool {
	return strings.HasPrefix(branch, localReferenceBranch)
}

// isGitSSH checks if the URL is in the SSH URL format using regular expressions.
// [anything]@[anything]:[anything]
// ssh://<user>@<mydomain.example>:<port>/<path>/<repository-name>
func isGitSSH(gitURL string) (bool, error) {
	pattern1 := `^.+@.+:.+$`
	pattern2 := `^ssh://[^@]+@[^:]+:\d+/.+/.+$`
	return regexp.MatchString(pattern1+"|"+pattern2, gitURL)
}

// convertDERToPEM converts a DER-encoded certificate (src) into a PEM-encoded
// certificate with proper formatting, including line breaks, a header, and footer.
// It returns the PEM-encoded certificate as a byte slice.
func convertDERToPEM(src []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:    "CERTIFICATE",
		Headers: map[string]string{},
		Bytes:   src,
	})
}

// formatGitURL is used by remoteSHAChange
func formatGitURL(endpoint, branch string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}

	pathParts := strings.Split(u.Path, "/")
	switch u.Hostname() {
	case "github.com":
		if len(pathParts) >= 3 {
			org := pathParts[1]
			repo := strings.TrimSuffix(pathParts[2], ".git")
			return fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s", org, repo, branch)
		}
	case "git.rancher.io":
		repo := strings.TrimSuffix(pathParts[1], ".git")
		u.Path = fmt.Sprintf("/repos/%s/commits/%s", repo, branch)
		return u.String()
	}

	return ""
}
