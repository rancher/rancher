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
	CommitMode           = "commit"
	BranchMode           = "branch"
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

// isValidCommitHash checks if the provided input string is a valid git commit hash
//   - true: if valid
//   - false; if invalid
func isValidCommitHash(input string) bool {
	// Define a regular expression pattern for Git commit hashes.
	commitHashPattern := "^[0-9a-fA-F]{40}$"

	// Compile the regular expression pattern.
	regex, err := regexp.Compile(commitHashPattern)
	if err != nil {
		// Handle regex compilation error, if any.
		fmt.Println("Error compiling regex:", err)
		return false
	}

	// Use the regular expression to match the input string.
	return regex.MatchString(input)
}

// checkReference will test if the provided reference is a commit or a branch.
// Returns a constant string telling what the reference is.
//   - "commit"; Valid Commit Hash;
//   - "branch";
func checkReference(reference string) string {
	commit := isValidCommitHash(reference)
	if commit {
		return CommitMode
	}
	return BranchMode
}

// isGitSSH checks if the url is parsable to ssh url standard
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
