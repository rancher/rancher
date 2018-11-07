package templatecontent

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io/ioutil"
	"os"
	"path"

	"github.com/rancher/rancher/pkg/catalog/helm"
	"github.com/rancher/rancher/pkg/catalog/manager"
)

var cacheRoot = path.Join("./management-state", "catalog-controller", "cache", "templateContent")

func GetTemplateFromTag(tag string, urls []string, isIcon bool, versionDir, versionName string) (string, error) {
	fileLocation := path.Join(cacheRoot, tag)
	if _, err := os.Stat(fileLocation); err != nil {
		// if file doesn't exist, rebuild cache based on urls and fileTypes
		if isIcon {
			data, err := helm.RebuildIcon(urls[0])
			if err != nil {
				return "", err
			}
			if err := ioutil.WriteFile(fileLocation, data, 0777); err != nil {
				return "", err
			}
		} else {
			files, err := helm.FetchFiles(versionName, versionDir, urls)
			if err != nil {
				return "", err
			}
			for _, file := range files {
				t, content, err := manager.ZipAndHash(file.Contents)
				if err != nil {
					return "", err
				}
				if t == tag {
					if err := ioutil.WriteFile(path.Join(cacheRoot, tag), []byte(content), 0777); err != nil {
						return "", err
					}
				}
			}
		}
	}
	data, err := ioutil.ReadFile(fileLocation)
	if err != nil {
		return "", err
	}
	content, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return "", err
	}
	reader, err := gzip.NewReader(bytes.NewReader(content))
	if err != nil {
		return "", err
	}
	defer reader.Close()
	data, err = ioutil.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
