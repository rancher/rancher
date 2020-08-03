package token

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/gofrs/flock"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// env variable name for custom credential cache file location
const cacheFileNameEnv = "AWS_IAM_AUTHENTICATOR_CACHE_FILE"

// A mockable filesystem interface
var f filesystem = osFS{}

type filesystem interface {
	Stat(filename string) (os.FileInfo, error)
	ReadFile(filename string) ([]byte, error)
	WriteFile(filename string, data []byte, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
}

// default os based implementation
type osFS struct{}

func (osFS) Stat(filename string) (os.FileInfo, error) {
	return os.Stat(filename)
}

func (osFS) ReadFile(filename string) ([]byte, error) {
	return ioutil.ReadFile(filename)
}

func (osFS) WriteFile(filename string, data []byte, perm os.FileMode) error {
	return ioutil.WriteFile(filename, data, perm)
}

func (osFS) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// A mockable environment interface
var e environment = osEnv{}

type environment interface {
	Getenv(key string) string
	LookupEnv(key string) (string, bool)
}

// default os based implementation
type osEnv struct{}

func (osEnv) Getenv(key string) string {
	return os.Getenv(key)
}

func (osEnv) LookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}

// A mockable flock interface
type filelock interface {
	Unlock() error
	TryLockContext(ctx context.Context, retryDelay time.Duration) (bool, error)
	TryRLockContext(ctx context.Context, retryDelay time.Duration) (bool, error)
}

var newFlock = func(filename string) filelock {
	return flock.New(filename)
}

// cacheFile is a map of clusterID/roleARNs to cached credentials
type cacheFile struct {
	// a map of clusterIDs/profiles/roleARNs to cachedCredentials
	ClusterMap map[string]map[string]map[string]cachedCredential `yaml:"clusters"`
}

// a utility type for dealing with compound cache keys
type cacheKey struct {
	clusterID string
	profile   string
	roleARN   string
}

func (c *cacheFile) Put(key cacheKey, credential cachedCredential) {
	if _, ok := c.ClusterMap[key.clusterID]; !ok {
		// first use of this cluster id
		c.ClusterMap[key.clusterID] = map[string]map[string]cachedCredential{}
	}
	if _, ok := c.ClusterMap[key.clusterID][key.profile]; !ok {
		// first use of this profile
		c.ClusterMap[key.clusterID][key.profile] = map[string]cachedCredential{}
	}
	c.ClusterMap[key.clusterID][key.profile][key.roleARN] = credential
}

func (c *cacheFile) Get(key cacheKey) (credential cachedCredential) {
	if _, ok := c.ClusterMap[key.clusterID]; ok {
		if _, ok := c.ClusterMap[key.clusterID][key.profile]; ok {
			// we at least have this cluster and profile combo in the map, if no matching roleARN, map will
			// return the zero-value for cachedCredential, which expired a long time ago.
			credential = c.ClusterMap[key.clusterID][key.profile][key.roleARN]
		}
	}
	return
}

// cachedCredential is a single cached credential entry, along with expiration time
type cachedCredential struct {
	Credential credentials.Value
	Expiration time.Time
	// If set will be used by IsExpired to determine the current time.
	// Defaults to time.Now if CurrentTime is not set.  Available for testing
	// to be able to mock out the current time.
	currentTime func() time.Time
}

// IsExpired determines if the cached credential has expired
func (c *cachedCredential) IsExpired() bool {
	curTime := c.currentTime
	if curTime == nil {
		curTime = time.Now
	}
	return c.Expiration.Before(curTime())
}

// readCacheWhileLocked reads the contents of the credential cache and returns the
// parsed yaml as a cacheFile object.  This method must be called while a shared
// lock is held on the filename.
func readCacheWhileLocked(filename string) (cache cacheFile, err error) {
	cache = cacheFile{
		map[string]map[string]map[string]cachedCredential{},
	}
	data, err := f.ReadFile(filename)
	if err != nil {
		err = fmt.Errorf("unable to open file %s: %v", filename, err)
		return
	}

	err = yaml.Unmarshal(data, &cache)
	if err != nil {
		err = fmt.Errorf("unable to parse file %s: %v", filename, err)
	}
	return
}

// writeCacheWhileLocked writes the contents of the credential cache using the
// yaml marshaled form of the passed cacheFile object.  This method must be
// called while an exclusive lock is held on the filename.
func writeCacheWhileLocked(filename string, cache cacheFile) error {
	data, err := yaml.Marshal(cache)
	if err == nil {
		// write privately owned by the user
		err = f.WriteFile(filename, data, 0600)
	}
	return err
}

// FileCacheProvider is a Provider implementation that wraps an underlying Provider
// (contained in Credentials) and provides caching support for credentials for the
// specified clusterID, profile, and roleARN (contained in cacheKey)
type FileCacheProvider struct {
	credentials      *credentials.Credentials // the underlying implementation that has the *real* Provider
	cacheKey         cacheKey                 // cache key parameters used to create Provider
	cachedCredential cachedCredential         // the cached credential, if it exists
}

// NewFileCacheProvider creates a new Provider implementation that wraps a provided Credentials,
// and works with an on disk cache to speed up credential usage when the cached copy is not expired.
// If there are any problems accessing or initializing the cache, an error will be returned, and
// callers should just use the existing credentials provider.
func NewFileCacheProvider(clusterID, profile, roleARN string, creds *credentials.Credentials) (FileCacheProvider, error) {
	if creds == nil {
		return FileCacheProvider{}, errors.New("no underlying Credentials object provided")
	}
	filename := CacheFilename()
	cacheKey := cacheKey{clusterID, profile, roleARN}
	cachedCredential := cachedCredential{}
	// ensure path to cache file exists
	_ = f.MkdirAll(filepath.Dir(filename), 0700)
	if info, err := f.Stat(filename); !os.IsNotExist(err) {
		if info.Mode()&0077 != 0 {
			// cache file has secret credentials and should only be accessible to the user, refuse to use it.
			return FileCacheProvider{}, fmt.Errorf("cache file %s is not private", filename)
		}

		// do file locking on cache to prevent inconsistent reads
		lock := newFlock(filename)
		defer lock.Unlock()
		// wait up to a second for the file to lock
		ctx, cancel := context.WithTimeout(context.TODO(), time.Second)
		defer cancel()
		ok, err := lock.TryRLockContext(ctx, 250*time.Millisecond) // try to lock every 1/4 second
		if !ok {
			// unable to lock the cache, something is wrong, refuse to use it.
			return FileCacheProvider{}, fmt.Errorf("unable to read lock file %s: %v", filename, err)
		}

		cache, err := readCacheWhileLocked(filename)
		if err != nil {
			// can't read or parse cache, refuse to use it.
			return FileCacheProvider{}, err
		}

		cachedCredential = cache.Get(cacheKey)
	} else {
		// cache file is missing.  maybe this is the very first run?  continue to use cache.
		_, _ = fmt.Fprintf(os.Stderr, "Cache file %s does not exist.\n", filename)
	}

	return FileCacheProvider{
		creds,
		cacheKey,
		cachedCredential,
	}, nil
}

// Retrieve() implements the Provider interface, returning the cached credential if is not expired,
// otherwise fetching the credential from the underlying Provider and caching the results on disk
// with an expiration time.
func (f *FileCacheProvider) Retrieve() (credentials.Value, error) {
	if !f.cachedCredential.IsExpired() {
		// use the cached credential
		return f.cachedCredential.Credential, nil
	} else {
		_, _ = fmt.Fprintf(os.Stderr, "No cached credential available.  Refreshing...\n")
		// fetch the credentials from the underlying Provider
		credential, err := f.credentials.Get()
		if err != nil {
			return credential, err
		}
		if expiration, err := f.credentials.ExpiresAt(); err == nil {
			// underlying provider supports Expirer interface, so we can cache
			filename := CacheFilename()
			// do file locking on cache to prevent inconsistent writes
			lock := newFlock(filename)
			defer lock.Unlock()
			// wait up to a second for the file to lock
			ctx, cancel := context.WithTimeout(context.TODO(), time.Second)
			defer cancel()
			ok, err := lock.TryLockContext(ctx, 250*time.Millisecond) // try to lock every 1/4 second
			if !ok {
				// can't get write lock to create/update cache, but still return the credential
				_, _ = fmt.Fprintf(os.Stderr, "Unable to write lock file %s: %v\n", filename, err)
				return credential, nil
			}
			f.cachedCredential = cachedCredential{
				credential,
				expiration,
				nil,
			}
			// don't really care about read error.  Either read the cache, or we create a new cache.
			cache, _ := readCacheWhileLocked(filename)
			cache.Put(f.cacheKey, f.cachedCredential)
			err = writeCacheWhileLocked(filename, cache)
			if err != nil {
				// can't write cache, but still return the credential
				_, _ = fmt.Fprintf(os.Stderr, "Unable to update credential cache %s: %v\n", filename, err)
				err = nil
			} else {
				_, _ = fmt.Fprintf(os.Stderr, "Updated cached credential\n")
			}
		} else {
			// credential doesn't support expiration time, so can't cache, but still return the credential
			_, _ = fmt.Fprintf(os.Stderr, "Unable to cache credential: %v\n", err)
			err = nil
		}
		return credential, err
	}
}

// IsExpired() implements the Provider interface, deferring to the cached credential first,
// but fall back to the underlying Provider if it is expired.
func (f *FileCacheProvider) IsExpired() bool {
	return f.cachedCredential.IsExpired() && f.credentials.IsExpired()
}

// ExpiresAt implements the Expirer interface, and gives access to the expiration time of the credential
func (f *FileCacheProvider) ExpiresAt() time.Time {
	return f.cachedCredential.Expiration
}

// CacheFilename returns the name of the credential cache file, which can either be
// set by environment variable, or use the default of ~/.kube/cache/aws-iam-authenticator/credentials.yaml
func CacheFilename() string {
	if filename, ok := e.LookupEnv(cacheFileNameEnv); ok {
		return filename
	} else {
		return filepath.Join(UserHomeDir(), ".kube", "cache", "aws-iam-authenticator", "credentials.yaml")
	}
}

// UserHomeDir returns the home directory for the user the process is
// running under.
func UserHomeDir() string {
	if runtime.GOOS == "windows" { // Windows
		return e.Getenv("USERPROFILE")
	}

	// *nix
	return e.Getenv("HOME")
}
