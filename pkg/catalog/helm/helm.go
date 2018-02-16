package helm

import (
	"archive/tar"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"

	yaml "gopkg.in/yaml.v2"

	"encoding/base64"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

func DownloadIndex(indexURL string) (*RepoIndex, error) {
	if indexURL[len(indexURL)-1:] == "/" {
		indexURL = indexURL[:len(indexURL)-1]
	}
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
		URL:       indexURL,
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

	f, err := os.OpenFile(indexPath, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return nil
	}

	_, err = f.Write(fileBytes)
	return err
}

func LoadIndex(repoPath string) (*RepoIndex, error) {
	indexPath := path.Join(repoPath, "index.yaml")

	f, err := os.Open(indexPath)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(f)
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

func FetchFiles(urls []string) ([]v3.File, error) {
	if len(urls) == 0 {
		return nil, nil
	}

	var files []v3.File
	for _, url := range urls {
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
				files = append(files, filterFile(v3.File{
					Name:     name,
					Contents: base64.StdEncoding.EncodeToString(contents),
				}))
			}
		}
	}
	return files, nil
}

func LoadMetadata(path string) (*ChartMetadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	metadata := &ChartMetadata{}
	return metadata, yaml.Unmarshal(data, metadata)
}

func LoadFile(path string) (*v3.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	filteredFile := filterFile(v3.File{
		Name:     f.Name(),
		Contents: base64.StdEncoding.EncodeToString(data),
	})
	return &filteredFile, nil
}
