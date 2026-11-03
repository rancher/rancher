package git

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/go-git/go-git/v5/plumbing/object"
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

var errNotPopulated = errors.New("working tree is not populated")

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

// markIfPopulated marks the repository in dir as checked out when its working tree is already there,
// as it is when the repository is shipped with its files, and reports whether it did. Only the local
// files are looked at: no reset is needed, and none of the shipped content has to be served from
// behind the not ready window.
func markIfPopulated(dir string) bool {
	if !worktreePopulated(dir) {
		return false
	}
	markCheckedOut(dir)
	return true
}

// worktreePopulated returns true if the working tree of the repository in dir is the one HEAD
// refers to: the index lists exactly the files of the HEAD tree, with the same hashes and modes, and
// each of them is present on disk with the size the index records. Files are stat'ed and not hashed,
// which is much cheaper than a reset and enough to tell a restored working tree from the states a
// repository can be found in:
//   - cloned without a checkout, there is no index
//   - go-git's hard reset writes the index before the files, so an interrupted reset leaves an
//     index listing files that aren't there
//   - HEAD and the index point at different commits, as after a reset that didn't complete.
//
// It doesn't look for modified files, resetting is what cleans them.
func worktreePopulated(dir string) bool {
	repo, err := gogit.PlainOpen(dir)
	if err != nil {
		return false
	}
	head, err := repo.Head()
	if err != nil {
		return false
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return false
	}
	tree, err := commit.Tree()
	if err != nil {
		return false
	}
	idx, err := repo.Storer.Index()
	if err != nil || len(idx.Entries) == 0 {
		return false
	}

	entries := make(map[string]*index.Entry, len(idx.Entries))
	for _, entry := range idx.Entries {
		if entry.Mode == filemode.Submodule {
			continue
		}
		// A stage other than 0 is a path with a merge in progress. go-git's own index.Merged constant
		// is 1 and can't be used for this.
		if entry.Stage != 0 {
			return false
		}
		entries[entry.Name] = entry
	}

	files := 0
	err = tree.Files().ForEach(func(f *object.File) error {
		files++
		entry, ok := entries[f.Name]
		if !ok || entry.Hash != f.Hash || entry.Mode != f.Mode {
			return errNotPopulated
		}
		info, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(f.Name)))
		if err != nil || (entry.Mode.IsRegular() && uint32(info.Size()) != entry.Size) {
			return errNotPopulated
		}
		return nil
	})
	// Every entry of the index has been found in the tree if the counts match
	return err == nil && files == len(entries)
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

// ForgetRepo drops the checkout state of every repository cloned under parentDir, the directory
// returned by ParentRepoDir for the ClusterRepo. It takes the directory rather than resolving it
// again because ParentRepoDir falls back to the state directory once a bundled one is removed.
func ForgetRepo(parentDir string) {
	parent := checkoutKey(parentDir) + string(filepath.Separator)
	checkedOut.Range(func(key, _ any) bool {
		if strings.HasPrefix(key.(string), parent) {
			checkedOut.Delete(key)
		}
		return true
	})
}
