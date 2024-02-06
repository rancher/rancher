package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	filepathsecure "github.com/cyphar/filepath-securejoin"
	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/sirupsen/logrus"
)

const (
	FilesTxtFilename    = "files.txt"
	PackageJSONFilename = "plugin/package.json"

	// Cache states used by custom resources
	Cached   = "cached"
	Disabled = "disabled"
	Pending  = "pending"
)

var (
	FsCache        = FSCache{}
	FSCacheRootDir = filepath.Join("management-state", "uiplugin")
	osRemoveAll    = os.RemoveAll
	osStat         = os.Stat
	isDirEmpty     = isDirectoryEmpty
)

type FSCache struct{}

type PackageJSON struct {
	Version string `json:"version,omitempty"`
}

// SyncWithControllersCache takes in a slice of UI Plugins objects and syncs the filesystem cache with it
func (c FSCache) SyncWithControllersCache(cachedPlugins []*v1.UIPlugin) error {
	for _, p := range cachedPlugins {
		plugin := p.Spec.Plugin
		if plugin.NoCache {
			logrus.Debugf("skipped caching plugin [Name: %s Version: %s] cache is disabled [noCache: %v]", plugin.Name, plugin.Version, plugin.NoCache)
			continue
		}
		if isCached, err := c.isCached(plugin.Name, plugin.Version); err != nil {
			return err
		} else if isCached {
			logrus.Debugf("skipped caching plugin [Name: %s Version: %s] is already cached", plugin.Name, plugin.Version)
			continue
		}
		version, err := getVersionFromPackageJSON(fmt.Sprintf("%s/%s", plugin.Endpoint, PackageJSONFilename))
		if err != nil {
			return err
		}
		cachedVersion, err := semver.NewVersion(plugin.Version)
		if err != nil {
			return err
		}
		if !cachedVersion.Equal(version) {
			return fmt.Errorf("plugin [%s] version [%s] does not match version in controller's cache [%s]", plugin.Name, version.String(), cachedVersion.String())
		}
		files, err := fetchFilesTxt(fmt.Sprintf("%s/%s", plugin.Endpoint, FilesTxtFilename))
		if err != nil {
			return err
		}
		for _, file := range files {
			if file == "" {
				continue
			}
			data, err := fetchFile(plugin.Endpoint + "/" + file)
			if err != nil {
				return err
			}
			path, err := filepathsecure.SecureJoin(FSCacheRootDir, filepath.Join(plugin.Name, plugin.Version, file))
			if err != nil {
				return err
			}
			if err := c.Save(data, path); err != nil {
				logrus.Debugf("failed to cache plugin [Name: %s Version: %s] in filesystem [path: %s]", plugin.Name, plugin.Version, path)
			}
		}
	}

	return nil
}

// SyncWithIndex syncs up entries in the filesystem cache with the index's entries
// Entries that aren't in the index, but present in the filesystem cache are deleted
func (c FSCache) SyncWithIndex(index *SafeIndex, fsCacheFiles []string) error {
	for _, file := range fsCacheFiles {
		logrus.Debugf("syncing index with filesystem cache")
		// Splits /{root}/{pluginName}/{pluginVersion}/* from a fs cache path
		rel, _ := filepath.Rel(FSCacheRootDir, file)
		s := strings.Split(rel, "/")
		name := s[0]
		version := s[1]
		_, ok := index.Entries[name]
		if !ok || index.Entries[name].Version != version {
			err := c.Delete(name, version)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Save takes in data and a path to save it in the filesystem cache
func (c FSCache) Save(data []byte, path string) error {
	logrus.Debugf("creating file [%s]", path)
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return err
	}
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	out.Write(data)

	return nil
}

// Delete takes in a plugin's name and version, and deletes its entry in the filesystem cache
func (c FSCache) Delete(name, version string) error {
	p, err := filepathsecure.SecureJoin(FSCacheRootDir, name)
	if err != nil {
		return err
	}
	err = osRemoveAll(p)
	if err != nil {
		err = fmt.Errorf("failed to delete entry [Name: %s Version: %s] from filesystem cache: %s", name, version, err.Error())
		return err
	}
	logrus.Debugf("deleted plugin entry from cache [Name: %s Version: %s]", name, version)

	return nil
}

// isCached takes in the name and version of a plugin and returns true if
// it is cached (entry exists and files were fetched), returns false otherwise
func (c FSCache) isCached(name, version string) (bool, error) {
	path, err := filepathsecure.SecureJoin(FSCacheRootDir, filepath.Join(name, version))
	if err != nil {
		return false, err
	}
	_, err = osStat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		isEmpty, err := isDirEmpty(path)
		if err != nil {
			return false, err
		}
		if !isEmpty {
			return true, nil
		}

		return false, err
	}

	if !errors.Is(err, os.ErrNotExist) {
		isEmpty, err := isDirEmpty(path)
		if err != nil {
			return false, err
		} else if !isEmpty {
			return true, nil
		}
	} else if errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return false, nil
}

func fsCacheFilepathGlob(pattern string) ([]string, error) {
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("files matching glob pattern [%s] found in filesystem cache: %+v", pattern, files)

	return files, nil
}

// getVersionFromPackageJSON takes in a URL for a plugin's package.json, reads it, and returns a Semver object of the version contained in the file
func getVersionFromPackageJSON(packageJSONURL string) (*semver.Version, error) {
	data, err := fetchFile(packageJSONURL)
	if err != nil {
		return nil, err
	}
	var packageJSON PackageJSON
	err = json.Unmarshal(data, &packageJSON)
	if err != nil {
		return nil, err
	}
	version, err := semver.NewVersion(packageJSON.Version)
	if err != nil {
		return nil, err
	}

	return version, nil
}

// fetchFilesTxt takes in a URL for a plugin's files.txt, reads it, and returns a slice of the file paths contained in the file
func fetchFilesTxt(filesTxtURL string) ([]string, error) {
	data, err := fetchFile(filesTxtURL)
	if err != nil {
		return nil, err
	}
	files := strings.Split(string(data), "\n")

	return files, nil
}

// fetchFile reads the file from the given URL and returns the data
func fetchFile(URL string) ([]byte, error) {
	logrus.Debugf("fetching file [%s]", URL)
	resp, err := http.Get(URL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func isDirectoryEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}

	return false, err
}
