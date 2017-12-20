package config

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

func extractConfig(baseDir, extractedConfig string) error {
	baseDir = filepath.Dir(baseDir)
	configBytes, err := base64.StdEncoding.DecodeString(extractedConfig)
	if err != nil {
		return fmt.Errorf("error reinitializing config (base64.DecodeString). Config Dir: %v. Error: %v", baseDir, err)
	}

	gzipReader, err := gzip.NewReader(bytes.NewReader(configBytes))
	if err != nil {
		return err
	}
	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("error reinitializing config (tarRead.Next). Config Dir: %v. Error: %v", baseDir, err)
		}

		filename := header.Name
		filePath := filepath.Join(baseDir, filename)
		logrus.Debugf("Extracting %v", filePath)

		info := header.FileInfo()
		if info.IsDir() {
			err = os.MkdirAll(filePath, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("error reinitializing config (Mkdirall). Config Dir: %v. Dir: %v. Error: %v", baseDir, info.Name(), err)
			}
			continue
		}

		file, err := os.OpenFile(filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		if err != nil {
			return fmt.Errorf("error reinitializing config (OpenFile). Config Dir: %v. File: %v. Error: %v", baseDir, info.Name(), err)
		}

		_, err = io.Copy(file, tarReader)
		file.Close()
		if err != nil {
			return fmt.Errorf("error reinitializing config (Copy). Config Dir: %v. File: %v. Error: %v", baseDir, info.Name(), err)
		}
	}
}

func compressConfig(baseDir string) (string, error) {
	// create the tar.gz file
	destFile := &bytes.Buffer{}

	fileWriter := gzip.NewWriter(destFile)
	tarfileWriter := tar.NewWriter(fileWriter)

	if err := addDirToArchive(baseDir, tarfileWriter); err != nil {
		return "", err
	}

	tarfileWriter.Close()
	fileWriter.Close()

	return base64.StdEncoding.EncodeToString(destFile.Bytes()), nil
}

func addDirToArchive(source string, tarfileWriter *tar.Writer) error {
	baseDir := filepath.Base(source)

	return filepath.Walk(source,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if path == source || strings.HasSuffix(info.Name(), ".iso") ||
				strings.HasSuffix(info.Name(), ".tar.gz") ||
				strings.HasSuffix(info.Name(), ".vmdk") ||
				strings.HasSuffix(info.Name(), ".img") {
				return nil
			}

			header, err := tar.FileInfoHeader(info, info.Name())
			if err != nil {
				return err
			}

			header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))

			if err := tarfileWriter.WriteHeader(header); err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(tarfileWriter, file)
			return err
		})
}
