package git

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rancher/rancher/pkg/catalogv2/chart"
	"helm.sh/helm/v4/pkg/provenance"
	repo "helm.sh/helm/v4/pkg/repo/v1"
)

func BuildOrGetIndex(namespace, name, gitURL string) (*repo.IndexFile, error) {
	dir := RepoDir(namespace, name, gitURL)
	return buildOrGetIndex(dir)
}

func buildOrGetIndex(dir string) (*repo.IndexFile, error) {
	// Run first: this is a security check, so it has to happen before anything is read from the
	// repository. It is only a stat of each entry, so it is cheap relative to the work below.
	if err := ensureNoSymlinks(dir); err != nil {
		return nil, err
	}

	// A repository that ships its own index.yaml at its root is the common case, and it is the
	// index the walk below would settle on anyway, since no path can be shallower than the repo
	// root. Take it directly.
	//
	// The walk is very expensive to reach the same answer: for every chart version it finds it
	// runs chart.LoadArchive, which re-tars the chart to a temporary file, and then digests that
	// file with SHA-256 — and then discards all of it at the end in favour of this very file.
	// On a cluster agent bootstrapping against the full rancher/charts tree that accounted for
	// minutes before any system chart could be installed.
	if index := rootIndex(dir); index != nil {
		return index, nil
	}

	var (
		existingIndex *repo.IndexFile
		indexPath     = ""
		builtIndex    = repo.NewIndexFile()
	)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if info.Name() == "index.yaml" {
			if indexPath == "" || len(path) < len(indexPath) {
				if index, err := repo.LoadIndexFile(path); err == nil {
					existingIndex = index
					indexPath = path
					return filepath.SkipDir
				}
			}
		}

		if !info.IsDir() {
			return nil
		}

		archive, ok, err := chart.LoadArchive(path)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}

		digest, err := provenance.DigestFile(archive.Path)
		archive.Close()
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return fmt.Errorf("building path for chart at %s: %w", dir, err)
		}

		builtIndex.Add(archive.Metadata, rel, "", digest)
		return filepath.SkipDir
	})
	if err != nil {
		return nil, err
	}

	if existingIndex != nil {
		return existingIndex, nil
	}

	return builtIndex, nil
}

// rootIndex loads the repository's own index.yaml, or returns nil if it does not have a usable
// one so that the caller falls back to walking the tree.
//
// A missing or malformed index.yaml is not an error here: the walk in buildOrGetIndex likewise
// ignores an index.yaml it cannot load and goes on to build an index from the charts it finds.
func rootIndex(dir string) *repo.IndexFile {
	path := filepath.Join(dir, "index.yaml")

	// ensureNoSymlinks has already rejected the whole tree if anything in it is a symlink, so
	// this only confirms the entry exists and is a plain file.
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		return nil
	}

	index, err := repo.LoadIndexFile(path)
	if err != nil {
		return nil
	}
	return index
}

func ensureNoSymlinks(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return err
		}
		if isSymlink(info) {
			return fmt.Errorf("symlink found at path %s", path)
		}
		return nil
	})
}

func isSymlink(info os.FileInfo) bool {
	return info.Mode()&os.ModeSymlink != 0
}
