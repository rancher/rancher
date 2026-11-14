package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

var (
	// ErrNotFound is returned when an artifact with the specified digest is not present in the cache.
	ErrNotFound = errors.New("not found")

	// ErrDigestMismatch is returned when the content of an artifact does not match its expected digest.
	ErrDigestMismatch = errors.New("digest mismatch")

	// ErrInvalidDigest is returned when a digest string is malformed or not a valid SHA-256 hex string.
	ErrInvalidDigest = errors.New("invalid digest")
)

const (
	// DefaultCacheDir is the default directory where chart archives are stored by their SHA-256 digest.
	DefaultCacheDir = "management-state/catalog-cache-sha256"
	tmpSubdir       = "tmp"
)

// CAS provides content-addressable storage for Helm chart archives keyed by their SHA-256 digest.
type CAS interface {
	Get(digest string) (io.ReadCloser, error)
	Put(digest string, r io.Reader) error
	Has(digest string) bool
	Path(digest string) (string, error)
}

// DiskCAS implements CAS on the local filesystem.
type DiskCAS struct {
	rootDir string
	tmpDir  string
}

// NewDiskCAS creates a new DiskCAS with the specified root directory.
func NewDiskCAS(rootDir string) (*DiskCAS, error) {
	if rootDir == "" {
		rootDir = DefaultCacheDir
	}
	tmpDir := filepath.Join(rootDir, tmpSubdir)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create CAS cache directory %s: %w", tmpDir, err)
	}
	return &DiskCAS{
		rootDir: rootDir,
		tmpDir:  tmpDir,
	}, nil
}

// NormalizeDigest cleans and validates an expected SHA-256 digest string.
// It removes any "sha256:" prefix and ensures the result is a valid 64-character hexadecimal hash.
func NormalizeDigest(digest string) (string, error) {
	clean := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(digest), "sha256:"))
	if len(clean) != 64 {
		return "", fmt.Errorf("invalid sha256 digest length for %q: %w", digest, ErrInvalidDigest)
	}
	for _, c := range clean {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return "", fmt.Errorf("invalid sha256 hex character in digest %q: %w", digest, ErrInvalidDigest)
		}
	}
	return clean, nil
}

func (c *DiskCAS) pathNormalized(hash string) string {
	// using a "hash[:2]" subdirectory provides better disk performance, avoiding directories with too many files.
	return filepath.Join(c.rootDir, hash[:2], hash+".tgz")
}

// Path returns the filesystem path where the chart archive for the given digest is located.
func (c *DiskCAS) Path(digest string) (string, error) {
	hash, err := NormalizeDigest(digest)
	if err != nil {
		return "", err
	}
	return c.pathNormalized(hash), nil
}

// Has reports whether a chart archive with the given digest exists in the cache.
func (c *DiskCAS) Has(digest string) bool {
	target, err := c.Path(digest)
	if err != nil {
		return false
	}
	info, err := os.Stat(target)
	return err == nil && !info.IsDir()
}

// Get returns an open reader for the chart archive corresponding to the given digest,
// verifying that its SHA-256 hash matches the expected digest.
// If the file is missing, an error wrapping ErrNotFound is returned.
// If the file is corrupted or fails verification, it is evicted from the cache
// and an error wrapping ErrDigestMismatch and ErrNotFound is returned.
func (c *DiskCAS) Get(digest string) (io.ReadCloser, error) {
	expectedHash, err := NormalizeDigest(digest)
	if err != nil {
		return nil, err
	}
	target := c.pathNormalized(expectedHash)

	f, err := os.Open(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("artifact with digest %s not found: %w", digest, ErrNotFound)
		}
		return nil, err
	}

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("failed to verify cached artifact %s: %w", target, err)
	}

	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if actualHash != expectedHash {
		_ = f.Close()
		_ = os.Remove(target)
		logrus.Warnf("cas cache: artifact %s failed digest verification (got %s, expected %s), evicting", target, actualHash, expectedHash)
		return nil, fmt.Errorf("cached artifact %s failed digest verification: %w", target, errors.Join(ErrDigestMismatch, ErrNotFound))
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("failed to seek cached artifact %s: %w", target, err)
	}

	return f, nil
}

// Put streams chart bytes from r into a temporary file, verifies that its SHA-256 hash
// matches expectedDigest (if non-empty), and atomically moves it to its content-addressed location.
func (c *DiskCAS) Put(digest string, r io.Reader) error {
	var expectedHash string
	if digest != "" {
		var err error
		expectedHash, err = NormalizeDigest(digest)
		if err != nil {
			return err
		}
	}

	tmpFile, err := os.CreateTemp(c.tmpDir, "cas-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file in %s: %w", c.tmpDir, err)
	}
	tmpName := tmpFile.Name()
	defer func() {
		if tmpName != "" {
			_ = os.Remove(tmpName)
		}
	}()

	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmpFile, hasher), r); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write chart bytes to temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if expectedHash != "" && actualHash != expectedHash {
		return fmt.Errorf("digest mismatch: computed %s, expected %s: %w", actualHash, expectedHash, ErrDigestMismatch)
	}

	targetPath := filepath.Join(c.rootDir, actualHash[:2], actualHash+".tgz")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("failed to create shard directory %s: %w", filepath.Dir(targetPath), err)
	}

	_ = os.Chmod(tmpName, 0o400)
	if err := os.Rename(tmpName, targetPath); err != nil {
		return fmt.Errorf("failed to rename temp file to %s: %w", targetPath, err)
	}
	tmpName = "" // cleanup not needed

	return nil
}
