package nodeconfig

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/pkg/errors"
)

func ExtractConfigJSON(extractedConfig string) (map[string]interface{}, error) {
	result := map[string]interface{}{}

	configBytes, err := base64.StdEncoding.DecodeString(extractedConfig)
	if err != nil {
		return nil, fmt.Errorf("error reinitializing config (base64.DecodeString): %v", err)
	}

	gzipReader, err := gzip.NewReader(bytes.NewReader(configBytes))
	if err != nil {
		return nil, err
	}
	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				return result, nil
			}
			return nil, fmt.Errorf("error reinitializing config (tarRead.Next): %v", err)
		}

		info := header.FileInfo()
		if info.IsDir() {
			continue
		}

		filename := header.Name
		if strings.Contains(filename, "/machines/") && strings.HasSuffix(filename, "/config.json") {
			buf := &bytes.Buffer{}
			_, err = io.Copy(buf, tarReader)
			if err != nil {
				return nil, fmt.Errorf("error reinitializing config (Copy): %v", err)
			}

			if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
				return nil, errors.Wrap(err, "failed to read config.json")
			}
		}
	}
}
