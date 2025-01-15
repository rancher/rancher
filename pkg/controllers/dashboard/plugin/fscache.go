package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/Masterminds/semver/v3"
	filepathsecure "github.com/cyphar/filepath-securejoin"
	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/settings"
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
	FsCache             = FSCache{}
	errMaxFileSizeError = fmt.Errorf("file size limit of %s bytes reached", settings.MaxUIPluginFileByteSize.Get())
	FSCacheRootDir      = filepath.Join("management-state", "uiplugin")
	osRemoveAll         = os.RemoveAll
	osStat              = os.Stat
	isDirEmpty          = isDirectoryEmpty
	fileNameRegex       = regexp.MustCompile(`(^[/\\.])|\.\.`)
)

type FSCache struct {
}

type PackageJSON struct {
	Version string `json:"version,omitempty"`
}

// SyncWithControllersCache takes in a UI Plugin object and syncs the filesystem cache with it
func (c FSCache) SyncWithControllersCache(p *v1.UIPlugin, forceUpdate bool) error {
	plugin := p.Spec.Plugin
	if plugin.NoCache {
		logrus.Debugf("skipped caching plugin [Name: %s Version: %s] cache is disabled [noCache: %v]", plugin.Name, plugin.Version, plugin.NoCache)
		return nil
	}
	if forceUpdate {
		err := c.Delete(plugin.Name, plugin.Version)
		if err != nil {
			return fmt.Errorf("failed to delete cache during forceUpdate. Error: %w", err)
		}
	} else {
		if isCached, err := c.isCached(plugin.Name, plugin.Version); err != nil {
			return fmt.Errorf("failed to check if plugin is cached. Error: %w", err)
		} else if isCached {
			logrus.Debugf("skipped caching plugin [Name: %s Version: %s] is already cached", plugin.Name, plugin.Version)
			return nil
		}
	}
	version, err := getVersionFromPackageJSON(fmt.Sprintf("%s/%s", plugin.Endpoint, PackageJSONFilename))
	if err != nil {
		return fmt.Errorf("failed to get version from package.json file. Error: %w", err)
	}
	cachedVersion, err := semver.NewVersion(plugin.Version)
	if err != nil {
		return fmt.Errorf("spec.plugin.version [%s] is not a semver version. Error: %w", plugin.Version, err)
	}
	if !cachedVersion.Equal(version) {
		return fmt.Errorf("plugin [%s] version [%s] does not match version in controller's cache [%s]", plugin.Name, version.String(), cachedVersion.String())
	}
	files, err := fetchFilesTxt(fmt.Sprintf("%s/%s", plugin.Endpoint, FilesTxtFilename))
	if err != nil {
		return fmt.Errorf("failed to get files.txt file. Error: %w", err)
	}
	for _, file := range files {
		if file == "" {
			continue
		}
		data, err := fetchFile(plugin.Endpoint + "/" + file)
		if err != nil {
			return fmt.Errorf("failed to fetch file [%s] .Error: %w", file, err)
		}
		path, err := filepathsecure.SecureJoin(FSCacheRootDir, filepath.Join(plugin.Name, plugin.Version, file))
		if err != nil {
			return fmt.Errorf("failed to build file [%s] path for caching. Error: %w", file, err)
		}
		if err := c.Save(data, path); err != nil {
			logrus.Debugf("failed to cache plugin [Name: %s Version: %s] in filesystem [path: %s]", plugin.Name, plugin.Version, path)
		}
	}

	return nil
}

// SyncWithIndex syncs up entries in the filesystem cache with the index's entries.
// Entries that aren't in the index, but present in the filesystem cache are deleted
func (c FSCache) SyncWithIndex(index *SafeIndex, fsCacheFiles []string) error {
	for _, file := range fsCacheFiles {
		logrus.Debugf("syncing index with filesystem cache")
		chartName, chartVersion, err := getChartNameAndVersion(file)
		if err != nil {
			return fmt.Errorf("failed to get chart name and version for file [%s]. Error: %w", file, err)
		}
		_, ok := index.Entries[chartName]
		if !ok || index.Entries[chartName].Version != chartVersion {
			err := c.Delete(chartName, chartVersion)
			if err != nil {
				return fmt.Errorf("failed to delete cache entry. Error: %w", err)
			}
		}
	}

	return nil
}

// Save takes in data and a path to save it in the filesystem cache
func (c FSCache) Save(data []byte, path string) error {
	logrus.Debugf("creating file [%s]", path)
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create cache directory with path [%s]. Error: %w", filepath.Dir(path), err)
	}
	out, err := os.OpenFile(path, syscall.O_RDWR|syscall.O_CREAT|syscall.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open file [%s]. Error: %w", path, err)
	}
	defer out.Close()
	out.Write(data)

	return nil
}

// Delete takes in a plugin's name and version, and deletes its entry in the filesystem cache
func (c FSCache) Delete(name, version string) error {
	p, err := filepathsecure.SecureJoin(FSCacheRootDir, name)
	if err != nil {
		return fmt.Errorf("failed to get cache path for [%s]. Error: %w", name, err)
	}
	err = osRemoveAll(p)
	if err != nil {
		err = fmt.Errorf("failed to delete entry [Name: %s Version: %s] from filesystem cache: %w", name, version, err)
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
		return false, fmt.Errorf("failed to get cache directory path [%s]. Error: %w", filepath.Join(name, version), err)
	}
	_, err = osStat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check cache directory for path [%s]. Error: %w", path, err)
	}
	isEmpty, err := isDirEmpty(path)
	if err != nil {
		return false, fmt.Errorf("failed to check if cache directory is empty for path [%s]. Error: %w", path, err)
	}
	if !isEmpty {
		return true, nil
	}
	return false, nil
}

// getChartNameAndVersion receives a filepath and returns the chart name and chart version.
// The path has to follow /{root}/{chartName}/{chartVersion}/*
func getChartNameAndVersion(file string) (string, string, error) {
	root, err := filepath.Abs(FSCacheRootDir)
	if err != nil {
		return "", "", fmt.Errorf("unable to get absolute root path: %w", err)
	}
	filePath, err := filepath.Abs(file)
	if err != nil {
		return "", "", fmt.Errorf("unable to get absolute file [%s] path: %w", file, err)
	}

	// file has to be rooted at FSCacheRootDir
	if strings.HasPrefix(filePath, root) {
		p, _ := strings.CutPrefix(filePath, root)
		s := strings.Split(p, string(os.PathSeparator))
		if len(s) < 3 {
			return "", "", fmt.Errorf("file path is not valid. Path provided: %s", file)
		}
		chartName := s[1]
		chartVersion := s[2]
		if _, err := semver.NewVersion(chartVersion); err != nil {
			return "", "", fmt.Errorf("invalid chart version [%s]: %w", chartVersion, err)
		}
		return chartName, chartVersion, nil
	}
	return "", "", fmt.Errorf("path root is not the root cache path. Path provided: %s", file)

}

func fsCacheFilepathGlob(pattern string) ([]string, error) {
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("malformed pattern [%s]. Error: %w", pattern, err)
	}
	logrus.Debugf("files matching glob pattern [%s] found in filesystem cache: %+v", pattern, files)

	return files, nil
}

// getVersionFromPackageJSON takes in a URL for a plugin's package.json, reads it, and returns a Semver object of the version contained in the file
func getVersionFromPackageJSON(packageJSONURL string) (*semver.Version, error) {
	data, err := fetchFile(packageJSONURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch package.json file. Error: %w", err)
	}
	var packageJSON PackageJSON
	err = json.Unmarshal(data, &packageJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal package.json file. Error: %w", err)
	}
	version, err := semver.NewVersion(packageJSON.Version)
	if err != nil {
		return nil, fmt.Errorf("version [%s] in package.json file is not a semver version. Error: %w", packageJSON.Version, err)
	}

	return version, nil
}

// fetchFilesTxt takes in a URL for a plugin's files.txt, reads it, and returns a slice of the file paths contained in the file
func fetchFilesTxt(filesTxtURL string) ([]string, error) {
	data, err := fetchFile(filesTxtURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch files.txt file. Error: %w", err)
	}
	files := strings.Split(string(data), "\n")

	err = validateFilesTxtEntries(files)
	if err != nil {
		return nil, fmt.Errorf("invalid file.txt file. Error: %w", err)
	}

	return files, nil
}

// fetchFile reads the file from the given URL and returns the data
func fetchFile(URL string) ([]byte, error) {
	logrus.Debugf("fetching file [%s]", URL)
	resp, err := http.Get(URL)
	if err != nil {
		return nil, fmt.Errorf("get request failed for URL [%s]. Error: %w", URL, err)
	}
	defer resp.Body.Close()
	maxFileSize, err := strconv.ParseInt(settings.MaxUIPluginFileByteSize.Get(), 10, 64)
	if err != nil {
		logrus.Errorf("failed to convert setting MaxUIPluginFileByteSize to int64, using fallback. err: %s", err.Error())
		maxFileSize = settings.DefaultMaxUIPluginFileSizeInBytes
	}
	if resp.ContentLength > maxFileSize {
		return nil, errMaxFileSizeError
	}
	reader := &io.LimitedReader{R: resp.Body, N: maxFileSize + 1}
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read responde body from URL [%s]. Error: %w", URL, err)
	}
	if reader.N == 0 {
		return nil, errMaxFileSizeError
	}
	return data, nil
}

func isDirectoryEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("failed to open directory for path [%s]. Error: %w", path, err)
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}

	return false, err
}

func validateFilesTxtEntries(entries []string) error {
	for _, entry := range entries {
		file, err := url.QueryUnescape(entry)
		if err != nil {
			return fmt.Errorf("failed to decode entry: %s", entry)
		}
		if fileNameRegex.MatchString(file) {
			return fmt.Errorf("invalid file entry: %s", file)
		}
	}
	return nil
}
