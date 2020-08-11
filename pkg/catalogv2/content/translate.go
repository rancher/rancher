package content

import (
	"net/url"

	"helm.sh/helm/v3/pkg/repo"
)

func TranslateURLs(baseURL *url.URL, index *repo.IndexFile) error {
	u := *baseURL
	for chartName, versions := range index.Entries {
		for _, version := range versions {
			v := url.Values{}
			v.Set("chartName", chartName)
			v.Set("version", version.Version)
			v.Set("link", "chart")
			u.RawQuery = v.Encode()
			version.URLs = []string{
				u.String(),
			}

			v.Set("link", "icon")
			if version.Metadata != nil && version.Icon != "" {
				u.RawQuery = v.Encode()
				version.Icon = u.String()
			}
		}
	}

	return nil
}
