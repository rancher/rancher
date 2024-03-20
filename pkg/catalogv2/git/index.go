package git

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rancher/rancher/pkg/catalogv2/chart"
	"helm.sh/helm/v3/pkg/provenance"
	"helm.sh/helm/v3/pkg/repo"
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

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
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
