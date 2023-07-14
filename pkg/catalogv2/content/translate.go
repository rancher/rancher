package content

import (
	"net/url"

	"helm.sh/helm/v3/pkg/repo"
)

// TranslateURLs - creates URLs that are unique to each version of a chart,
// that contain the chart name, version, and link type as query parameters, making easier to identify
// and download specific version of charts when web browsing.
//
// This is achieved by iterating through each entry of each Helm chart, concatenating, and encoding the information into the URL.
// The base URL will be the one that Rancher is available at.
//
// Parameters:
//   - baseURL: The base URL to use as the prefix for all chart and icon URLs in the index file.
//   - index: The index file to modify. This should be a parsed JSON object that contains information
//     about the charts and their versions.
//
// Returns:
//   - An error if there was a problem modifying the URLs in the index file
//   - nil if the operation was successful.
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
