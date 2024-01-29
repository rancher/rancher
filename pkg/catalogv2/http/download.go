package http

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/rancher/wrangler/v2/pkg/schemas/validation"

	"sigs.k8s.io/yaml"

	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/repo"
	corev1 "k8s.io/api/core/v1"
)

func Icon(secret *corev1.Secret, repoURL string, caBundle []byte, insecureSkipTLSVerify bool, disableSameOriginCheck bool, chart *repo.ChartVersion) (io.ReadCloser, string, error) {
	if len(chart.URLs) == 0 {
		return nil, "", fmt.Errorf("failed to find chartName %s version %s: %w", chart.Name, chart.Version, validation.NotFound)
	}

	client, err := HelmClient(secret, caBundle, insecureSkipTLSVerify, disableSameOriginCheck, repoURL)
	if err != nil {
		return nil, "", err
	}
	defer client.CloseIdleConnections()

	u, err := url.Parse(chart.Icon)
	if err != nil {
		return nil, "", err
	}
	if !u.IsAbs() {
		base, err := url.Parse(repoURL)
		if err != nil {
			return nil, "", err
		}
		base.Path = strings.TrimSuffix(base.Path, "/") + "/"
		u = base.ResolveReference(u)
		u.RawQuery = base.RawQuery
	}

	resp, err := client.Get(u.String())
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		defer ioutil.ReadAll(resp.Body)
		return nil, "", validation.ErrorCode{
			Status: resp.StatusCode,
		}
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return ioutil.NopCloser(bytes.NewBuffer(data)), path.Ext(u.Path), nil
}

func Chart(secret *corev1.Secret, repoURL string, caBundle []byte, insecureSkipTLSVerify bool, disableSameOriginCheck bool, chart *repo.ChartVersion) (io.ReadCloser, error) {
	if len(chart.URLs) == 0 {
		return nil, fmt.Errorf("failed to find chartName %s version %s: %w", chart.Name, chart.Version, validation.NotFound)
	}

	client, err := HelmClient(secret, caBundle, insecureSkipTLSVerify, disableSameOriginCheck, repoURL)
	if err != nil {
		return nil, err
	}
	defer client.CloseIdleConnections()

	u, err := url.Parse(chart.URLs[0])
	if err != nil {
		return nil, err
	}
	if !u.IsAbs() {
		base, err := url.Parse(repoURL)
		if err != nil {
			return nil, err
		}
		// Prevent ResolveReference from stripping the last element
		// of the path by ensuring it has a trailing slash
		base.Path = strings.TrimSuffix(base.Path, "/") + "/"
		u = base.ResolveReference(u)
		// Retain the query string of the repository URL as it might
		// contain an access credential.
		u.RawQuery = base.RawQuery
	}

	resp, err := client.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	return ioutil.NopCloser(bytes.NewBuffer(data)), err
}

func DownloadIndex(secret *corev1.Secret, repoURL string, caBundle []byte, insecureSkipTLSVerify bool, disableSameOriginCheck bool) (*repo.IndexFile, error) {
	client, err := HelmClient(secret, caBundle, insecureSkipTLSVerify, disableSameOriginCheck, repoURL)
	if err != nil {
		return nil, err
	}
	defer client.CloseIdleConnections()

	parsedURL, err := url.Parse(repoURL)
	if err != nil {
		return nil, err
	}

	parsedURL.RawPath = path.Join(parsedURL.RawPath, "index.yaml")
	parsedURL.Path = path.Join(parsedURL.Path, "index.yaml")

	url := parsedURL.String()
	logrus.Infof("Downloading repo index from %s", url)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Install-Uuid", settings.InstallUUID.Get())

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Marshall to file to ensure it matches the schema and this component doesn't just
	// become a "fetch any file" service.
	index := &repo.IndexFile{}
	if err := yaml.Unmarshal(bytes, index); err != nil {
		logrus.Errorf("failed to unmarshal %s: %v", url, err)
		return nil, fmt.Errorf("failed to parse response from %s", url)
	}

	if index.APIVersion == "" {
		return nil, repo.ErrNoAPIVersion
	}

	return index, nil
}
