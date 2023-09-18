package git

import (
	"fmt"
	"net/url"
	"os"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	corev1 "k8s.io/api/core/v1"

	// gogit packages
	gogit "github.com/go-git/go-git/v5"
	config "github.com/go-git/go-git/v5/config"
	plumbing "github.com/go-git/go-git/v5/plumbing"
	transport "github.com/go-git/go-git/v5/plumbing/transport"
	plumbingHTTP "github.com/go-git/go-git/v5/plumbing/transport/http"
	plumbingSSH "github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// repository holds the config of a git repository and the repo instance.
type Repository struct {
	URL               string
	Directory         string
	username          string
	password          string
	agent             *agent.Agent
	caBundle          []byte
	insecureTLSVerify bool
	secret            *corev1.Secret
	knownHosts        []byte
	repoGogit         *gogit.Repository
	auth              transport.AuthMethod
	cloneOpts         *gogit.CloneOptions
	fetchOpts         *gogit.FetchOptions
	listOpts          *gogit.ListOptions
	resetOpts         *gogit.ResetOptions
}

// BuildRepoConfig constructs and returns a new repository object for the given repository.
// It requires a secret for authentication, the namespace, the name of the repository,
// the gitURL, a flag indicating if TLS verification should be skipped, and a CA bundle for SSL.
// If the Git URL uses the SSH protocol, it checks if the URL is a valid SSH URL.
// If the Git URL uses HTTP or HTTPS, it parses and verifies the URL.
// It then constructs a directory path for the git repository.
// If a CA bundle is provided, it converts the CA bundle from DER to PEM format, since Git requires PEM format.
// In this case, insecureSkipTLS is set to false since a CA bundle is provided for secure communication.
// Finally, it returns a new git object configured with these settings,
// or an error if any step in this process fails.
func BuildRepoConfig(secret *corev1.Secret, namespace, name, gitURL string, insecureSkipTLS bool, caBundle []byte) (*Repository, error) {

	isGitSSH, err := isGitSSH(gitURL)
	if err != nil {
		logrus.Error(fmt.Errorf("failed to verify the type of URL %s: %w", gitURL, err))
	}
	if !isGitSSH {
		u, err := url.Parse(gitURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse URL %s: %w", gitURL, err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return nil, fmt.Errorf("invalid git URL scheme %s, only http(s) supported", u.Scheme)
		}
	}
	dir := gitDir(namespace, name, gitURL)

	// convert caBundle to PEM format because git requires correct line breaks, header and footer.
	if len(caBundle) > 0 {
		caBundle = convertDERToPEM(caBundle)
		insecureSkipTLS = false
	}

	repo := &Repository{
		URL:               gitURL,
		Directory:         dir,
		caBundle:          caBundle,
		insecureTLSVerify: insecureSkipTLS,
		secret:            secret,
		repoGogit:         &gogit.Repository{},
		cloneOpts:         &gogit.CloneOptions{},
		fetchOpts:         &gogit.FetchOptions{},
		listOpts:          &gogit.ListOptions{},
		resetOpts:         &gogit.ResetOptions{},
	}

	// credentials must be set before options
	err = repo.setRepoCredentials()
	if err != nil {
		return repo, err
	}
	repo.setRepoOptions()

	return repo, nil
}

// setRepoCredentials detects which type of authentication and communication protocol
// and configurates the git repo communications accordingly.
func (r *Repository) setRepoCredentials() error {
	if r.secret == nil {
		return nil
	}

	switch r.secret.Type {
	case corev1.SecretTypeBasicAuth: // BASIC HTTP(S) AUTHENTICATION
		// get the credentials set in kubernetes
		username := string(r.secret.Data[corev1.BasicAuthUsernameKey])
		password := string(r.secret.Data[corev1.BasicAuthPasswordKey])
		if len(password) == 0 || len(username) == 0 {
			return fmt.Errorf("username or password not provided")
		}
		// BasicAuth implements transport.AuthMethod interface
		r.auth = &plumbingHTTP.BasicAuth{
			Username: username,
			Password: password,
		}
		return nil

	case corev1.SecretTypeSSHAuth: // SSH AUTHENTICATION
		// Make signer from private keys
		pemBytes := r.secret.Data[corev1.SSHAuthPrivateKey]
		signer, err := ssh.ParsePrivateKey(pemBytes)
		if err != nil {
			return fmt.Errorf("failed to parse ssh private key: %w", err)
		}

		// PublicKeys implements transport.AuthMethod interface
		r.auth = &plumbingSSH.PublicKeys{
			User:   "git",
			Signer: signer,
		}

		// Create temporary known_hosts file
		hostsBytes := r.secret.Data["known_hosts"]
		if len(hostsBytes) > 0 {
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
			hostKeyCB, err := plumbingSSH.NewKnownHostsCallback(f.Name())
			if err != nil {
				return fmt.Errorf("setRepoCredentials at known hosts failure: %w", err)
			}

			r.auth = &plumbingSSH.PublicKeys{
				User:                  "git",
				Signer:                signer,
				HostKeyCallbackHelper: plumbingSSH.HostKeyCallbackHelper{HostKeyCallback: hostKeyCB},
			}

			// Clean known_hosts file after setting up the callback
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

// setRepoOptions assigns the options configured before in credentials.
// hard-code other needed configurations like Depth for faster cloning.
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

// cloneOrOpen executes the clone operation of a git repository at given branch with depth = 1 if it does not exist.
// If exists, just open the local repository and assign it to ro.localRepo
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
		repoGogit, cloneErr := gogit.PlainClone(r.Directory, false, cloneOptions)
		if cloneErr != nil && cloneErr != gogit.ErrRepositoryAlreadyExists {
			return fmt.Errorf("plainClone failure: %w", cloneErr)
		}
		// serious problem warning
		if openErr == gogit.ErrRepositoryNotExists && cloneErr == gogit.ErrRepositoryAlreadyExists {
			return fmt.Errorf("serious failure, neither open or clone succeeded: %w", cloneErr)
		}
		r.repoGogit = repoGogit
		return nil
	}

	return nil
}

// plainOpen opens an existing local git repository on the specified folder not walking parent directories looking for '.git/'
func (r *Repository) plainOpen() error {
	openOptions := gogit.PlainOpenOptions{
		DetectDotGit: false,
	}
	localRepository, err := gogit.PlainOpenWithOptions(r.Directory, &openOptions)
	if err != nil {
		return err
	}

	r.repoGogit = localRepository
	return nil
}

// getCurrentCommit returns the commit hash of the HEAD from the current branch
func (r *Repository) getCurrentCommit() (plumbing.Hash, error) {
	headRef, err := r.repoGogit.Head()
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("getCurrentCommit failure: %w", err)
	}

	return headRef.Hash(), nil
}

// fetchCheckoutAndReset is a convenience method that fetches updates from the remote repository
// for a specific branch, and then resets the current branch to a specified commit.
func (r *Repository) fetchCheckoutAndReset(branch string) error {
	if err := r.fetch(branch); err != nil {
		return fmt.Errorf("fetchCheckoutAndReset failure: %w", err)
	}

	err := r.checkout(branch)
	if err != nil {
		return fmt.Errorf("checkout failure: %w", err)
	}

	return r.hardReset(branch)
}

// updateRefSpec updates the reference specification (RefSpec) in the fetch options
// of the repository operation.
//   - If a branch name is provided, it sets the RefSpec to fetch that specific branch.
//   - Otherwise, it sets the RefSpec to fetch all branches.
//
// fetching the last commit of one branch is faster than fetching from all branches.
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
// If the fetch operation is already up-to-date, this is not treated as an error.
// Any other error that occurs during fetch is returned.
func (r *Repository) fetch(branch string) error {
	r.updateRefSpec(branch)
	fetchOptions := r.fetchOpts

	err := r.repoGogit.Fetch(fetchOptions)
	if err != nil && err != gogit.NoErrAlreadyUpToDate && err != transport.ErrEmptyUploadPackRequest {
		return fmt.Errorf("fetch failure: %w", err)
	}

	return nil
}

// hardReset performs a hard reset of the git repository to a specific commit or branch
func (r *Repository) hardReset(reference string) error {
	var err error
	resetOpts := r.resetOpts

	switch {
	case isLocalBranch(reference):
		branchRef, err := r.repoGogit.Reference(plumbing.ReferenceName(reference), true)
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
		branchRef, err := r.repoGogit.Reference(plumbing.NewRemoteReferenceName("origin", reference), false)
		if err != nil {
			return fmt.Errorf("hardReset failure to get branch reference: %w", err)
		}
		resetOpts.Commit = branchRef.Hash()
	}

	// Validate hashCommit and reset options
	err = resetOpts.Validate(r.repoGogit)
	if err != nil {
		return fmt.Errorf("hardReset validation failure: %w", err)
	}

	// Open new worktree
	wt, err := r.repoGogit.Worktree()
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

	remote, err := r.repoGogit.Remote(r.cloneOpts.RemoteName)
	if err != nil {
		return commitHASH, err
	}

	references, err := remote.List(r.listOpts)
	if err != nil {
		return commitHASH, err
	}

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

func (r *Repository) checkout(branch string) error {
	branchRef := plumbing.NewBranchReferenceName(branch)
	checkOpts := gogit.CheckoutOptions{
		Branch: branchRef,
	}

	_, err := r.repoGogit.Reference(branchRef, false)
	switch {
	case err == nil:
		// local branch exists
		checkOpts.Force = true
		checkOpts.Create = false
	case err == plumbing.ErrReferenceNotFound:
		checkOpts.Create = true
	case err != plumbing.ErrReferenceNotFound && err != nil:
		return fmt.Errorf("checkout failure to check branch: %w", err)
	}

	err = checkOpts.Validate()
	if err != nil {
		return fmt.Errorf("checkout options validation failure: %w", err)
	}

	wt, err := r.repoGogit.Worktree()
	if err != nil {
		return fmt.Errorf("checkout failure to open worktree: %w", err)
	}

	err = wt.Checkout(&checkOpts)
	if err != nil {
		return fmt.Errorf("checkout failure: %w", err)
	}

	return nil
}
