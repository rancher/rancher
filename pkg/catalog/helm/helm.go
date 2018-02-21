package helm

import (
	"archive/tar"
	"compress/gzip"
	"crypto/md5"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/blang/semver"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

func DownloadIndex(indexURL string) (*RepoIndex, error) {
	indexURL = strings.TrimSuffix(indexURL, "/")
	indexURL = indexURL + "/index.yaml"
	resp, err := http.Get(indexURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
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

func SaveIndex(index *RepoIndex, repoPath string) error {
	fileBytes, err := yaml.Marshal(index.IndexFile)
	if err != nil {
		return err
	}

	indexPath := path.Join(repoPath, "index.yaml")
	return ioutil.WriteFile(indexPath, fileBytes, 0755)
}

func LoadIndex(repoPath string) (*RepoIndex, error) {
	indexPath := path.Join(repoPath, "index.yaml")

	body, err := ioutil.ReadFile(indexPath)
	if os.IsNotExist(err) {
		return buildIndex(repoPath)
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

func FetchTgz(url string) ([]v3.File, error) {
	var files []v3.File

	logrus.Infof("Fetching file %s", url)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
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
			files = append(files, v3.File{
				Name:     name,
				Contents: base64.StdEncoding.EncodeToString(contents),
			})
		}
	}

	return files, nil
}

func FetchFiles(version *ChartVersion, urls []string) ([]v3.File, error) {
	if len(urls) == 0 {
		return nil, nil
	}

	var files []v3.File
	for _, url := range urls {
		if strings.HasPrefix(url, "file://") {
			newFile, err := LoadFile(version, strings.TrimPrefix(url, "file://"))
			if err != nil {
				return nil, err
			}
			files = append(files, *newFile)
			continue
		}

		newFiles, err := FetchTgz(url)
		if err != nil {
			return nil, err
		}
		files = append(files, newFiles...)
	}
	return files, nil
}

func LoadFile(version *ChartVersion, path string) (*v3.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	return &v3.File{
		Name:     filepath.Join(version.Name, strings.TrimPrefix(f.Name(), version.Dir+"/")),
		Contents: base64.StdEncoding.EncodeToString(data),
	}, nil
}

func buildIndex(repoPath string) (*RepoIndex, error) {
	index := &RepoIndex{
		IndexFile: &IndexFile{
			Entries: map[string]ChartVersions{},
		},
	}

	filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
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

		digest := md5.New()
		version.Dir = filepath.Dir(path)
		filepath.Walk(version.Dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}

			version.URLs = append(version.URLs, "file://"+path)

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

func iconFromFile(versions ChartVersions) (string, string, error) {
	for _, version := range versions {
		if version.Dir == "" || version.Icon == "" {
			continue
		}

		filename := filepath.Base(version.Icon)
		iconFile := filepath.Join(filepath.Dir(version.Dir), filename)

		bytes, err := ioutil.ReadFile(iconFile)
		if err == nil {
			return base64.StdEncoding.EncodeToString(bytes), filename, nil
		}
	}

	return "", "", os.ErrNotExist
}

func Icon(versions ChartVersions) (string, string, error) {
	data, file, err := iconFromFile(versions)
	if err == nil {
		return data, file, nil
	}

	if len(versions) == 0 || versions[0].Icon == "" {
		return "", "", nil
	}

	url := versions[0].Icon

	resp, err := http.Get(url)
	if err != nil {
		return "", "", err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	parts := strings.Split(url, "/")
	iconFilename := parts[len(parts)-1]
	iconData := base64.StdEncoding.EncodeToString(body)

	return iconData, iconFilename, nil
}
