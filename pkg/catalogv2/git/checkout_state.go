package git

import (
	"errors"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

// ErrNotCheckedOut is returned when a repository's files are requested before its working tree has
// been restored in this process.
//
// Bundled repositories are shipped without a working tree, and each Rancher replica restores it on
// its own (Ensure, Head and Update all hard reset). ClusterRepo.Status is shared by all replicas
// and can report a repository as ready before this replica has done so, hence the state is tracked
// per process.
var ErrNotCheckedOut = errors.New("repository has not been checked out yet")

// checkedOut holds the absolute directory of every repository whose working tree has been restored
// by this process.
var checkedOut sync.Map

func checkoutKey(dir string) string {
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}

// markCheckedOut records that the working tree of the repository in dir is populated.
func markCheckedOut(dir string) {
	if _, loaded := checkedOut.LoadOrStore(checkoutKey(dir), struct{}{}); !loaded {
		logrus.Infof("catalog repository %s: working tree restored", dir)
	}
}

// clearCheckedOut records that the working tree of the repository in dir can't be relied on, for
// instance because it is about to be cloned again.
func clearCheckedOut(dir string) {
	checkedOut.Delete(checkoutKey(dir))
}

// IsCheckedOut returns true if this process has restored the working tree of the ClusterRepo's
// repository. ClusterRepos are cluster scoped, hence there is no namespace.
func IsCheckedOut(name, gitURL string) bool {
	_, ok := checkedOut.Load(checkoutKey(RepoDir("", name, gitURL)))
	return ok
}

// ForgetRepo drops the checkout state of every repository cloned for the ClusterRepo, to be used
// once its directory has been removed.
func ForgetRepo(name string) {
	// ClusterRepos never use namespace cause they are cluster scoped so we pass ""
	parent := checkoutKey(ParentRepoDir("", name)) + string(filepath.Separator)
	checkedOut.Range(func(key, _ any) bool {
		if strings.HasPrefix(key.(string), parent) {
			checkedOut.Delete(key)
		}
		return true
	})
}

// markIfPopulated marks the repository in dir as checked out when its working tree is already there.
func markIfPopulated(dir string) bool { return false }

// worktreePopulated returns true if the working tree of the repository in dir is the one HEAD refers to.
func worktreePopulated(dir string) bool { return false }
