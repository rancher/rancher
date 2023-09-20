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

// validateGitURL checks if the provided URL uses one of the supported protocols:
//   - HTTP(S)
//   - SSH
//
// It also validates if the SSH URL is well-formed.
//
// Returns a boolean value indicating the communication protocol:
//   - false for HTTP(S)
//   - true for SSH
//
// If the URL is invalid or uses an unsupported protocol, an error is returned.
func validateGitURL(gitURL string) (bool, error) {
	// check for https and ssh prefix first
	isHTTP := strings.HasPrefix(gitURL, "http://") || strings.HasPrefix(gitURL, "https://")
	if isHTTP {
		return false, nil
	}
	// It has to be a valid URL, if it is not, throw an error
	return isGitSSH(gitURL) // SSH
}

// isGitSSH checks if the URL is in the SSH URL format using regular expressions.
// [anything]@[anything]:[anything]
// ssh://<user>@<mydomain.example>:<port>/<path>/<repository-name>
func isGitSSH(gitURL string) (bool, error) {
	pattern1 := `^[^:/]+@[^:]+:.+$`
	pattern2 := `^ssh://[^@]+@[^:]+:\d+/.+$`
	validSSH, err := regexp.MatchString(pattern1+"|"+pattern2, gitURL)
	if err != nil {
		return true, fmt.Errorf("regexp failed: %w", err)
	}
	if !validSSH {
		return true, fmt.Errorf("only http(s) or ssh protocols supported")
	}
	// valid SSH URL
	return true, nil
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

// parseUserFromSSHURL will receive a valid SSH url and extract the user from it
// ssh://[user@]server/project.git
// [user@]server:project.git
func parseUserFromSSHURL(URL string) (user string, err error) {
	// separate string parts from "@" character
	parts := strings.Split(URL, "@")
	user = parts[0]
	if len(parts) == 2 {
		if strings.HasPrefix(parts[0], "ssh://") {
			// Remove "ssh://" prefix
			user = parts[0][len("ssh://"):]
		} else {
			user = parts[0]
		}
	} else {
		return "", fmt.Errorf("invalid ssh url: %v", URL)
	}

	return user, nil
}

// checkOSDefaultSSHKeys will look at the OS default $HOME/.ssh directory
// for existing SSH Keys with the default names and return the private key slice of bytes for parsing
func checkOSDefaultSSHKeys() ([]byte, error) {
	// Get the home directory of the current user
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return []byte{}, fmt.Errorf("error getting user home directory: %w", err)
	}

	// Construct the path to the SSH private and public keys
	publicKeyPath := filepath.Join(homeDir, ".ssh", "id_rsa.pub")
	privateKeyPath := filepath.Join(homeDir, ".ssh", "id_rsa")

	// Read the contents of both files but only return the private one
	pvtKeyBytes, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return []byte{}, err
	}
	pubKeyBytes, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return []byte{}, err
	}

	// Keys must not be empty
	if len(pvtKeyBytes) == 0 || len(pubKeyBytes) == 0 {
		return []byte{}, fmt.Errorf("empty ssh keys given")
	}

	return pvtKeyBytes, nil
}
