package git

import corev1 "k8s.io/api/core/v1"

// Options contains configuration for git client initialization
type Options struct {
	Credential        *corev1.Secret
	CABundle          []byte
	InsecureTLSVerify bool
	Headers           map[string]string
}

// gitClient abstracts git operations for catalog repositories.
// All methods are lowercase (package-private) as they are called by
// public package functions (Ensure, Head, Update) which handle
// higher-level concerns like bundled mode and error wrapping.
//
// Implementation: gitGo (go-git library)
type gitClient interface {
	// === Idempotent/Atomic Operations ===

	// reset performs a hard reset to a specific revision
	// Idempotent: calling multiple times has same effect
	reset(rev string) error

	// currentCommit returns the current HEAD commit SHA
	// Read-only: no side effects
	currentCommit() (string, error)

	// directory returns the local directory path
	// Pure: deterministic, no side effects
	directory() string

	// === Stateful/Composite Operations ===

	// clone ensures repository is cloned, cleaning directory if needed
	// Idempotent: safe to call multiple times (checks if already exists)
	// Network: may clone if .git doesn't exist
	clone(branch string) error

	// Update fetches latest changes and returns current commit SHA
	// Multi-step: clone → reset → currentCommit → remoteSHAChanged → fetchAndReset
	// Network: HTTP check for SHA, fetch if changed
	Update(branch string) (string, error)

	// fetchAndReset fetches a specific ref and resets to it
	// Multi-step: fetch + reset
	// Network: fetches from remote
	fetchAndReset(rev string) error
}

// commonGitFields contains shared state for all git implementations
type commonGitFields struct {
	URL               string
	Directory         string
	password          string
	caBundle          []byte
	insecureTLSVerify bool
	secret            *corev1.Secret
	headers           map[string]string
	knownHosts        []byte
}
