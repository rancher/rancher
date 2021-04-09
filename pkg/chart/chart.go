package chart

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/name"
	v1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	chartName = "chart-contents"
)

var (
	// Rancher is a global variable that hold the current helm chart for Rancher
	Rancher *embed.FS
)

func versionAndAppVersion() (string, string) {
	appVersion := settings.ServerVersion.Get()
	version := strings.TrimPrefix(appVersion, "v")

	_, err := semver.NewVersion(version)
	if err != nil {
		version = name.SafeConcatName("0.0.0", appVersion)
	}

	if appVersion == "dev" {
		appVersion = "master-head"
	}

	return version, appVersion
}

func Populate(ctx context.Context, config corecontrollers.ConfigMapClient) error {
	if Rancher == nil {
		return nil
	}

	files := map[string][]byte{}
	digest := sha256.New()

	err := fs.WalkDir(Rancher, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		f, err := Rancher.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		content, err := io.ReadAll(f)
		if err != nil {
			return err
		}

		path = strings.TrimPrefix(path, "chart/")
		if path == "Chart.yaml" {
			version, appVersion := versionAndAppVersion()
			content = bytes.ReplaceAll(content, []byte("%VERSION%"), []byte(version))
			content = bytes.ReplaceAll(content, []byte("%APP_VERSION%"), []byte(appVersion))
		} else if path == "values.yaml" {
			_, appVersion := versionAndAppVersion()
			content = bytes.ReplaceAll(content, []byte("%POST_DELETE_IMAGE_NAME%"), []byte(settings.AgentImage.Get()))
			content = bytes.ReplaceAll(content, []byte("%POST_DELETE_IMAGE_TAG%"), []byte(appVersion))
		}
		digest.Write([]byte(path))
		digest.Write(content)

		files[path] = content
		return nil
	})
	if err != nil {
		return err
	}

	hash := hex.EncodeToString(digest.Sum(nil)[:])
	json, err := json.Marshal(files)
	if err != nil {
		return err
	}
	data := map[string]string{
		"files": string(json),
	}

	existing, err := config.Get(namespace.System, chartName, metav1.GetOptions{})
	if apierror.IsNotFound(err) {
		_, err := config.Create(&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace.System,
				Name:      chartName,
				Annotations: map[string]string{
					"chart.cattle.io/hash": hash,
				},
			},
			Data: data,
		})
		return err
	} else if err != nil {
		return err
	} else if existing.Annotations["chart.cattle.io/hash"] != hash {
		existing.Data = data
		_, err := config.Update(existing)
		return err
	}

	return nil
}
