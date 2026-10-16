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
	if err := ensureNoSymlinks(dir); err != nil {
		return nil, err
	}

	var (
		existingIndex *repo.IndexFile
		indexPath     = ""
		builtIndex    = repo.NewIndexFile()
	)

	// Most repositories ship an index.yaml at the root, short-circuit if this is the case.
	if index := rootIndex(dir); index != nil {
		return index, nil
	}

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
