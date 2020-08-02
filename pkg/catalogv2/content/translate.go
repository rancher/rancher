package content

import (
	"net/url"
	"path"

	"helm.sh/helm/v3/pkg/repo"
)

func TranslateURLs(baseURL *url.URL, index *repo.IndexFile) error {
	u := *baseURL
	for chartName, versions := range index.Entries {
		for _, version := range versions {
			v := url.Values{}
			v.Set("chartName", chartName)
			v.Set("version", version.Version)
			u.RawQuery = v.Encode()
			u.Path = path.Join(baseURL.Path, "chart")
			version.URLs = []string{
				u.String(),
			}
			if version.Metadata != nil && version.Icon != "" {
				u.RawQuery = v.Encode()
				u.Path = path.Join(baseURL.Path, "icon")
				version.Icon = u.String()
			}
		}
	}

	return nil
}
