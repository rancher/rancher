package helm

import (
	"archive/tar"
	"compress/gzip"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/git"

	"github.com/rancher/norman/controller"
	mgmtv3 "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	nsutil "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"

	"github.com/blang/semver"
	"github.com/moby/locker"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

var (
	httpTimeout = time.Second * 30
	httpClient  = &http.Client{
		Timeout: httpTimeout,
	}
	uuid            = settings.InstallUUID.Get()
	Locker          = locker.New()
	CatalogCache    = filepath.Join("management-state", "catalog-cache")
	IconCache       = filepath.Join(CatalogCache, ".icon-cache")
	InternalCatalog = filepath.Join("..", "rancher-data", "local-catalogs")
)

type Helm struct {
	LocalPath   string
	IconPath    string
	catalogName string
	Hash        string
	Kind        string
	url         string
	branch      string
	username    string
	password    string
	lastCommit  string
}

func (h *Helm) lock() {
	Locker.Lock(h.Hash)
}

func (h *Helm) unlock() {
	Locker.Unlock(h.Hash)
}

func (h *Helm) lockAndVerifyCachePath() error {
	h.lock()
	if _, err := os.Stat(h.LocalPath); os.IsNotExist(err) {
		return err
	}
	if _, err := os.Stat(h.IconPath); os.IsNotExist(err) {
		return err
	}
	return nil
}

func (h *Helm) request(pathURL string) (*http.Response, error) {
	baseEndpoint, err := url.Parse(pathURL)
	if err != nil {
		return nil, err
	}
	if !baseEndpoint.IsAbs() {
		helmURLstring := h.url
		if !strings.HasSuffix(helmURLstring, "/") {
			helmURLstring = helmURLstring + "/"
		}
		helmURL, err := url.Parse(helmURLstring)
		if err != nil {
			return nil, err
		}
		baseEndpoint = helmURL.ResolveReference(baseEndpoint)
	}

	if err := git.ValidateURL(baseEndpoint.String()); err != nil {
		return nil, err
	}

	if len(h.username) > 0 && len(h.password) > 0 {
		baseEndpoint.User = url.UserPassword(h.username, h.password)
	}
	req, err := http.NewRequest(http.MethodGet, baseEndpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	return httpClient.Do(req)
}

func (h *Helm) downloadIndex(indexURL string) (*RepoIndex, error) {
	indexURL = strings.TrimSuffix(indexURL, "/")
	indexURL = indexURL + "/index.yaml"
	resp, err := h.request(indexURL)
	if err != nil {
		if e, ok := err.(net.Error); ok && e.Timeout() {
			return nil, errors.Errorf("Timeout in HTTP GET to [%s], did not respond in %s", indexURL, httpTimeout)
		}
		return nil, errors.Errorf("Error in HTTP GET to [%s], error: %s", indexURL, err)
	}
	defer resp.Body.Close()

	// only return forgot error if status code is unauthorized.
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return nil, &controller.ForgetError{Err: errors.Errorf("Unexpected HTTP status code %d from [%s], expected 200", resp.StatusCode, indexURL)}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("Unexpected HTTP status code %d from [%s], expected 200", resp.StatusCode, indexURL)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Errorf("Error while reading response from [%s], error: %s", indexURL, err)
	}

	sum := md5.Sum(body)
	hash := hex.EncodeToString(sum[:])

	helmRepoIndex := &RepoIndex{
		IndexFile: &IndexFile{},
		Hash:      hash,
	}
	err = yaml.Unmarshal(body, helmRepoIndex.IndexFile)
	if err != nil {
		return nil, errors.Errorf("error unmarshalling response from [%s]", indexURL)
	}
	return helmRepoIndex, nil
}

func (h *Helm) saveIndex(index *RepoIndex) error {
	fileBytes, err := yaml.Marshal(index.IndexFile)
	if err != nil {
		return err
	}

	indexPath := filepath.Join(h.LocalPath, "index.yaml")
	return ioutil.WriteFile(indexPath, fileBytes, 0644)
}

func (h *Helm) LoadIndex() (*RepoIndex, error) {
	err := h.lockAndVerifyCachePath()
	defer h.unlock()
	if err != nil {
		return nil, err
	}

	indexPath := filepath.Join(h.LocalPath, "index.yaml")

	body, err := ioutil.ReadFile(indexPath)
	if os.IsNotExist(err) {
		return h.buildIndex()
	}
	if err != nil {
		return nil, err
	}

	sum := md5.Sum(body)
	hash := hex.EncodeToString(sum[:])

	helmRepoIndex := &RepoIndex{
		IndexFile: &IndexFile{},
		Hash:      hash,
	}
	return helmRepoIndex, yaml.Unmarshal(body, helmRepoIndex.IndexFile)
}

func (h *Helm) fetchTgz(helmURL string) ([]v32.File, error) {
	var files []v32.File
	logrus.Debugf("Helm fetching file %s", helmURL)

	resp, err := h.request(helmURL)
	if err != nil || resp.StatusCode > 400 {
		if e, ok := err.(net.Error); ok && e.Timeout() {
			return nil, errors.Errorf("Timeout in HTTP GET to [%s], did not respond in %s", helmURL, httpTimeout)
		}
		if err == nil {
			return nil, errors.Errorf("Error in HTTP GET of [%s], received: %s", helmURL, resp.Status)
		}
		return nil, errors.Errorf("Error in HTTP GET of [%s], error: %s", helmURL, err)
	}
	defer resp.Body.Close()

	gzf, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, err
	}
	defer gzf.Close()

	tarReader := tar.NewReader(gzf)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg:
			fallthrough
		case tar.TypeRegA:
			name := header.Name
			contents, err := ioutil.ReadAll(tarReader)
			if err != nil {
				return nil, err
			}
			files = append(files, v32.File{
				Name:     name,
				Contents: string(contents),
			})
		}
	}

	return files, nil
}

func (h *Helm) FetchLocalFiles(version *ChartVersion) ([]v32.File, error) {
	err := h.lockAndVerifyCachePath()
	defer h.unlock()
	if err != nil {
		return nil, err
	}

	if len(version.LocalFiles) == 0 && len(version.URLs) == 0 {
		return nil, errors.New("No files or urls provided for helm fetch")
	}

	var files []v32.File
	for _, file := range version.LocalFiles {
		newFile, err := h.loadFile(version, file)
		if err != nil {
			return nil, err
		}
		files = append(files, *newFile)
	}

	return files, nil
}

func (h *Helm) loadChartFiles(versionDir, prefix string, filters []string) (map[string]string, error) {
	filemap := map[string]string{}
	versionPath := filepath.Join(h.LocalPath, versionDir)
	err := filepath.Walk(versionPath, func(filename string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		relPath, err := filepath.Rel(versionPath, filename)
		if err != nil {
			return err
		}
		name := path.Join(prefix, filepath.ToSlash(relPath))
		if !filterMatch(name, filters) {
			return nil
		}

		content, err := ioutil.ReadFile(filename)
		if err != nil {
			return err
		}
		filemap[name] = string(content)
		return nil
	})
	return filemap, err
}

func (h *Helm) LoadChart(templateVersion *v32.TemplateVersionSpec, filters []string) (map[string]string, error) {
	err := h.lockAndVerifyCachePath()
	defer h.unlock()
	if err != nil {
		return nil, err
	}

	version := templateVersion.Version
	versionDir := templateVersion.VersionDir
	versionName := templateVersion.VersionName
	versionURLs := templateVersion.VersionURLs

	// Read Helm files from Git repo
	if versionDir != "" {
		return h.loadChartFiles(versionDir, versionName, filters)
	}
	// Fetch Helm URLs or use cached data
	if len(versionURLs) > 0 {
		return h.loadChartURLs(version, versionName, versionURLs, filters)
	}

	return nil, errors.New("Template version has no URLs or directory for chart to load from")
}

func (h *Helm) fetchAndCacheURLs(versionPath, versionName string, versionURLs, filters []string) (map[string]string, error) {
	filemap := map[string]string{}
	files, err := h.fetchURLs(versionURLs)
	if err != nil {
		return nil, err
	}
	// existence of this file indicates the cache exists
	if err := os.MkdirAll(versionPath, 0755); err != nil {
		return nil, err
	}
	for _, file := range files {
		filename, err := filepath.Rel(versionName, file.Name)
		if err != nil {
			return nil, err
		}
		fp := filepath.Join(versionPath, filename)
		if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
			return nil, err
		}
		if err := ioutil.WriteFile(fp, []byte(file.Contents), 0644); err != nil {
			return nil, err
		}
		if filterMatch(file.Name, filters) {
			filemap[file.Name] = file.Contents
		}
	}
	return filemap, nil
}

func (h *Helm) loadChartURLs(version, versionName string, versionURLs, filters []string) (map[string]string, error) {
	versionDir := filepath.Join(versionName, version)
	versionPath := filepath.Join(h.LocalPath, versionDir)
	if _, err := os.Stat(versionPath); os.IsNotExist(err) {
		return h.fetchAndCacheURLs(versionPath, versionName, versionURLs, filters)
	}
	return h.loadChartFiles(versionDir, versionName, filters)
}

func filterMatch(match string, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	for _, filter := range filters {
		if strings.EqualFold(match, filter) {
			return true
		}
	}
	return false
}

func (h *Helm) fetchURLs(urls []string) ([]v32.File, error) {
	var (
		errs []error
	)
	for _, url := range urls {
		newFiles, err := h.fetchTgz(url)
		if err == nil {
			return newFiles, nil
		}
		errs = append(errs, err)
	}
	return nil, errors.Errorf("Error fetching helm URLs: %v", errs)
}

func (h *Helm) loadFile(version *ChartVersion, filename string) (*v32.File, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	versionPath := filepath.Join(h.LocalPath, version.Dir)
	relPath, err := filepath.Rel(versionPath, filename)
	if err != nil {
		return nil, err
	}

	return &v32.File{
		Name:     path.Join(version.Name, filepath.ToSlash(relPath)),
		Contents: string(data),
	}, nil
}

func (h *Helm) buildIndex() (*RepoIndex, error) {
	index := &RepoIndex{
		IndexFile: &IndexFile{
			Entries: map[string]ChartVersions{},
		},
	}

	filepath.Walk(h.LocalPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if !strings.EqualFold(info.Name(), "Chart.yaml") {
			return nil
		}

		version := &ChartVersion{}
		content, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		if err := yaml.Unmarshal(content, version); err != nil {
			return err
		}

		dir := filepath.Dir(path)
		relDir, err := filepath.Rel(h.LocalPath, dir)
		if err != nil {
			return err
		}
		version.Dir = relDir
		digest := md5.New()

		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}

			version.LocalFiles = append(version.LocalFiles, path)
			digest.Write([]byte(path))

			b := make([]byte, 8)
			binary.LittleEndian.PutUint64(b, uint64(info.Size()))
			digest.Write(b)

			binary.LittleEndian.PutUint64(b, uint64(info.ModTime().Second()))
			digest.Write(b)

			return nil
		})

		version.Digest = hex.EncodeToString(digest.Sum(nil))
		index.IndexFile.Entries[version.Name] = append(index.IndexFile.Entries[version.Name], version)

		return filepath.SkipDir
	})

	for _, versions := range index.IndexFile.Entries {
		sort.Slice(versions, func(i, j int) bool {
			left, err := semver.ParseTolerant(versions[i].Version)
			if err != nil {
				return false
			}

			right, err := semver.ParseTolerant(versions[j].Version)
			if err != nil {
				return false
			}

			// reverse sort
			return right.LT(left)
		})
	}

	return index, nil
}

func (h *Helm) loadCachedIcon(iconURL string) ([]byte, string, string, error) {
	hashName := md5Hash(iconURL)
	matches, err := filepath.Glob(filepath.Join(h.IconPath, hashName+".*"))
	if err != nil {
		return nil, "", "", err
	}
	if len(matches) != 1 {
		return nil, "", "", errors.New("Multiple icon cache matches")
	}
	filename := matches[0]
	iconBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, "", "", err
	}
	cacheFile := filepath.Base(filename)
	return iconBytes, cacheFile, iconURL, nil
}

// cacheIcon create the cache data & filename from the given parameters. If extension is "" then it will craft one
func (h *Helm) cacheIcon(iconURL, extension string, iconBytes []byte) (string, error) {
	if extension == "" {
		extension = craftExtension(iconURL)
	}

	hashName := md5Hash(iconURL) + extension
	if filepath.Ext(hashName) == "" {
		logrus.Debugf("No extension for: %s", hashName)
	}
	iconCacheFile := filepath.Join(h.IconPath, hashName)
	if err := ioutil.WriteFile(iconCacheFile, iconBytes, 0644); err != nil {
		return "", err
	}
	return hashName, nil
}

// craftExtension attempts to parse the given iconURL to create an appropriate extension
func craftExtension(iconURL string) string {
	parsedURL, err := url.Parse(iconURL)
	var filename string
	if err == nil {
		if parsedURL.Path != "" {
			filename = path.Base(parsedURL.Path)
		} else {
			filename = parsedURL.Host
		}
	} else {
		logrus.Debugf("url.Parse(%s) error [%s]", iconURL, err)
		parts := strings.Split(iconURL, "/")
		filename = parts[len(parts)-1]
	}
	return filepath.Ext(filename)

}

func (h *Helm) iconFromFile(iconURL, versionDir string) ([]byte, string, string, error) {
	filename := strings.TrimPrefix(iconURL, "file://")
	iconPath := filepath.Join(h.LocalPath, versionDir)
	iconFile := filepath.Join(iconPath, filename)
	if !strings.HasPrefix(iconFile, h.LocalPath) {
		return nil, "", "", errors.Errorf("Won't read [%s], outside of tmp path [%s]", iconFile, h.LocalPath)
	}

	iconBytes, err := ioutil.ReadFile(iconFile)
	if err != nil {
		return nil, "", "", err
	}
	newPath, err := filepath.Rel(h.LocalPath, iconFile)
	if err != nil {
		return nil, "", "", err
	}

	iconURL = "file://" + newPath
	cacheFile, err := h.cacheIcon(iconURL, "", iconBytes)
	if err != nil {
		return nil, "", "", err
	}

	return iconBytes, cacheFile, iconURL, nil
}

func (h *Helm) fetchIcon(iconURL, versionDir string) ([]byte, string, string, error) {
	if strings.HasPrefix(iconURL, "file:") {
		return h.iconFromFile(iconURL, versionDir)
	}
	if strings.HasPrefix(iconURL, "http:") || strings.HasPrefix(iconURL, "https:") {
		return nil, "", iconURL, nil
	}
	return nil, "", "", errors.Errorf("unknown file type [%s]", iconURL)
}

func (h *Helm) iconFromCache(cacheFile string) ([]byte, error) {
	filename := filepath.Join(h.IconPath, cacheFile)
	if !strings.HasPrefix(filename, h.IconPath) {
		return nil, errors.Errorf("Icon file [%s] outside of icon path [%s]", filename, h.IconPath)
	}
	return ioutil.ReadFile(filename)
}

func (h *Helm) LoadIcon(cacheFile, iconURL string) ([]byte, error) {
	err := h.lockAndVerifyCachePath()
	defer h.unlock()
	if err != nil {
		return nil, err
	}

	if iconBytes, err := h.iconFromCache(cacheFile); err == nil {
		return iconBytes, nil
	}
	iconBytes, _, _, err := h.fetchIcon(iconURL, "")
	return iconBytes, err
}

func (h *Helm) Icon(versions ChartVersions) (string, string, error) {
	err := h.lockAndVerifyCachePath()
	defer h.unlock()
	if err != nil {
		return "", "", err
	}

	failed := map[string]bool{}
	for _, version := range versions {
		if version.Icon == "" || failed[version.Icon] {
			continue
		}

		_, filename, url, err := h.fetchIcon(version.Icon, version.Dir)
		if err != nil {
			logrus.Infof("Helm icon error: %s", err)
			failed[version.Icon] = true
			continue
		}

		return filename, url, nil
	}

	// avoid catalog reload if icon fetch fails
	return "", "", nil
}

func SplitNamespaceAndName(id string) (string, string) {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) < 2 {
		return "", id
	}
	return parts[0], parts[1]
}

func GetCatalog(
	catalogType, namespace, catalogName string,
	catalogLister v3.CatalogLister,
	clusterCatalogLister v3.ClusterCatalogLister,
	projectCatalogLister v3.ProjectCatalogLister,
) (*v3.Catalog, error) {
	if catalogType == "" {
		if namespace == "" || namespace == nsutil.GlobalNamespace {
			catalogType = mgmtv3.CatalogType
		} else if strings.HasPrefix(namespace, "p-") {
			logrus.Warnf("Defaulting catalog type to project for [%s/%s]", namespace, catalogName)
			catalogType = mgmtv3.ProjectCatalogType
		} else {
			logrus.Warnf("Defaulting catalog type to cluster for [%s/%s]", namespace, catalogName)
			catalogType = mgmtv3.ClusterCatalogType
		}
	}
	switch catalogType {
	case mgmtv3.CatalogType:
		catalog, err := catalogLister.Get("", catalogName)
		if err != nil {
			return nil, err
		}
		return catalog, nil
	case mgmtv3.ClusterCatalogType:
		clusterCatalog, err := clusterCatalogLister.Get(namespace, catalogName)
		if err != nil {
			return nil, err
		}
		return &clusterCatalog.Catalog, nil
	case mgmtv3.ProjectCatalogType:
		projectCatalog, err := projectCatalogLister.Get(namespace, catalogName)
		if err != nil {
			return nil, err
		}
		return &projectCatalog.Catalog, nil
	}
	return nil, errors.Errorf("Unknown catalog type in namespace [%s] and name [%s]", namespace, catalogName)
}
