package git

import (
	"errors"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"

	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"

	// gogit packages
	gogit "github.com/go-git/go-git/v5"
	config "github.com/go-git/go-git/v5/config"
	plumbing "github.com/go-git/go-git/v5/plumbing"
	transport "github.com/go-git/go-git/v5/plumbing/transport"
	plumbingHTTP "github.com/go-git/go-git/v5/plumbing/transport/http"
	plumbingSSH "github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// repository holds the configuration of a git repository and the repo instance.
type Repository struct {
	secret            *corev1.Secret // Kubernetes secret holding credentials
	URL               string
	Directory         string
	username          string
	password          string
	caBundle          []byte
	knownHosts        []byte
	insecureTLSVerify bool
	protocol          string
	// go-git package objects
	auth      transport.AuthMethod
	localGit  *gogit.Repository
	cloneOpts *gogit.CloneOptions
	fetchOpts *gogit.FetchOptions
	listOpts  *gogit.ListOptions
	resetOpts *gogit.ResetOptions
}

// BuildRepoConfig constructs and returns a new repository object for the given repository.
// If the Git URL uses the SSH protocol, it checks if the URL is a valid SSH URL and parses the user from it.
// If the Git URL uses HTTP(S), it parses and verifies the URL.
// It then constructs a directory path for the git repository.
// If a CA bundle is provided, it converts the CA bundle from DER to PEM format since Git requires PEM format.
// In this case, insecureSkipTLS is set to false since a CA bundle is provided for secure communication.
// Finally, it returns a new git object configured with these settings,
// or an error if any step in this process fails.
func BuildRepoConfig(secret *corev1.Secret, namespace, name, gitURL string, insecureSkipTLS bool, caBundle []byte) (*Repository, error) {
	var err error

	repo := &Repository{
		URL:               gitURL,
		insecureTLSVerify: insecureSkipTLS,
		secret:            secret,
		localGit:          &gogit.Repository{},
		cloneOpts:         &gogit.CloneOptions{},
		fetchOpts:         &gogit.FetchOptions{},
		listOpts:          &gogit.ListOptions{},
		resetOpts:         &gogit.ResetOptions{},
	}

	// Check which supported communication protocol will be used (HTTP(S)/SSH)
	repo.protocol, err = validateGitURL(gitURL)
	if err != nil {
		return repo, fmt.Errorf("invalid URL: %s; error: %w", gitURL, err)
	}

	if repo.protocol == SSH && repo.secret == nil {
		// SSH without Secret, get keys from local system OS
		err := repo.checkDefaultSSHAgent()
		if err != nil {
			return repo, err
		}
	}
	// build Rancher git helm repository directory path pattern
	repo.Directory = gitDir(namespace, name, gitURL)

	// check if a CA Bundle was provided
	if len(caBundle) > 0 {
		// convert caBundle to PEM format
		repo.caBundle = convertDERToPEM(caBundle)
		repo.insecureTLSVerify = false
	}

	// Check and extract sensitive credentials if necessary
	err = repo.setRepoCredentials()
	if err != nil {
		return repo, err
	}
	// Apply the extracted credentials to the git repo operation options
	repo.setRepoOptions()
	return repo, nil
}

// setRepoCredentials detects which type of authentication from the Kubernetes secret
// and configures the git repo authentication interfaces accordingly.
func (r *Repository) setRepoCredentials() error {
	// If no secret provided then we will not be using any authentication credentials
	if r.secret == nil {
		return nil
	}

	// There is a secret, parse sensitive credentials from it
	switch {
	case r.secret.Type == corev1.SecretTypeBasicAuth && r.protocol == HTTPS: // HTTP(S) AUTHENTICATION
		// extract data from Kubernetes secret
		username := string(r.secret.Data[corev1.BasicAuthUsernameKey])
		password := string(r.secret.Data[corev1.BasicAuthPasswordKey])
		if len(password) == 0 || len(username) == 0 {
			return fmt.Errorf("username or password not provided")
		}
		// Set up authentication interface
		r.auth = &plumbingHTTP.BasicAuth{
			Username: username,
			Password: password,
		}
		return nil

	case r.secret.Type == corev1.SecretTypeSSHAuth && r.protocol == SSH: // SSH AUTHENTICATION
		// Kubernetes secret
		pemBytes := r.secret.Data[corev1.SSHAuthPrivateKey]
		// Make signer from private keys
		signer, err := ssh.ParsePrivateKey(pemBytes)
		if err != nil {
			return fmt.Errorf("failed to parse ssh private key: %w", err)
		}
		// Retrieve the username from the URL
		r.username, err = parseUserFromSSHURL(r.URL)
		if err != nil {
			return fmt.Errorf("failed to parse user from SSH URL: %w", err)
		}
		// Create an AuthMethod using the parsed private key
		r.auth = &plumbingSSH.PublicKeys{
			User:   r.username,
			Signer: signer,
		}

		// Check Kubernetes secret for Known Hosts data
		hostsBytes := r.secret.Data["known_hosts"]
		if len(hostsBytes) > 0 {
			// Create temporary known_hosts file
			f, err := os.CreateTemp("", "known_hosts")
			if err != nil {
				return fmt.Errorf("failed to create temporary known_hosts file: %w", err)
			}
			// Write received knownhosts from kubernetes secret
			_, err = f.Write(hostsBytes)
			if err != nil {
				return fmt.Errorf("failed to write knonw_hosts file: %w", err)
			}
			// Create callback from recently created temporary file
			// This will hold the known hosts in-memory so we can delete the file
			hostKeyCB, err := plumbingSSH.NewKnownHostsCallback(f.Name())
			if err != nil {
				return fmt.Errorf("setRepoCredentials at known hosts failure: %w", err)
			}
			r.auth = &plumbingSSH.PublicKeys{
				User:                  r.username,
				Signer:                signer,
				HostKeyCallbackHelper: plumbingSSH.HostKeyCallbackHelper{HostKeyCallback: hostKeyCB},
			}

			// Close and delete known_hosts file after setting up the callback
			err = f.Close()
			if err != nil {
				return fmt.Errorf("failed to close known hosts file: %w", err)
			}
			err = os.Remove(f.Name())
			if err != nil {
				return fmt.Errorf("failed to remove temporar known_hosts file: %w", err)
			}
		}

		return nil
	}
	// if all else failed, something nasty happened
	return errors.New("could not set repository credentials")
}

// checkDefaultSSHAgent checks if there are SSH keys located at the system's default path,
// parses these keys, retrieves the username from the URL, and implements an AuthMethod interface
// for go-git using the parsed private key. If successful, it sets the AuthMethod in the Repository.
// It returns an error if SSH keys are not found, fail to parse, or if the URL has an unsupported scheme.
func (r *Repository) checkDefaultSSHAgent() error {
	// Attempt to read system SSH keys
	sysPvtKey, err := checkOSDefaultSSHKeys()
	if err != nil {
		return fmt.Errorf("no ssh keys provided neither by secret or at default system path: %w", err)
	}

	// Parse the system's private key
	signer, err := ssh.ParsePrivateKey(sysPvtKey)
	if err != nil {
		return fmt.Errorf("failed to parse ssh private key: %w", err)
	}

	// Retrieve the username from the URL
	r.username, err = parseUserFromSSHURL(r.URL)
	if err != nil {
		return fmt.Errorf("invalid git URL scheme, only http(s) or ssh supported")
	}

	// Create an AuthMethod using the parsed private key
	r.auth = &plumbingSSH.PublicKeys{
		User:   r.username,
		Signer: signer,
	}

	return nil
}

// setRepoOptions assigns the options configured before in credentials.
// Hard-code other needed configurations like Depth for faster cloning.
func (r *Repository) setRepoOptions() {
	// Clone Options
	r.cloneOpts.URL = r.URL
	r.cloneOpts.Depth = 1
	r.cloneOpts.InsecureSkipTLS = r.insecureTLSVerify
	r.cloneOpts.Tags = gogit.NoTags

	// Fetch Options
	r.fetchOpts.RemoteURL = r.URL
	r.fetchOpts.InsecureSkipTLS = r.insecureTLSVerify
	r.fetchOpts.Tags = gogit.NoTags
	r.fetchOpts.Depth = 1

	// List Options
	r.listOpts.InsecureSkipTLS = r.insecureTLSVerify

	// Reset Options
	r.resetOpts.Mode = gogit.HardReset

	// Set authentication Methods for cloning
	if r.auth != nil {
		r.cloneOpts.Auth = r.auth
		r.fetchOpts.Auth = r.auth
		r.listOpts.Auth = r.auth
	}
	// Set CABundle for all git operations
	if len(r.caBundle) > 0 {
		r.cloneOpts.CABundle = r.caBundle
		r.fetchOpts.CABundle = r.caBundle
		r.listOpts.CABundle = r.caBundle
	}
	// Debug if enabled
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		r.cloneOpts.Progress = os.Stdout
		r.fetchOpts.Progress = os.Stdout
	}
}

// cloneOrOpen executes the clone operation of a Git repository at the given branch with depth = 1 if it does not exist.
// If it exists, it just opens the local repository and assigns it to ro.localRepo.
func (r *Repository) cloneOrOpen(branch string) error {
	cloneOptions := r.cloneOpts
	if branch != "" {
		cloneOptions.ReferenceName = plumbing.NewBranchReferenceName(branch)
	}

	err := cloneOptions.Validate()
	if err != nil {
		return fmt.Errorf("plainClone validation failure: %w", err)
	}

	openErr := r.plainOpen()
	if openErr != nil && openErr != gogit.ErrRepositoryNotExists {
		return fmt.Errorf("plainOpen failure: %w", err)
	} else if openErr == gogit.ErrRepositoryNotExists {
		localGit, cloneErr := gogit.PlainClone(r.Directory, false, cloneOptions)
		if cloneErr != nil && cloneErr != gogit.ErrRepositoryAlreadyExists {
			return fmt.Errorf("plainClone failure: %w", cloneErr)
		}
		// serious problem warning
		if openErr == gogit.ErrRepositoryNotExists && cloneErr == gogit.ErrRepositoryAlreadyExists {
			return fmt.Errorf("serious failure, neither open or clone succeeded: %w", cloneErr)
		}
		r.localGit = localGit
		return nil
	}

	return nil
}

// plainOpen opens an existing local Git repository on the specified folder without walking parent directories looking for '.git/'.
func (r *Repository) plainOpen() error {
	openOptions := gogit.PlainOpenOptions{
		DetectDotGit: false,
	}
	localRepository, err := gogit.PlainOpenWithOptions(r.Directory, &openOptions)
	if err != nil {
		return err
	}

	r.localGit = localRepository
	return nil
}

// getCurrentCommit returns the commit hash of the HEAD from the current branch
func (r *Repository) getCurrentCommit() (plumbing.Hash, error) {
	headRef, err := r.localGit.Head()
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("getCurrentCommit failure: %w", err)
	}

	return headRef.Hash(), nil
}

// fetchAndReset is a convenience method that fetches updates from the remote repository
// for a specific branch and then resets the current branch to a specified commit.
func (r *Repository) fetchAndReset(branch string) error {
	if err := r.fetch(branch); err != nil {
		return fmt.Errorf("fetchAndReset failure: %w", err)
	}

	return r.hardReset(branch)
}

// updateRefSpec updates the reference specification (RefSpec) in the fetch options
// of the repository operation.
//   - If a branch name is provided, it sets the RefSpec to fetch that specific branch.
//   - Otherwise, it sets the RefSpec to fetch all branches.
//
// Fetching the last commit of one branch is faster than fetching from all branches.
func (r *Repository) updateRefSpec(branch string) {
	var newRefSpec string

	if branch != "" {
		newRefSpec = fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", branch, branch)
	} else {
		newRefSpec = "+refs/heads/*:refs/remotes/origin/*"
	}

	if len(r.fetchOpts.RefSpecs) > 0 {
		r.fetchOpts.RefSpecs[0] = config.RefSpec(newRefSpec)
	} else {
		r.fetchOpts.RefSpecs = []config.RefSpec{config.RefSpec(newRefSpec)}
	}
}

// fetch fetches updates from the remote repository for a specific branch.
// If the fetch operation is already up-to-date(NoErrAlreadyUpToDate or ErrEmptyUploadPackRequest),
// this is not treated as an error, any other error that occurs during fetch is returned.
func (r *Repository) fetch(branch string) error {
	r.updateRefSpec(branch)
	fetchOptions := r.fetchOpts

	err := r.localGit.Fetch(fetchOptions)
	if err != nil && err != gogit.NoErrAlreadyUpToDate && err != transport.ErrEmptyUploadPackRequest {
		return fmt.Errorf("fetch failure: %w", err)
	}

	return nil
}

// hardReset performs a hard reset of the Git repository to a specific reference.
//   - HEAD (local latest commit)
//   - local branch reference ("refs/heads/<some-branch>")
//   - remote branch reference
func (r *Repository) hardReset(reference string) error {
	var err error
	resetOpts := r.resetOpts

	switch {
	case isLocalBranch(reference):
		branchRef, err := r.localGit.Reference(plumbing.ReferenceName(reference), true)
		if err != nil {
			return fmt.Errorf("hardReset failure, branch does not exist locally: %w", err)
		}
		resetOpts.Commit = branchRef.Hash()
	case reference == plumbing.HEAD.String():
		commitHash, err := r.getCurrentCommit()
		if err != nil {
			return fmt.Errorf("hardReset failure to get current commit: %w", err)
		}
		resetOpts.Commit = commitHash
	default:
		branchRef, err := r.localGit.Reference(plumbing.NewRemoteReferenceName("origin", reference), false)
		if err != nil {
			return fmt.Errorf("hardReset failure to get branch reference: %w", err)
		}
		resetOpts.Commit = branchRef.Hash()
	}

	// Validate hashCommit and reset options
	err = resetOpts.Validate(r.localGit)
	if err != nil {
		return fmt.Errorf("hardReset validation failure: %w", err)
	}

	// Open new worktree
	wt, err := r.localGit.Worktree()
	if err != nil {
		return fmt.Errorf("hardReset failure on WorkTree: %w", err)
	}

	// Reset
	err = wt.Reset(resetOpts)
	if err != nil {
		return fmt.Errorf("hardReset failure: %w", err)
	}

	return nil
}

// getLastCommitHash checks if the last commit hash for a given branch has changed in the remote repository.
// It fetches the reference list from the remote repository and compares the commit hashes.
//   - If the hash has not changed, it returns the same commit hash.
//   - If it has changed, it returns the updated commit hash.
//   - If an error occurs while fetching the reference list, an error is returned.
func (r *Repository) getLastCommitHash(branch string, commitHASH plumbing.Hash) (plumbing.Hash, error) {
	var lastCommitHASH plumbing.Hash
	// get remote repository
	remote, err := r.localGit.Remote(r.cloneOpts.RemoteName)
	if err != nil {
		return commitHASH, err
	}
	// list all remote repository references
	references, err := remote.List(r.listOpts)
	if err != nil {
		return commitHASH, err
	}
	// Iterate and compare current commit Hash with remote commit Hashes
	for _, ref := range references {
		if ref.Name().IsBranch() && ref.Name().Short() == branch {
			// lastCommit has not changed
			if commitHASH == ref.Hash() {
				return commitHASH, nil
			}
			// lastCommit changed
			lastCommitHASH = ref.Hash()
		}
	}

	return lastCommitHASH, nil
}
