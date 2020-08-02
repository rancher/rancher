package helm

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/ioutil"
	"strings"

	"github.com/rancher/rancher/pkg/api/steve/catalog/types"

	"sigs.k8s.io/yaml"
)

func decodeYAML(input io.Reader, target interface{}) error {
	data, err := ioutil.ReadAll(input)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, target)
}

func InfoFromTarball(input io.Reader) (*types.ChartInfo, error) {
	result := &types.ChartInfo{
		Values:    map[string]interface{}{},
		Questions: map[string]interface{}{},
		Chart:     map[string]interface{}{},
	}

	gz, err := gzip.NewReader(input)
	if err != nil {
		return nil, err
	}

	tarball := tar.NewReader(gz)
	for {
		file, err := tarball.Next()
		if err == io.EOF {
			break
		}

		parts := strings.SplitN(file.Name, "/", 2)
		if len(parts) == 1 {
			continue
		}

		switch strings.ToLower(parts[1]) {
		case "values.yaml":
			if err := decodeYAML(tarball, &result.Values); err != nil {
				return nil, err
			}
		case "questions.yaml":
			if err := decodeYAML(tarball, &result.Values); err != nil {
				return nil, err
			}
		case "chart.yaml":
			if err := decodeYAML(tarball, &result.Chart); err != nil {
				return nil, err
			}
		case "app-readme.md":
			bytes, err := ioutil.ReadAll(tarball)
			if err != nil {
				return nil, err
			}
			result.APPReadme = string(bytes)
		case "readme.md":
			bytes, err := ioutil.ReadAll(tarball)
			if err != nil {
				return nil, err
			}
			result.Readme = string(bytes)
		}
	}

	return result, nil
}
