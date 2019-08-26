package workload

import (
	"net/url"
	"strings"
)

func GetRegistryDomain(registry string) (string, error) {
	if strings.HasPrefix(registry, "http://") ||
		strings.HasPrefix(registry, "https://") {
		domain, err := url.Parse(registry)
		if err != nil {
			return registry, err
		}
		return domain.Host, nil
	}
	return registry, nil
}
