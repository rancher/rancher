// Package git provides go-git based implementation of gitClient interface.
//
// WORKAROUND: This implementation includes safeFetch() to work around go-git#845
// (https://github.com/go-git/go-git/issues/845), a bug where shallow fetch
// followed by hard reset can produce missing objects. This workaround can be
// removed once we upgrade to go-git v6 stable (post-v6.0.0-alpha.5) which
// includes the fix from PR #1884.
package git

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/catalogv2/roundtripper"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	corev1 "k8s.io/api/core/v1"
)

var (
	// httpClientMutex protects concurrent InstallProtocol/restore operations
	httpClientMutex sync.Mutex
)

type gitGo struct {
	commonGitFields

	// go-git specific fields
	auth             transport.AuthMethod
	customHTTPClient *http.Client
}

var _ gitClient = (*gitGo)(nil)

// newGitGo creates a new go-git based gitClient
func newGitGo(directory, url string, opts *Options) (*gitGo, error) {
	if opts == nil {
		opts = &Options{}
	}

	g := &gitGo{
		commonGitFields: commonGitFields{
			URL:               url,
			Directory:         directory,
			caBundle:          opts.CABundle,
			insecureTLSVerify: opts.InsecureTLSVerify,
			secret:            opts.Credential,
			headers:           opts.Headers,
		},
	}

	if err := g.setCredential(opts.Credential); err != nil {
		return nil, err
	}

	// Build custom HTTP client if needed for TLS/CA bundle
	if (opts.Credential != nil && opts.Credential.Type == corev1.SecretTypeTLS) || len(opts.CABundle) > 0 || len(opts.Headers) > 0 {
		client, err := g.httpClientWithCreds()
		if err != nil {
			return nil, err
		}
		g.customHTTPClient = client
	}

	return g, nil
}

// setCredential configures authentication for go-git transport
func (g *gitGo) setCredential(cred *corev1.Secret) error {
	if cred == nil {
		return nil
	}

	switch cred.Type {
	case corev1.SecretTypeBasicAuth:
		username, password := cred.Data[corev1.BasicAuthUsernameKey], cred.Data[corev1.BasicAuthPasswordKey]
		if len(password) == 0 && len(username) == 0 {
			return nil
		}

		// Build BasicAuth for go-git
		g.auth = &githttp.BasicAuth{
			Username: string(username),
			Password: string(password),
		}
		g.password = string(password)

	case corev1.SecretTypeSSHAuth:
		// Use go-git's SSH auth instead of agent socket
		publicKeys, err := gitssh.NewPublicKeys(
			"git",
			cred.Data[corev1.SSHAuthPrivateKey],
			"", // no passphrase
		)
		if err != nil {
			return err
		}

		// Handle known_hosts
		g.knownHosts = cred.Data["known_hosts"]
		if len(g.knownHosts) > 0 {
			// Write to temp file for knownhosts.New()
			tmpFile, err := os.CreateTemp("", "known_hosts-*")
			if err != nil {
				return err
			}
			tmpPath := tmpFile.Name()
			if _, err := tmpFile.Write(g.knownHosts); err != nil {
				tmpFile.Close()
				os.Remove(tmpPath)
				return err
			}
			tmpFile.Close()

			callback, err := knownhosts.New(tmpPath)
			if err != nil {
				os.Remove(tmpPath)
				return err
			}
			os.Remove(tmpPath) // Clean up immediately after parsing
			publicKeys.HostKeyCallback = callback
		} else {
			publicKeys.HostKeyCallback = ssh.InsecureIgnoreHostKey()
		}

		g.auth = publicKeys

	case corev1.SecretTypeTLS:
		// TLS is handled via custom HTTP client, not auth
		// Build custom HTTP client in newGitGo()
	}

	return nil
}

// httpClientWithCreds creates an HTTP client with custom TLS config and headers
func (g *gitGo) httpClientWithCreds() (*http.Client, error) {
	var (
		username  string
		password  string
		tlsConfig tls.Config
	)

	if g.secret != nil {
		switch g.secret.Type {
		case corev1.SecretTypeBasicAuth:
			username = string(g.secret.Data[corev1.BasicAuthUsernameKey])
			password = string(g.secret.Data[corev1.BasicAuthPasswordKey])
		case corev1.SecretTypeTLS:
			cert, err := tls.X509KeyPair(g.secret.Data[corev1.TLSCertKey], g.secret.Data[corev1.TLSPrivateKeyKey])
			if err != nil {
				return nil, err
			}
			tlsConfig.Certificates = append(tlsConfig.Certificates, cert)
		}
	}

	if len(g.caBundle) > 0 {
		cert, err := x509.ParseCertificate(g.caBundle)
		if err != nil {
			return nil, err
		}
		pool, err := x509.SystemCertPool()
		if err != nil {
			logrus.Debugf("getting system cert pool failed with %s", err)
			pool = x509.NewCertPool()
		}
		pool.AddCert(cert)
		tlsConfig.RootCAs = pool
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tlsConfig
	transport.TLSClientConfig.InsecureSkipVerify = g.insecureTLSVerify
	transport.Proxy = http.ProxyFromEnvironment

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	// Wrap the transport with a custom RoundTripper to set the User-Agent header
	client.Transport = &roundtripper.UserAgent{
		UserAgent: roundtripper.BuildUserAgent("go-git", "(Git-based Helm Repository)"),
		Next:      client.Transport,
	}

	// Wrap with custom headers (X-Install-Uuid, etc.)
	if len(g.headers) > 0 {
		client.Transport = &headerRoundTripper{
			headers: g.headers,
			next:    client.Transport,
		}
	}

	if username != "" || password != "" {
		client.Transport = &basicRoundTripper{
			username: username,
			password: password,
			next:     client.Transport,
		}
	}

	return client, nil
}

// headerRoundTripper wraps an http.RoundTripper to inject custom headers
type headerRoundTripper struct {
	headers map[string]string
	next    http.RoundTripper
}

func (h *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}
	return h.next.RoundTrip(req)
}

// directory returns the repository directory path
func (g *gitGo) directory() string {
	return g.Directory
}

// Clone performs a shallow clone of the repository
func (g *gitGo) Clone(branch string) error {
	cloneOpts := &gogit.CloneOptions{
		URL:        g.URL,
		Auth:       g.auth,
		Depth:      1,    // Shallow clone
		NoCheckout: true, // -n flag equivalent
		Progress:   g.getProgress(),
	}

	if branch != "" {
		cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(branch)
		cloneOpts.SingleBranch = true
	}

	// Use custom HTTP client if we have TLS/CA bundle
	// Protected by mutex to avoid race with concurrent calls
	if g.customHTTPClient != nil {
		httpClientMutex.Lock()
		defer httpClientMutex.Unlock()

		customClient := githttp.NewClient(g.customHTTPClient)
		client.InstallProtocol("https", customClient)
		defer client.InstallProtocol("https", githttp.DefaultClient)

		_, err := gogit.PlainClone(g.Directory, false, cloneOpts)
		return err
	}

	_, err := gogit.PlainClone(g.Directory, false, cloneOpts)
	return err
}

func (g *gitGo) getProgress() io.Writer {
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		return os.Stdout
	}
	return nil
}

// clone ensures the repository is cloned (idempotent)
func (g *gitGo) clone(branch string) error {
	gitDir := filepath.Join(g.Directory, ".git")
	if dir, err := os.Stat(gitDir); err == nil && dir.IsDir() {
		return nil
	}

	if err := os.RemoveAll(g.Directory); err != nil {
		return fmt.Errorf("failed to remove directory %s: %v", g.Directory, err)
	}

	return g.Clone(branch)
}

// Update updates git repo if remote sha has changed
func (g *gitGo) Update(branch string) (string, error) {
	if err := g.clone(branch); err != nil {
		return "", err
	}

	if err := g.reset("HEAD"); err != nil {
		return "", err
	}

	commit, err := g.currentCommit()
	if err != nil {
		return commit, err
	}

	if changed, err := g.remoteSHAChanged(branch, commit); err != nil || !changed {
		return commit, err
	}

	if err := g.fetchAndReset(branch); err != nil {
		return "", err
	}

	return g.currentCommit()
}

// fetchAndReset fetches a specific ref and resets to it
func (g *gitGo) fetchAndReset(rev string) error {
	repo, err := gogit.PlainOpen(g.Directory)
	if err != nil {
		return err
	}

	// Determine if rev is a commit hash or a branch/tag
	isCommitHash := len(rev) == 40 && isHexString(rev)

	fetchOpts := &gogit.FetchOptions{
		RemoteName: "origin",
		Auth:       g.auth,
		Progress:   g.getProgress(),
		// Depth will be set by doFetchWithVerify
	}

	// If rev is a commit hash, we can't fetch it directly (servers don't allow it)
	// Instead, fetch the default branch with deeper history to try to find the commit
	if isCommitHash {
		// Check if commit already exists locally
		hash := plumbing.NewHash(rev)
		if _, err := repo.CommitObject(hash); err == nil {
			// Commit exists locally, just reset to it
			return g.reset(rev)
		}

		// Commit not found locally - fetch with deeper history
		// We fetch the default branch (no refspec = all refs) with depth to find the commit
		fetchOpts.Depth = 100 // Reasonable default for finding recent commits
	} else if rev != "" {
		// Branch/tag name - fetch it specifically
		fetchOpts.RefSpecs = []config.RefSpec{
			config.RefSpec(fmt.Sprintf("+%s:%s", rev, rev)),
		}
	}

	// Use custom HTTP client if we have TLS/CA bundle
	// Protected by mutex to avoid race with concurrent calls
	if g.customHTTPClient != nil {
		httpClientMutex.Lock()
		defer httpClientMutex.Unlock()

		customClient := githttp.NewClient(g.customHTTPClient)
		client.InstallProtocol("https", customClient)
		defer client.InstallProtocol("https", githttp.DefaultClient)

		if err := g.doFetchWithVerify(repo, fetchOpts, rev); err != nil {
			return err
		}
	} else {
		if err := g.doFetchWithVerify(repo, fetchOpts, rev); err != nil {
			return err
		}
	}

	// After fetch, reset to the branch we just fetched
	// If rev is empty, we fetched the default branch, so reset to HEAD
	// go-git doesn't create FETCH_HEAD, so we resolve the ref directly
	if rev == "" {
		return g.reset("HEAD")
	}
	return g.reset(rev)
}

// isHexString checks if a string contains only lowercase hexadecimal characters (git SHAs are lowercase)
func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// doFetchWithVerify performs fetch with completeness verification (safeFetch workaround for go-git#845)
func (g *gitGo) doFetchWithVerify(repo *gogit.Repository, fetchOpts *gogit.FetchOptions, targetRev string) error {
	// Determine if targetRev is a commit hash
	isCommitHash := len(targetRev) == 40 && isHexString(targetRev)

	// For commit hashes, we might need deeper history
	// Start with depth 50 as a reasonable default for finding recent commits
	if isCommitHash {
		fetchOpts.Depth = 50
	} else {
		fetchOpts.Depth = 1
	}

	// First fetch attempt
	err := repo.Fetch(fetchOpts)
	if err != nil && !errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		return fmt.Errorf("fetch: %w", err)
	}

	// If targetRev is empty, we're fetching the default branch - no verification needed
	if targetRev == "" {
		return nil
	}

	// Resolve the target hash
	var targetHash plumbing.Hash
	if isCommitHash {
		targetHash = plumbing.NewHash(targetRev)
	} else {
		// Resolve branch/tag to hash
		ref, err := repo.Reference(plumbing.NewBranchReferenceName(targetRev), true)
		if err != nil {
			// Try as remote branch
			ref, err = repo.Reference(plumbing.NewRemoteReferenceName("origin", targetRev), true)
			if err != nil {
				return fmt.Errorf("resolving ref %s: %w", targetRev, err)
			}
		}
		targetHash = ref.Hash()
	}

	// Verify we have complete tree (safeFetch workaround for go-git#845)
	complete, err := hasCompleteTree(repo, targetHash)
	if err != nil {
		return fmt.Errorf("verifying fetched objects: %w", err)
	}
	if !complete {
		// Recovery fetch with much larger depth
		retry := *fetchOpts
		retry.Depth = 1 << 20 // force the server to stop withholding objects
		err := repo.Fetch(&retry)
		if err != nil && !errors.Is(err, gogit.NoErrAlreadyUpToDate) {
			return fmt.Errorf("recovery fetch: %w", err)
		}

		complete, err = hasCompleteTree(repo, targetHash)
		if err != nil {
			return fmt.Errorf("verifying fetched objects after recovery: %w", err)
		}
		if !complete {
			return fmt.Errorf("commit %s is still incomplete after recovery fetch (see go-git#845)", targetHash)
		}
	}

	return nil
}

// hasCompleteTree reports whether commit and every blob reachable from its
// tree is present in the local object store.
func hasCompleteTree(repo *gogit.Repository, commit plumbing.Hash) (bool, error) {
	c, err := repo.CommitObject(commit)
	if err != nil {
		if errors.Is(err, plumbing.ErrObjectNotFound) {
			return false, nil
		}
		return false, err
	}

	tree, err := c.Tree()
	if err != nil {
		if errors.Is(err, plumbing.ErrObjectNotFound) {
			return false, nil
		}
		return false, err
	}

	files := tree.Files()
	defer files.Close()

	for {
		f, err := files.Next()
		if errors.Is(err, io.EOF) {
			return true, nil
		}
		if err != nil {
			if errors.Is(err, plumbing.ErrObjectNotFound) {
				return false, nil
			}
			return false, err
		}

		if _, err := repo.BlobObject(f.Hash); err != nil {
			if errors.Is(err, plumbing.ErrObjectNotFound) {
				return false, nil
			}
			return false, err
		}
	}
}

// reset performs a hard reset to a specific revision
func (g *gitGo) reset(rev string) error {
	repo, err := gogit.PlainOpen(g.Directory)
	if err != nil {
		return err
	}

	w, err := repo.Worktree()
	if err != nil {
		return err
	}

	// Resolve revision to hash
	var hash plumbing.Hash
	if rev == "HEAD" {
		head, err := repo.Head()
		if err != nil {
			return err
		}
		hash = head.Hash()
	} else if len(rev) == 40 && isHexString(rev) {
		// Commit hash - verify it exists
		hash = plumbing.NewHash(rev)
		_, err := repo.CommitObject(hash)
		if err != nil {
			return fmt.Errorf("commit %s not found: %w", rev, err)
		}
	} else {
		// Try as branch name
		ref, err := repo.Reference(plumbing.NewBranchReferenceName(rev), true)
		if err != nil {
			// Try as remote branch
			ref, err = repo.Reference(plumbing.NewRemoteReferenceName("origin", rev), true)
			if err != nil {
				return fmt.Errorf("resolving ref %s: %w", rev, err)
			}
		}
		hash = ref.Hash()
	}

	// Hard reset
	err = w.Reset(&gogit.ResetOptions{
		Commit: hash,
		Mode:   gogit.HardReset,
	})
	if err != nil {
		return err
	}

	// Update HEAD to point to the commit
	return repo.Storer.SetReference(plumbing.NewHashReference(plumbing.HEAD, hash))
}

// currentCommit returns the current HEAD commit SHA
func (g *gitGo) currentCommit() (string, error) {
	repo, err := gogit.PlainOpen(g.Directory)
	if err != nil {
		return "", err
	}

	head, err := repo.Head()
	if err != nil {
		return "", err
	}

	return head.Hash().String(), nil
}

// remoteSHAChanged checks if remote branch SHA differs from local
func (g *gitGo) remoteSHAChanged(branch, sha string) (bool, error) {
	formattedURL := formatGitURL(g.URL, branch)
	if formattedURL == "" {
		return true, nil
	}

	client, err := g.httpClientWithCreds()
	if err != nil {
		logrus.Warnf("Problem creating http client to check git remote sha of repo [%v]: %v", g.URL, err)
		return true, nil
	}
	defer client.CloseIdleConnections()

	req, err := http.NewRequest("GET", formattedURL, nil)
	if err != nil {
		logrus.Warnf("Problem creating request to check git remote sha of repo [%v]: %v", g.URL, err)
		return true, nil
	}

	req.Header.Set("Accept", "application/vnd.github.v3.sha")
	req.Header.Set("If-None-Match", fmt.Sprintf("\"%s\"", sha))
	for k, v := range g.headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		// Return timeout errors so caller can decide whether or not to proceed with updating the repo
		// On network errors, return true to allow update attempt
		return true, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return false, nil
	}

	return true, nil
}
