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
)

const (
	stateDir             = "management-state/git-repo"
	staticDir            = "/var/lib/rancher-data/local-catalogs/v2"
	localDir             = "../rancher-data/local-catalogs/v2" // identical to helm.InternalCatalog
	localReferenceBranch = "refs/heads/"
)

func gitDir(namespace, name, gitURL string) string {
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

// isBundled check if the repositories are bundled on local static directory
func isBundled(directory string) bool {
	return strings.HasPrefix(directory, staticDir) || strings.HasPrefix(directory, localDir)
}

func isLocalBranch(branch string) bool {
	return strings.HasPrefix(branch, localReferenceBranch)
}

// isGitSSH checks if the URL is in the SSH URL format.
func isGitSSH(gitURL string) (bool, error) {
	// Define two regular expressions to match the two URL patterns
	pattern1 := `^.+@.+:.+$`                    // [anything]@[anything]:[anything]
	pattern2 := `^ssh://[^@]+@[^:]+:\d+/.+/.+$` // ssh://<user>@<mydomain.example>:<port>/<path>/<repository-name>

	// Check if the URL matches either of the patterns
	return regexp.MatchString(pattern1+"|"+pattern2, gitURL)
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
