package git

import (
	"fmt"
	"net/url"
	"os"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/settings"
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

// extendedRepo embeds repository interface defining git-related operations.
// This is needed in order to generate mocks for unit-tests
type extendedRepo struct {
	repository
}

// repoOperation holds the config of a git repository and the repo instance.
type repoOperation struct {
	config       *git
	localRepo    *gogit.Repository
	auth         transport.AuthMethod // interface
	cloneOpts    *gogit.CloneOptions
	fetchOpts    *gogit.FetchOptions
	checkoutOpts *gogit.CheckoutOptions
	listOpts     *gogit.ListOptions
	resetOpts    *gogit.ResetOptions
}

type git struct {
	URL               string
	Directory         string
	username          string
	password          string
	agent             *agent.Agent
	caBundle          []byte
	insecureTLSVerify bool
	secret            *corev1.Secret
	headers           map[string]string
	knownHosts        []byte
}

// gitForRepo constructs and returns a new git object for the given repository.
// It requires a secret for authentication, the namespace, name of the repository,
// the gitURL, a flag indicating if TLS verification should be skipped, and a CA bundle for SSL.
// If the Git URL uses the SSH protocol, it checks if the URL is a valid SSH URL.
// If the Git URL uses HTTP or HTTPS, it parses and verifies the URL.
// It then constructs a directory path for the git repository.
// It also sets an X-Install-Uuid header if an installation UUID is available.
// If a CA bundle is provided, it converts the CA bundle from DER to PEM format, since Git requires PEM format.
// In this case, insecureSkipTLS is set to false since a CA bundle is provided for secure communication.
// Finally, it returns a new git object configured with these settings,
// or an error if any step in this process fails.
func gitForRepo(secret *corev1.Secret, namespace, name, gitURL string, insecureSkipTLS bool, caBundle []byte) (*extendedRepo, error) {
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
			return nil, fmt.Errorf("invalid git URL scheme %s, only http(s) supported", u.Scheme)
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

	config := &git{
		URL:               gitURL,
		Directory:         dir,
		caBundle:          caBundle,
		insecureTLSVerify: insecureSkipTLS,
		secret:            secret,
		headers:           headers,
	}

	repoOperation, err := newRepoOperation(config)
	if err != nil {
		return nil, err
	}

	git := newRepository(&extendedRepo{repoOperation})

	return git, nil
}

// newRepoOperation is a constructor for the repoOperation struct.
func newRepoOperation(config *git) (repository, error) {
	ro := &repoOperation{
		config:       config,
		localRepo:    &gogit.Repository{},
		cloneOpts:    &gogit.CloneOptions{},
		checkoutOpts: &gogit.CheckoutOptions{},
		fetchOpts:    &gogit.FetchOptions{},
		listOpts:     &gogit.ListOptions{},
		resetOpts:    &gogit.ResetOptions{},
	}
	// credentials must be setted before options
	err := ro.setRepoCredentials()
	if err != nil {
		return ro, err
	}
	ro.setRepoOptions()

	return ro, err
}

// newRepository is a constructor for the extendedRepo struct
// which embeds the gogit interacting methods
func newRepository(g *extendedRepo) *extendedRepo {
	return &extendedRepo{g}
}

// setRepoCredentials detects which type of authentication and communication protocol
// and configurates the git repo communications accordingly
func (ro *repoOperation) setRepoCredentials() error {
	if ro.config.secret == nil {
		return nil
	}

	var username string
	var password string

	switch ro.config.secret.Type {
	case corev1.SecretTypeBasicAuth: // BASIC HTTP(S) AUTHENTICATION
		// get the credentials setted in kubernetes
		username = string(ro.config.secret.Data[corev1.BasicAuthUsernameKey])
		password = string(ro.config.secret.Data[corev1.BasicAuthPasswordKey])
		if len(password) == 0 || len(username) == 0 {
			return fmt.Errorf("username and password not provided")
		}
		// BasicAuth implements transport.AuthMethod interface
		ro.auth = &plumbingHTTP.BasicAuth{
			Username: username,
			Password: password,
		}
		return nil

	case corev1.SecretTypeSSHAuth: // SSH AUTHENTICATION
		// Make signer from private keys
		pemBytes := ro.config.secret.Data[corev1.SSHAuthPrivateKey]
		signer, err := ssh.ParsePrivateKey(pemBytes)
		if err != nil {
			return fmt.Errorf("failed to parse ssh private key: %w", err)
		}

		// PublicKeys implements transport.AuthMethod interface
		ro.auth = &plumbingSSH.PublicKeys{
			User:   "git",
			Signer: signer,
		}

		// Create temporary known_hosts file
		hostsBytes := ro.config.secret.Data["known_hosts"]
		if len(hostsBytes) > 0 {
			f, err := os.CreateTemp("", "known_hosts")
			if err != nil {
				return fmt.Errorf("failed to create temporar known_hosts file: %w", err)
			}
			// Write received knownhosts from kubernetes secret
			_, err = f.Write(hostsBytes)
			if err != nil {
				return fmt.Errorf("failed to write knonw_hosts file: %w", err)
			}
			// Create callback from recently created temporary file
			hostKeyCB, err := plumbingSSH.NewKnownHostsCallback(f.Name())
			if err != nil {
				return err
			}

			ro.auth = &plumbingSSH.PublicKeys{
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
func (ro *repoOperation) setRepoOptions() {
	// Clone Options
	ro.cloneOpts.URL = ro.config.URL
	ro.cloneOpts.Depth = 1
	ro.cloneOpts.InsecureSkipTLS = ro.config.insecureTLSVerify
	ro.cloneOpts.Tags = gogit.NoTags

	// Fetch Options
	ro.fetchOpts.RemoteURL = ro.config.URL
	ro.fetchOpts.Depth = 1
	ro.fetchOpts.Force = true
	ro.fetchOpts.InsecureSkipTLS = ro.config.insecureTLSVerify
	ro.fetchOpts.Tags = gogit.NoTags

	// Checkout Options
	ro.checkoutOpts.Force = true
	ro.checkoutOpts.Create = true

	// List Options
	ro.listOpts.InsecureSkipTLS = ro.config.insecureTLSVerify

	// Reset Options
	ro.resetOpts.Mode = gogit.HardReset

	// Set authentication Methods for cloning
	if ro.auth != nil {
		ro.cloneOpts.Auth = ro.auth
		ro.fetchOpts.Auth = ro.auth
		ro.listOpts.Auth = ro.auth
	}
	// Set CABundle for all git operations
	if len(ro.config.caBundle) > 0 {
		ro.cloneOpts.CABundle = ro.config.caBundle
		ro.fetchOpts.CABundle = ro.config.caBundle
		ro.listOpts.CABundle = ro.config.caBundle
	}
	// Debug if enabled
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		ro.cloneOpts.Progress = os.Stdout
		ro.fetchOpts.Progress = os.Stdout
	}
}

// repository defines functions to interact with git repositories through gogit package
type repository interface {
	GetConfig() *git
	GetAuth() transport.AuthMethod
	cloneOrOpen(branch string) error
	plainOpen() error
	getCurrentCommit() (plumbing.Hash, error)
	fetchAndReset(branch string) error
	updateRefSpec(branch string)
	fetch(branch string) error
	checkout(branch plumbing.ReferenceName) (plumbing.Hash, error)
	hardReset(reference string) error
	getLastCommitHash(branch string, commitHASH plumbing.Hash) (plumbing.Hash, error)
}

// GetConfif just retrieves the current git configuration
func (ro *repoOperation) GetConfig() *git {
	return ro.config
}

// GetAuth just returns the authentication method configured,
// this method is intended for unit-tests only
func (ro *repoOperation) GetAuth() transport.AuthMethod {
	return ro.auth
}

// cloneOrOpen executes the clone operation of a git repository at given branch with depth = 1 if it does not exist.
// If if exists, just open the local repository and assign it to ro.localRepo
func (ro *repoOperation) cloneOrOpen(branch string) error {
	var err error
	cloneOptions := ro.cloneOpts
	if branch != "" {
		cloneOptions.ReferenceName = plumbing.NewBranchReferenceName(branch)
	}

	err = cloneOptions.Validate()
	if err != nil {
		return fmt.Errorf("plainClone validation failure: %w", err)
	}

	err = ro.plainOpen()

	if err == gogit.ErrRepositoryNotExists {
		ro.localRepo, err = gogit.PlainClone(ro.config.Directory, false, cloneOptions)
		if err != nil && err != gogit.ErrRepositoryAlreadyExists {
			return fmt.Errorf("plainClone failure: %w", err)
		}
		return nil
	}

	return err
}

// plainOpen opens an existing local git repository on the specified folder not walking parent directories looking for '.git/'
func (ro *repoOperation) plainOpen() error {
	openOptions := gogit.PlainOpenOptions{
		DetectDotGit: false,
	}
	localRepository, err := gogit.PlainOpenWithOptions(ro.config.Directory, &openOptions)
	if err != nil {
		return err
	}

	ro.localRepo = localRepository
	return nil
}

// getCurrentCommit returns the commit hash of the HEAD from the current branch
func (ro *repoOperation) getCurrentCommit() (plumbing.Hash, error) {
	headRef, err := ro.localRepo.Head()
	if err != nil {
		return plumbing.Hash{}, fmt.Errorf("getCurrentCommit failure: %w", err)
	}

	return headRef.Hash(), nil
}

// fetchAndReset is a convenience method that fetches updates from the remote repository
// for a specific branch, and then resets the current branch to a specified commit.
func (ro *repoOperation) fetchAndReset(branch string) error {
	if err := ro.fetch(branch); err != nil {
		return fmt.Errorf("fetchAndReset failure: %w", err)
	}
	// before: g.reset("FETCH_HEAD")
	return ro.hardReset(branch)
}

// updateRefSpec updates the reference specification (RefSpec) in the fetch options
// of the repository operation.
//   - If a branch name is provided, it sets the RefSpec to fetch that specific branch.
//   - Otherwise, it sets the RefSpec to fetch all branches.
//
// fetching the last commit of one branch is faster than fetching from all branches.
func (ro *repoOperation) updateRefSpec(branch string) {
	var newRefSpec string

	if branch != "" {
		newRefSpec = fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", branch, branch)
	} else {
		newRefSpec = "+refs/heads/*:refs/remotes/origin/*"
	}

	if len(ro.fetchOpts.RefSpecs) > 0 {
		ro.fetchOpts.RefSpecs[0] = config.RefSpec(newRefSpec)
	} else {
		ro.fetchOpts.RefSpecs = []config.RefSpec{config.RefSpec(newRefSpec)}
	}
}

// fetch fetches updates from the remote repository for a specific branch.
// If the fetch operation is already up-to-date, this is not treated as an error.
// Any other error that occurs during fetch is returned.
func (ro *repoOperation) fetch(branch string) error {
	ro.updateRefSpec(branch)
	fetchOptions := ro.fetchOpts

	err := ro.localRepo.Fetch(fetchOptions)
	if err != nil && err != gogit.NoErrAlreadyUpToDate {
		return fmt.Errorf("fetch failure: %w", err)
	}

	return nil
}

func (ro *repoOperation) checkout(branch plumbing.ReferenceName) (plumbing.Hash, error) {

	checkOpts := gogit.CheckoutOptions{
		Branch: branch,
	}

	_, err := ro.localRepo.Branch(branch.Short())
	switch {
	case err == gogit.ErrBranchExists:
		checkOpts.Force = true
	case err == gogit.ErrBranchNotFound:
		checkOpts.Create = true
	case err != gogit.ErrBranchExists && err != gogit.ErrBranchNotFound && err != nil:
		return plumbing.ZeroHash, fmt.Errorf("checkout failure to check branch: %w", err)
	}

	wt, err := ro.localRepo.Worktree()
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("checkout failure to open worktree: %w", err)
	}

	err = wt.Checkout(&checkOpts)
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("checkout failure: %w", err)
	}

	return ro.getCurrentCommit()
}

// hardReset performs a hard reset of the git repository to a specific commit.
func (ro *repoOperation) hardReset(reference string) error {
	var err error
	resetOpts := ro.resetOpts

	switch {
	case isLocalBranch(reference):
		branchRef, err := ro.localRepo.Reference(plumbing.ReferenceName(reference), true)
		if err != nil {
			return fmt.Errorf("branch does not exist locally: %w", err)
		}
		resetOpts.Commit = branchRef.Hash()
	case reference == "HEAD":
		commitHash, err := ro.getCurrentCommit()
		if err != nil {
			return fmt.Errorf("hardReset failure to get current commit: %w", err)
		}
		resetOpts.Commit = commitHash
	// FETCH_HEAD
	default:
		branchRef, err := ro.localRepo.Reference(plumbing.NewRemoteReferenceName("origin", reference), false)
		// branchRef, err := ro.localRepo.Reference(plumbing.ReferenceName(reference), false)
		if err != nil {
			return fmt.Errorf("hardReset failure to get branch reference: %w", err)
		}
		resetOpts.Commit = branchRef.Hash()
	}

	// Validate hashCommit and reset options
	err = resetOpts.Validate(ro.localRepo)
	if err != nil {
		return fmt.Errorf("hardReset validation failure: %w", err)
	}

	// Open new worktree
	wt, err := ro.localRepo.Worktree()
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
func (ro *repoOperation) getLastCommitHash(branch string, commitHASH plumbing.Hash) (plumbing.Hash, error) {

	var lastCommitHASH plumbing.Hash

	remote, err := ro.localRepo.Remote(ro.cloneOpts.RemoteName)
	if err != nil {
		return commitHASH, err
	}

	references, err := remote.List(ro.listOpts)
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
