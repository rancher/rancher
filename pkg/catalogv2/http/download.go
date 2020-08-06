package http

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"

	"github.com/rancher/wrangler/pkg/schemas/validation"

	"sigs.k8s.io/yaml"

	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/repo"
	corev1 "k8s.io/api/core/v1"
)

func Icon(secret *corev1.Secret, repoURL string, caBundle []byte, insecureSkipTLSVerify bool, chart *repo.ChartVersion) (io.ReadCloser, string, error) {
	if len(chart.URLs) == 0 {
		return nil, "", fmt.Errorf("failed to find chartName %s version %s: %w", chart.Name, chart.Version, validation.NotFound)
	}

	client, err := HelmClient(secret, caBundle, insecureSkipTLSVerify)
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
		u = base.ResolveReference(u)
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
	return ioutil.NopCloser(bytes.NewBuffer(data)), path.Ext(u.String()), nil
}

func Chart(secret *corev1.Secret, repoURL string, caBundle []byte, insecureSkipTLSVerify bool, chart *repo.ChartVersion) (io.ReadCloser, error) {
	if len(chart.URLs) == 0 {
		return nil, fmt.Errorf("failed to find chartName %s version %s: %w", chart.Name, chart.Version, validation.NotFound)
	}

	client, err := HelmClient(secret, caBundle, insecureSkipTLSVerify)
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
		u = base.ResolveReference(u)
	}

	resp, err := client.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	return ioutil.NopCloser(bytes.NewBuffer(data)), nil
}

func DownloadIndex(secret *corev1.Secret, repoURL string, caBundle []byte, insecureSkipTLSVerify bool) (*repo.IndexFile, error) {
	client, err := HelmClient(secret, caBundle, insecureSkipTLSVerify)
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
