package git

import "errors"

// ErrNotCheckedOut is returned when a repository's files are requested before its working tree has
// been restored in this process.
var ErrNotCheckedOut = errors.New("repository has not been checked out yet")

func markCheckedOut(dir string) {}

func clearCheckedOut(dir string) {}

// IsCheckedOut returns true if this process has restored the working tree of the ClusterRepo's
// repository.
func IsCheckedOut(name, gitURL string) bool { return false }

// ForgetRepo drops the checkout state of every repository cloned for the ClusterRepo.
func ForgetRepo(name string) {}
