package git

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

const (
	stateDir  = "management-state/git-repo"
	staticDir = "/var/lib/rancher-data/local-catalogs/v2"
	localDir  = "../rancher-data/local-catalogs/v2" // identical to helm.InternalCatalog
)

// RepoDir returns the directory where the git repo is cloned.
func RepoDir(namespace, name, gitURL string) string {
	staticDir := filepath.Join(staticDir, namespace, name, Hash(gitURL))
	if s, err := os.Stat(staticDir); err == nil && s.IsDir() {
		return staticDir
	}
	localDir := filepath.Join(localDir, namespace, name, Hash(gitURL))
	if s, err := os.Stat(localDir); err == nil && s.IsDir() {
		return localDir
	}
	return filepath.Join(stateDir, namespace, name, Hash(gitURL))
}

// IsBundled checks the directory to see if it is a bundled catalog repository.
func IsBundled(dir string) bool {
	return strings.HasPrefix(dir, staticDir) || strings.HasPrefix(dir, localDir)
}

// isGitSSH checks if the URL is in the SSH URL format using regular expressions.
// [anything]@[anything]:[anything]
// ssh://<user>@<mydomain.example>:<port>/<path>/<repository-name>
func isGitSSH(gitURL string) bool {
	pattern1 := `^[^:/]+@[^:]+:[a-zA-Z]+/[^/]+$`
	pattern2 := `^ssh://[^@]+@[^:]+:\d+/.+$`

	// Check if the input matches either of the two patterns.
	valid, err := regexp.MatchString(pattern1, gitURL)
	if err != nil {
		return false
	}
	if valid {
		return true
	}
	valid, err = regexp.MatchString(pattern2, gitURL)
	if err != nil {
		return false
	}

	return valid
}

// validateURL will validate if the provided URL is in one of the expected patterns
// for the supported protocols http(s) or ssh.
//   - if Valid: returns nil
//   - if Invalid: returns an error
func validateURL(gitURL string) error {
	valid := isGitSSH(gitURL)
	if valid {
		return nil
	}
	// not ssh; validate http(s)
	u, err := url.Parse(gitURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("invalid git URL: %s", gitURL)
	}

	return nil
}

// Hash returns a hash of the git URL.
func Hash(gitURL string) string {
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

func firstField(lines []string, errText string) (string, error) {
	if len(lines) == 0 {
		return "", errors.New(errText)
	}

	fields := strings.Fields(lines[0])
	if len(fields) == 0 {
		return "", errors.New(errText)
	}

	if len(fields[0]) == 0 {
		return "", errors.New(errText)
	}

	return fields[0], nil
}

func formatRefForBranch(branch string) string {
	return fmt.Sprintf("refs/heads/%s", branch)
}

type basicRoundTripper struct {
	username string
	password string
	next     http.RoundTripper
}

func (b *basicRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	request.SetBasicAuth(b.username, b.password)
	return b.next.RoundTrip(request)
}
